package scenarios

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/custom/examples/otelcolbin"
	"github.com/dash0hq/agent-skills/evals/custom/harness"

	"github.com/dash0hq/agent-skills/evals/custom/internal/testutil"
)

// fixtureHooksFor returns the fixture hooks a scenario runs with: the
// Collector-backed scenarios (see collectorScenarioSpecs) run the pinned
// otelcol-contrib binary as a host process, everything else runs the Docker
// topology. It skips the test when the infrastructure the scenario needs is
// unavailable, so `go test ./...` stays hermetic on hosts without Docker or
// network access.
func fixtureHooksFor(t *testing.T, sc harness.Scenario) harness.FixtureHooks {
	t.Helper()
	if spec, ok := collectorScenarioSpecs[sc.ID]; ok {
		return NewCollectorFixture(t, otelcolBinary(t), spec).Hooks()
	}
	if !DockerAvailable() {
		t.Skip("skipping: docker CLI or daemon not available")
	}
	return fixtureHooks(NewDockerFixture(t, filepath.Join(testutil.RepoRoot(t), "evals", "custom")), sc)
}

var (
	otelcolOnce sync.Once
	otelcolPath string
	otelcolErr  error
)

// otelcolBinary returns the pinned otelcol-contrib binary (fetched once per
// test process; see evals/custom/examples/otelcolbin), skipping the test when it
// cannot be fetched in this environment.
func otelcolBinary(t *testing.T) string {
	t.Helper()
	otelcolOnce.Do(func() {
		versions, err := harness.LoadVersions(filepath.Join(testutil.RepoRoot(t), "evals", "custom", "versions.env"))
		if err != nil {
			otelcolErr = err
			return
		}
		otelcolPath, otelcolErr = otelcolbin.Fetch(versions.OtelcolContribVersion, versions.Raw)
	})
	if otelcolErr != nil {
		t.Skipf("otelcol-contrib binary unavailable: %v", otelcolErr)
	}
	return otelcolPath
}

// knownGoodCollectorConfig is a hardened configuration shaped the way the
// otel-collector skill teaches (memory_limiter first in all 3 signal
// pipelines, no batch processor, placeholder contract respected). The hook
// tests run it with the real otelcol-contrib binary.
const knownGoodCollectorConfig = `receivers:
  otlp:
    protocols:
      http:
        endpoint: 127.0.0.1:${env:EVAL_OTLP_RECEIVER_PORT}

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 256

exporters:
  otlphttp:
    endpoint: ${env:EVAL_OTLP_ENDPOINT}
    compression: none
    headers:
      Authorization: "Bearer ${env:EVAL_OTLP_TOKEN}"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlphttp]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlphttp]
    logs:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlphttp]
  telemetry:
    metrics:
      level: none
`

// writeCollectorWorkspace writes a config.yaml with the given content into a
// fresh temp workspace and returns the workspace path.
func writeCollectorWorkspace(t *testing.T, config string) string {
	t.Helper()
	workdir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workdir, collectorConfigFile), []byte(config), 0o644))
	return workdir
}

// collectorTestEnv composes the fixture environment the harness would supply,
// pointing the exporter placeholders straight at the in-process sink.
func collectorTestEnv(sink *otelsink.Sink) map[string]string {
	return map[string]string{
		harness.EnvOTLPEndpoint:    sink.HTTPEndpoint(),
		harness.EnvOTLPToken:       "test-token",
		"OTEL_RESOURCE_ATTRIBUTES": otelsink.TestIDAttribute + "=" + sink.TestID(),
	}
}

// The end-to-end hook test against a known-good configuration: Build
// validates and lints it, Run starts the real otelcol-contrib binary and
// feeds the synthetic telemetry of the pipeline-hardening scenario through
// its OTLP receiver, and the scenario assertion then sees all 3 signals at
// the sink with the test.id resource attribute preserved.
func TestCollectorFixtureRunsKnownGoodConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping the otelcol-contrib host-process test in -short mode")
	}
	binary := otelcolBinary(t)

	sink := otelsink.Start(t)
	workdir := writeCollectorWorkspace(t, knownGoodCollectorConfig)
	env := collectorTestEnv(sink)

	fix := NewCollectorFixture(t, binary, collectorScenarioSpecs[CollectorPipelineHardeningID])
	hooks := fix.Hooks()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	require.NoError(t, hooks.Build(ctx, workdir, env))
	require.NoError(t, hooks.Run(ctx, workdir, env))

	assertion := assertAllSignalsFlow(CollectorServiceName, hardeningSpanCount)
	deadline := time.Now().Add(30 * time.Second)
	for {
		err := assertion(t, sink)
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("telemetry did not flow through the Collector: %v", err)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// A configuration referencing an unknown component must fail in the Build
// hook (otelcol-contrib validate), which the runner classifies agent-build.
func TestCollectorFixtureBuildRejectsUnknownComponent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping the otelcol-contrib host-process test in -short mode")
	}
	binary := otelcolBinary(t)

	sink := otelsink.Start(t)
	broken := strings.ReplaceAll(knownGoodCollectorConfig, "memory_limiter", "not_a_real_processor")
	workdir := writeCollectorWorkspace(t, broken)

	fix := NewCollectorFixture(t, binary, collectorScenarioSpecs[CollectorPipelineHardeningID])

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err := fix.Hooks().Build(ctx, workdir, collectorTestEnv(sink))
	require.Error(t, err)
	require.Contains(t, err.Error(), "validate")
}

// A workspace where the agent deleted config.yaml fails the Build hook.
func TestCollectorFixtureBuildRequiresConfigFile(t *testing.T) {
	fix := NewCollectorFixture(t, "otelcol-contrib-not-invoked", collectorScenarioSpecs[CollectorPipelineHardeningID])
	err := fix.Hooks().Build(context.Background(), t.TempDir(), map[string]string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), collectorConfigFile)
}

func TestLintHardenedPipelines(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name:   "memory_limiter first in every pipeline passes",
			config: knownGoodCollectorConfig,
		},
		{
			name: "named memory_limiter instance first passes",
			config: `service:
  pipelines:
    traces:
      processors: [memory_limiter/spike, transform]
`,
		},
		{
			name: "no processors fails",
			config: `service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlphttp]
`,
			wantErr: "memory_limiter",
		},
		{
			name: "memory_limiter not first fails",
			config: `service:
  pipelines:
    traces:
      processors: [resource, memory_limiter]
`,
			wantErr: "memory_limiter",
		},
		{
			name: "batch processor fails",
			config: `service:
  pipelines:
    traces:
      processors: [memory_limiter, batch]
`,
			wantErr: "batch",
		},
		{
			name: "one bad pipeline among good ones fails",
			config: `service:
  pipelines:
    traces:
      processors: [memory_limiter]
    logs:
      processors: [transform]
`,
			wantErr: `pipeline "logs"`,
		},
		{
			name:    "no pipelines fails",
			config:  "service: {}\n",
			wantErr: "no service pipelines",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(writeCollectorWorkspace(t, tt.config), collectorConfigFile)
			err := lintHardenedPipelines(configPath)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestTestIDFromEnv(t *testing.T) {
	id, err := testIDFromEnv(map[string]string{
		"OTEL_RESOURCE_ATTRIBUTES": "deployment.environment.name=eval, " + otelsink.TestIDAttribute + "=abc-123",
	})
	require.NoError(t, err)
	require.Equal(t, "abc-123", id)

	_, err = testIDFromEnv(map[string]string{"OTEL_RESOURCE_ATTRIBUTES": "foo=bar"})
	require.Error(t, err)

	_, err = testIDFromEnv(map[string]string{})
	require.Error(t, err)
}

func TestComponentType(t *testing.T) {
	require.Equal(t, "memory_limiter", componentType("memory_limiter"))
	require.Equal(t, "memory_limiter", componentType("memory_limiter/spike"))
	require.Equal(t, "batch", componentType("batch/two/parts"))
}

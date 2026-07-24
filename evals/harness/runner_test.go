package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/internal/testutil"
)

// testScenario is the baseline scenario used across runner tests: assertions
// demand a "GET /checkout" server span with service.name=checkout.
func testScenario() Scenario {
	return Scenario{
		ID:        "go-http-basic",
		Skill:     SkillInstrumentation,
		RuleFiles: []string{"skills/otel-instrumentation/rules/sdks/go.md"},
		Prompt: "Instrument this service with OpenTelemetry using the otel-instrumentation skill. " +
			"Export OTLP to ${env:" + EnvOTLPEndpoint + "} authenticating with ${env:" + EnvOTLPToken + "}.",
		Timeout:          30 * time.Second,
		TelemetryTimeout: 3 * time.Second,
		MaxTurns:         10,
		Assert: func(t *testing.T, sink *otelsink.Sink) error {
			tr := sink.Traces(t).WithName("GET /checkout").WithResourceAttribute("service.name", "checkout")
			if tr.Len() == 0 {
				return errors.New("no GET /checkout span with service.name=checkout")
			}
			return nil
		},
	}
}

func newRunner(t *testing.T, stubBinary string, hooks FixtureHooks) *Runner {
	t.Helper()
	root := testutil.RepoRoot(t)
	return &Runner{
		RepoRoot: root,
		Agent:    &Agent{Binary: stubBinary, PluginDir: root, Model: "claude-fable-5"},
		Hooks:    hooks,
	}
}

func goSkillFile(t *testing.T) string {
	return filepath.Join(testutil.RepoRoot(t), "skills", "otel-instrumentation", "rules", "sdks", "go.md")
}

func TestRunnerHappyPath(t *testing.T) {
	stub := testutil.WriteStub(t, testutil.GoodStubBody(string(SkillInstrumentation), goSkillFile(t)))
	r := newRunner(t, stub, sendSpanHooks("GET /checkout", map[string]string{"service.name": "checkout"}))

	v := r.Run(t, testScenario())

	require.True(t, v.Passed, "verdict: %+v", v)
	require.Equal(t, ClassNone, v.Class)
	require.Equal(t, 1, v.AgentAttempts)
	require.Equal(t, 0, v.InfraRetries)
	require.False(t, v.Flake)
	require.InDelta(t, 0.05, v.CostUSD, 1e-9, "cost from the result event's total_cost_usd")
	require.Len(t, v.Evidence.TranscriptPaths, 1)
	require.FileExists(t, v.Evidence.TranscriptPaths[0])
	require.NotEmpty(t, v.Evidence.TelemetryPaths)
	for _, p := range v.Evidence.TelemetryPaths {
		require.FileExists(t, p)
	}
}

// Covers AE1: an attempt that fails agent-attributably and then succeeds
// yields a passing verdict that records the flake. Both attempts load the
// skill (their init events expose the command); the first attempt's span
// lacks the asserted service.name, so it fails agent-assert, and the second
// sends the correct span.
func TestRunnerFlakeRecordedOnRetry(t *testing.T) {
	stub := testutil.WriteStub(t, testutil.GoodStubBody(string(SkillInstrumentation), goSkillFile(t)))

	var attempt int
	hooks := FixtureHooks{
		Run: func(ctx context.Context, _ string, env map[string]string) error {
			attempt++
			attrs := map[string]string{"service.name": "checkout"}
			if attempt == 1 {
				attrs = nil // first span misses the asserted attribute
			}
			return sendTestSpan(ctx, env, "GET /checkout", attrs)
		},
	}
	r := newRunner(t, stub, hooks)

	v := r.Run(t, testScenario())

	require.True(t, v.Passed, "verdict: %+v", v)
	require.Equal(t, 2, v.AgentAttempts)
	require.True(t, v.Flake)
	require.Contains(t, v.FlakeNote, string(ClassAgentAssert))
	require.Len(t, v.Evidence.TranscriptPaths, 2)
}

// An API-overload failure is classified infra and retried without consuming
// the agent-attributable retry.
func TestRunnerInfraRetryDoesNotConsumeAgentRetry(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "invocations")
	body := fmt.Sprintf(`#!/bin/sh
echo x >> %q
n=$(wc -l < %q)
if [ "$n" -le 1 ]; then
	echo "API Error: 529 overloaded_error" >&2
	exit 1
fi
%s`, counter, counter, testutil.CatBody(testutil.InitLineWithSkill(string(SkillInstrumentation)), testutil.ReadToolLine(goSkillFile(t)), testutil.ResultLine))
	stub := testutil.WriteStub(t, body)
	r := newRunner(t, stub, sendSpanHooks("GET /checkout", map[string]string{"service.name": "checkout"}))

	v := r.Run(t, testScenario())

	require.True(t, v.Passed, "verdict: %+v", v)
	require.Equal(t, 1, v.InfraRetries, "infra failure retried without counting")
	require.Equal(t, 1, v.AgentAttempts, "agent retry not consumed by the infra failure")
	require.False(t, v.Flake, "an infra retry is not a skill flake")
}

// Infra retry cap: 3 infra failures produce a terminal infra-fail verdict
// (not an endless loop), flagged non-skill-attributable.
func TestRunnerInfraFailAfterCap(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "invocations")
	body := fmt.Sprintf(`#!/bin/sh
echo x >> %q
echo "API Error: 529 overloaded_error" >&2
exit 1
`, counter)
	stub := testutil.WriteStub(t, body)
	r := newRunner(t, stub, FixtureHooks{})

	v := r.Run(t, testScenario())

	require.False(t, v.Passed)
	require.Equal(t, ClassInfraFail, v.Class)
	require.True(t, v.NonSkillAttributable)
	require.Equal(t, MaxInfraAttempts, v.InfraRetries)
	require.Equal(t, 0, v.AgentAttempts)

	invocations, err := os.ReadFile(counter)
	require.NoError(t, err)
	require.Equal(t, MaxInfraAttempts, strings.Count(string(invocations), "x"), "stub invoked exactly MaxInfraAttempts times")
}

// preserveAgentWorkspace must copy human-authored source into the evidence
// dir with the per-run token scrubbed, and must skip binary files and
// dependency trees, so the uploaded CI artifact never carries the secret.
func TestPreserveAgentWorkspaceScrubsToken(t *testing.T) {
	workdir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "otel.go"), []byte("Authorization=Bearer SEKRET-TOKEN-42"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "app"), []byte{0x00, 0x01, 0xff, 0xfe, 0x00}, 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(workdir, "node_modules", "x"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "node_modules", "x", "y.js"), []byte("SEKRET-TOKEN-42"), 0o644))

	verdictDir := t.TempDir()
	t.Setenv("EVAL_VERDICT_DIR", verdictDir)
	preserveAgentWorkspace("scn", workdir, "SEKRET-TOKEN-42")

	got, err := os.ReadFile(filepath.Join(verdictDir, "agent-workspace", "scn", "otel.go"))
	require.NoError(t, err)
	require.Equal(t, "Authorization=Bearer ***", string(got))

	_, err = os.Stat(filepath.Join(verdictDir, "agent-workspace", "scn", "app"))
	require.True(t, os.IsNotExist(err), "binary file must be skipped")
	_, err = os.Stat(filepath.Join(verdictDir, "agent-workspace", "scn", "node_modules"))
	require.True(t, os.IsNotExist(err), "node_modules must be skipped")
}

// Covers harness-level AE4: telemetry arrives without the expected attribute;
// the verdict fails with class agent-assert and attaches evidence.
func TestRunnerAssertFailureAttachesEvidence(t *testing.T) {
	stub := testutil.WriteStub(t, testutil.GoodStubBody(string(SkillInstrumentation), goSkillFile(t)))
	// The span arrives, but without the service.name the assertion demands.
	r := newRunner(t, stub, sendSpanHooks("GET /checkout", nil))

	v := r.Run(t, testScenario())

	require.False(t, v.Passed)
	require.Equal(t, ClassAgentAssert, v.Class)
	require.Contains(t, v.Detail, "service.name")
	require.Equal(t, MaxAgentAttempts, v.AgentAttempts)
	require.Len(t, v.Evidence.TranscriptPaths, MaxAgentAttempts)
	require.NotEmpty(t, v.Evidence.TelemetryPaths, "received telemetry attached as evidence")
	for _, p := range append(v.Evidence.TranscriptPaths, v.Evidence.TelemetryPaths...) {
		require.FileExists(t, p)
	}
}

// Non-empty plugin_errors in the system/init event classifies the attempt
// infra: the scenario never counts as a skill failure.
func TestRunnerPluginErrorsNeverSkillAttributed(t *testing.T) {
	body := "#!/bin/sh\n" + testutil.CatBody(
		`{"type":"system","subtype":"init","plugin_errors":["dash0-agent-skills: skill frontmatter invalid"]}`,
		testutil.ResultLine,
	)
	stub := testutil.WriteStub(t, body)
	r := newRunner(t, stub, FixtureHooks{})

	v := r.Run(t, testScenario())

	require.False(t, v.Passed)
	require.Equal(t, ClassInfraFail, v.Class, "plugin load failures are infra, terminal after the retry cap")
	require.True(t, v.NonSkillAttributable)
	require.Equal(t, 0, v.AgentAttempts, "no agent-attributable attempt was consumed")
	require.Contains(t, v.Detail, "plugin_errors")
}

// Skill causality: when the init event's slash_commands omit the target
// skill's command, the harness could not load the skill; that is a plugin
// failure classified infra (retried up to the infra cap, terminal
// infra-fail), never attributed to the skill. (The happy-path test covers the
// converse: an init event exposing the command loads the skill.)
func TestRunnerSkillNotLoaded(t *testing.T) {
	stub := testutil.WriteStub(t, testutil.NoSkillStubBody())
	r := newRunner(t, stub, sendSpanHooks("GET /checkout", map[string]string{"service.name": "checkout"}))

	v := r.Run(t, testScenario())

	require.False(t, v.Passed)
	require.Equal(t, ClassInfraFail, v.Class)
	require.True(t, v.NonSkillAttributable)
	require.Equal(t, 0, v.AgentAttempts, "a skill that never loaded consumes no agent-attributable attempt")
	require.Equal(t, MaxInfraAttempts, v.InfraRetries)
	require.Contains(t, v.Detail, "slash_commands")
}

// A stalled agent is killed at the scenario wall clock and classified
// agent-telemetry.
func TestRunnerTimeoutKillsStalledAgent(t *testing.T) {
	body := "#!/bin/sh\n" + testutil.CatBody(testutil.InitLine) + "exec sleep 60\n"
	stub := testutil.WriteStub(t, body)
	r := newRunner(t, stub, FixtureHooks{})

	sc := testScenario()
	sc.Timeout = 1 * time.Second
	sc.TelemetryTimeout = 500 * time.Millisecond

	start := time.Now()
	v := r.Run(t, sc)
	elapsed := time.Since(start)

	require.False(t, v.Passed)
	require.Equal(t, ClassAgentTelemetry, v.Class)
	require.Equal(t, MaxAgentAttempts, v.AgentAttempts)
	require.Less(t, elapsed, 20*time.Second, "stalled attempts must be killed at the wall clock, not run to completion")
}

// A failing fixture build hook classifies the attempt agent-build.
func TestRunnerBuildFailureClassifiedAgentBuild(t *testing.T) {
	stub := testutil.WriteStub(t, testutil.GoodStubBody(string(SkillInstrumentation), goSkillFile(t)))
	hooks := FixtureHooks{
		Build: func(context.Context, string, map[string]string) error {
			return errors.New("go build: syntax error in main.go")
		},
	}
	r := newRunner(t, stub, hooks)

	v := r.Run(t, testScenario())

	require.False(t, v.Passed)
	require.Equal(t, ClassAgentBuild, v.Class)
	require.Contains(t, v.Detail, "syntax error")
}

// No telemetry before the telemetry timeout classifies the attempt
// agent-telemetry.
func TestRunnerNoTelemetryClassifiedAgentTelemetry(t *testing.T) {
	stub := testutil.WriteStub(t, testutil.GoodStubBody(string(SkillInstrumentation), goSkillFile(t)))
	r := newRunner(t, stub, FixtureHooks{}) // no hook sends anything

	sc := testScenario()
	sc.TelemetryTimeout = 500 * time.Millisecond
	v := r.Run(t, sc)

	require.False(t, v.Passed)
	require.Equal(t, ClassAgentTelemetry, v.Class)
}

// The fixture copy lands in the workspace the agent (and the hooks) run in,
// and the composed environment carries the placeholder contract variables.
func TestRunnerCopiesFixtureAndComposesEnv(t *testing.T) {
	stub := testutil.WriteStub(t, testutil.GoodStubBody(string(SkillInstrumentation), goSkillFile(t)))

	var seenWorkdir string
	var seenEnv map[string]string
	hooks := FixtureHooks{
		Run: func(ctx context.Context, workdir string, env map[string]string) error {
			seenWorkdir = workdir
			seenEnv = env
			return sendTestSpan(ctx, env, "GET /checkout", map[string]string{"service.name": "checkout"})
		},
	}
	r := newRunner(t, stub, hooks)

	sc := testScenario()
	// Any committed directory works as a fixture stand-in; the relay command
	// directory is small and stable.
	sc.FixturePath = "evals/cmd/relay"
	v := r.Run(t, sc)

	require.True(t, v.Passed, "verdict: %+v", v)
	require.FileExists(t, filepath.Join(seenWorkdir, "main.go"), "fixture files copied into the workspace")
	require.NotEqual(t, filepath.Join(r.RepoRoot, "evals", "cmd", "relay"), seenWorkdir, "agent must work on a copy, not the checkout")
	require.NotEmpty(t, seenEnv[EnvOTLPEndpoint])
	require.NotEmpty(t, seenEnv[EnvOTLPToken])
	require.Contains(t, seenEnv["OTEL_RESOURCE_ATTRIBUTES"], otelsink.TestIDAttribute+"=")
	require.Equal(t, seenEnv[EnvOTLPEndpoint], seenEnv["OTEL_EXPORTER_OTLP_ENDPOINT"])
}

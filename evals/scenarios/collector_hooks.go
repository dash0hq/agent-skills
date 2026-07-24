package scenarios

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/harness"
)

// EnvOTLPReceiverPort is the placeholder environment variable carrying the
// port the agent-authored Collector configuration must bind its OTLP/HTTP
// receiver to (referenced as ${env:EVAL_OTLP_RECEIVER_PORT}). The harness
// allocates a free port per attempt so parallel Collector processes never
// collide; the agent's configuration executes verbatim, never rewritten.
const EnvOTLPReceiverPort = "EVAL_OTLP_RECEIVER_PORT"

// collectorConfigFile is the configuration file the collector-workspace
// fixture ships and the task prompts direct the agent to edit.
const collectorConfigFile = "config.yaml"

// collectorStartTimeout bounds how long the Run hook waits for the started
// Collector's OTLP receiver to accept TCP connections.
const collectorStartTimeout = 30 * time.Second

// CollectorScenarioSpec describes how the harness exercises the Collector for
// one scenario: what synthetic telemetry to feed through its OTLP receiver,
// and an optional structural check on the agent's configuration.
type CollectorScenarioSpec struct {
	// Feed posts synthetic OTLP telemetry to the running Collector's
	// OTLP/HTTP receiver base URL, stamping the given test.id on every
	// resource so the sink-side assertions can see it.
	Feed func(ctx context.Context, endpoint, testID string) error
	// LintConfig, when non-nil, runs in the Build hook after
	// `otelcol-contrib validate` succeeds and checks structural properties
	// of the agent's configuration that telemetry cannot prove (for
	// example memory_limiter placement). A returned error classifies the
	// attempt agent-build, like any other Build failure.
	LintConfig func(configPath string) error
}

// collectorScenarioSpecs maps the Collector-backed scenario IDs to their
// execution specs. The run entrypoint uses it to give these scenarios
// host-process Collector hooks instead of the Docker fixture hooks.
var collectorScenarioSpecs = map[string]CollectorScenarioSpec{
	CollectorPipelineHardeningID: {Feed: feedPipelineHardening, LintConfig: lintHardenedPipelines},
	OTTLRedactionID:              {Feed: feedRedactionSpans},
	OTTLEnrichmentID:             {Feed: feedEnrichmentSpans},
}

// CollectorFixture implements the harness FixtureHooks contract for scenarios
// whose artifact is a Collector configuration, without Docker:
//
//   - Build runs `otelcol-contrib validate` (the pinned binary, as a host
//     process) against the agent's config.yaml, with the placeholder
//     environment variables supplied for ${env:...} expansion, then applies
//     the spec's optional config lint. Any error classifies agent-build.
//   - Run starts otelcol-contrib with the config verbatim, waits for the
//     OTLP receiver to accept connections, and feeds the spec's synthetic
//     telemetry through it. The process keeps running when Run returns, so
//     telemetry still flushing through the pipeline reaches the sink while
//     the runner waits; Close (wired to t.Cleanup) stops it.
type CollectorFixture struct {
	binary string
	spec   CollectorScenarioSpec

	mu    sync.Mutex
	port  string
	procs []*exec.Cmd
}

// NewCollectorFixture returns a CollectorFixture running the given
// otelcol-contrib binary; its processes are stopped when the test finishes.
func NewCollectorFixture(t testingT, binary string, spec CollectorScenarioSpec) *CollectorFixture {
	t.Helper()
	f := &CollectorFixture{binary: binary, spec: spec}
	t.Cleanup(f.Close)
	return f
}

// Hooks returns the harness fixture hooks backed by this CollectorFixture.
func (f *CollectorFixture) Hooks() harness.FixtureHooks {
	return harness.FixtureHooks{Build: f.build, Run: f.run}
}

// Close kills every Collector process started so far.
func (f *CollectorFixture) Close() {
	f.mu.Lock()
	procs := f.procs
	f.procs = nil
	f.mu.Unlock()
	for _, cmd := range procs {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

// build validates the agent's configuration with the pinned otelcol-contrib
// binary and applies the spec's optional structural lint.
func (f *CollectorFixture) build(ctx context.Context, workdir string, env map[string]string) error {
	configPath := filepath.Join(workdir, collectorConfigFile)
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("the workspace has no %s (the task requires editing it in place): %w", collectorConfigFile, err)
	}

	port, err := allocatePort()
	if err != nil {
		return fmt.Errorf("allocate receiver port: %w", err)
	}
	f.mu.Lock()
	f.port = port
	f.mu.Unlock()

	cmd := exec.CommandContext(ctx, f.binary, "validate", "--config", configPath)
	cmd.Dir = workdir
	cmd.Env = collectorEnv(env, port)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("otelcol-contrib validate failed: %w\n%s", err, out)
	}

	if f.spec.LintConfig != nil {
		if err := f.spec.LintConfig(configPath); err != nil {
			return err
		}
	}
	return nil
}

// run starts the Collector with the agent's configuration verbatim, waits for
// the OTLP receiver, and feeds the spec's synthetic telemetry through it.
func (f *CollectorFixture) run(ctx context.Context, workdir string, env map[string]string) error {
	f.mu.Lock()
	port := f.port
	f.mu.Unlock()
	if port == "" {
		var err error
		if port, err = allocatePort(); err != nil {
			return fmt.Errorf("allocate receiver port: %w", err)
		}
	}

	var output bytes.Buffer
	cmd := exec.CommandContext(ctx, f.binary, "--config", filepath.Join(workdir, collectorConfigFile))
	cmd.Dir = workdir
	cmd.Env = collectorEnv(env, port)
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start otelcol-contrib: %w", err)
	}
	f.mu.Lock()
	f.procs = append(f.procs, cmd)
	f.mu.Unlock()
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	if err := waitForCollector(ctx, port, exited, &output); err != nil {
		return err
	}

	testID, err := testIDFromEnv(env)
	if err != nil {
		return err
	}
	return f.spec.Feed(ctx, "http://127.0.0.1:"+port, testID)
}

// waitForCollector polls the receiver port until it accepts a TCP connection,
// the process exits, or the start timeout elapses.
func waitForCollector(ctx context.Context, port string, exited <-chan error, output *bytes.Buffer) error {
	deadline := time.Now().Add(collectorStartTimeout)
	for {
		select {
		case err := <-exited:
			return fmt.Errorf("otelcol-contrib exited before its OTLP receiver came up: %v\n%s", err, output.String())
		default:
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return fmt.Errorf("otelcol-contrib did not accept connections on port %s within %s (last error: %v)", port, collectorStartTimeout, err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// collectorEnv composes the environment for otelcol-contrib invocations: the
// parent process environment, the harness-composed fixture environment, and
// the allocated receiver port for ${env:EVAL_OTLP_RECEIVER_PORT} expansion.
func collectorEnv(env map[string]string, port string) []string {
	out := os.Environ()
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return append(out, EnvOTLPReceiverPort+"="+port)
}

// testIDFromEnv extracts the per-run test.id from the harness-composed
// OTEL_RESOURCE_ATTRIBUTES value; telemetry without it is invisible to the
// sink's test-scoped views.
func testIDFromEnv(env map[string]string) (string, error) {
	for _, pair := range strings.Split(env["OTEL_RESOURCE_ATTRIBUTES"], ",") {
		if key, value, ok := strings.Cut(pair, "="); ok && strings.TrimSpace(key) == otelsink.TestIDAttribute {
			return strings.TrimSpace(value), nil
		}
	}
	return "", fmt.Errorf("no %s in the composed OTEL_RESOURCE_ATTRIBUTES", otelsink.TestIDAttribute)
}

// allocatePort reserves a free loopback TCP port and returns it. The listener
// is closed before returning, so a small race with other port consumers
// exists; it is acceptable for test infrastructure.
func allocatePort() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer ln.Close()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	return port, err
}

// --- hardening lint ---

// lintHardenedPipelines enforces the memory-safety shape the otel-collector
// skill teaches on the agent's configuration, which telemetry at the sink
// cannot prove: memory_limiter is placed first in every pipeline, and no
// pipeline uses the batch processor (the skill mandates exporter-level
// batching via the sending_queue instead). It runs in the Build hook, so a
// violation classifies agent-build; missing signal pipelines are deliberately
// not checked here, because the sink-side assertions catch missing signals
// with the more precise agent-assert class.
func lintHardenedPipelines(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}
	var cfg struct {
		Service struct {
			Pipelines map[string]struct {
				Processors []string `yaml:"processors"`
			} `yaml:"pipelines"`
		} `yaml:"service"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse %s: %w", collectorConfigFile, err)
	}
	if len(cfg.Service.Pipelines) == 0 {
		return fmt.Errorf("%s declares no service pipelines", collectorConfigFile)
	}

	names := make([]string, 0, len(cfg.Service.Pipelines))
	for name := range cfg.Service.Pipelines {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pipeline := cfg.Service.Pipelines[name]
		if len(pipeline.Processors) == 0 || componentType(pipeline.Processors[0]) != "memory_limiter" {
			return fmt.Errorf("pipeline %q does not place memory_limiter first (processors: %v); the otel-collector skill requires memory_limiter first in every pipeline", name, pipeline.Processors)
		}
		for _, processor := range pipeline.Processors {
			if componentType(processor) == "batch" {
				return fmt.Errorf("pipeline %q uses the batch processor; the otel-collector skill mandates exporter-level batching via the sending_queue instead", name)
			}
		}
	}
	return nil
}

// componentType strips the optional /name suffix from a Collector component
// ID (memory_limiter/spike -> memory_limiter).
func componentType(componentID string) string {
	base, _, _ := strings.Cut(componentID, "/")
	return base
}

// --- synthetic telemetry feeders ---

// Synthetic telemetry constants shared by the feeders and the assertions.
const (
	// hardeningSpanCount is how many spans feedPipelineHardening posts; the
	// assertion requires all of them at the sink.
	hardeningSpanCount = 3
	// hardeningMetricName is the Sum metric feedPipelineHardening posts.
	hardeningMetricName = "eval.checkout.orders"
	// hardeningLogBody is the log record body feedPipelineHardening posts.
	hardeningLogBody = "eval checkout completed"
)

// feedSpan describes one synthetic span posted to a Collector receiver.
type feedSpan struct {
	name  string
	kind  tracepb.Span_SpanKind
	attrs map[string]string
}

// feedPipelineHardening posts spans, one Sum metric, and one log record
// through the Collector's OTLP/HTTP receiver, all under the same resource, so
// the sink-side assertion can prove every signal pipeline is wired and
// preserves resource attributes.
func feedPipelineHardening(ctx context.Context, endpoint, testID string) error {
	if err := postHardeningSpans(ctx, endpoint, testID); err != nil {
		return err
	}
	if err := postHardeningMetrics(ctx, endpoint, testID); err != nil {
		return err
	}
	return postHardeningLogs(ctx, endpoint, testID)
}

// feedRedactionSpans posts spans carrying the user.email attribute the
// redaction scenario must remove, next to attributes that must survive.
func feedRedactionSpans(ctx context.Context, endpoint, testID string) error {
	spans := make([]feedSpan, 0, hardeningSpanCount)
	for i := 0; i < hardeningSpanCount; i++ {
		spans = append(spans, feedSpan{
			name: "GET /checkout",
			kind: tracepb.Span_SPAN_KIND_SERVER,
			attrs: map[string]string{
				"user.email": "user@example.test",
				"user.id":    "TEST-0001",
				"url.path":   "/checkout",
			},
		})
	}
	return postSpansTolerant(ctx, endpoint, testID, spans...)
}

// feedEnrichmentSpans posts spans without any deployment.environment.name so
// only Collector-side enrichment can introduce it.
func feedEnrichmentSpans(ctx context.Context, endpoint, testID string) error {
	spans := make([]feedSpan, 0, hardeningSpanCount)
	for i := 0; i < hardeningSpanCount; i++ {
		spans = append(spans, feedSpan{
			name:  "GET /checkout",
			kind:  tracepb.Span_SPAN_KIND_SERVER,
			attrs: map[string]string{"user.id": "TEST-0001"},
		})
	}
	return postSpansTolerant(ctx, endpoint, testID, spans...)
}

// postHardeningSpans posts the hardening scenario's spans.
func postHardeningSpans(ctx context.Context, endpoint, testID string) error {
	spans := make([]feedSpan, 0, hardeningSpanCount)
	for i := 0; i < hardeningSpanCount; i++ {
		spans = append(spans, feedSpan{
			name:  "GET /checkout",
			kind:  tracepb.Span_SPAN_KIND_SERVER,
			attrs: map[string]string{"url.path": "/checkout"},
		})
	}
	return postSpansTolerant(ctx, endpoint, testID, spans...)
}

// postSpansTolerant posts spans to the receiver's traces path, tolerating
// per-signal HTTP rejections (see postSignalTolerant).
func postSpansTolerant(ctx context.Context, endpoint, testID string, spans ...feedSpan) error {
	now := time.Now()
	pbSpans := make([]*tracepb.Span, 0, len(spans))
	for _, s := range spans {
		pbSpans = append(pbSpans, &tracepb.Span{
			TraceId:           randomBytes(16),
			SpanId:            randomBytes(8),
			Name:              s.name,
			Kind:              s.kind,
			StartTimeUnixNano: uint64(now.Add(-time.Millisecond).UnixNano()),
			EndTimeUnixNano:   uint64(now.UnixNano()),
			Attributes:        strAttrs(s.attrs),
		})
	}
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource:   evalResource(testID),
			ScopeSpans: []*tracepb.ScopeSpans{{Spans: pbSpans}},
		}},
	}
	return postSignalTolerant(ctx, endpoint+"/v1/traces", req)
}

// postHardeningMetrics posts one cumulative Sum metric.
func postHardeningMetrics(ctx context.Context, endpoint, testID string) error {
	now := time.Now()
	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{{
			Resource: evalResource(testID),
			ScopeMetrics: []*metricspb.ScopeMetrics{{
				Metrics: []*metricspb.Metric{{
					Name:        hardeningMetricName,
					Description: "Synthetic order counter fed through the Collector under eval.",
					Unit:        "{order}",
					Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{
						AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
						IsMonotonic:            true,
						DataPoints: []*metricspb.NumberDataPoint{{
							StartTimeUnixNano: uint64(now.Add(-time.Second).UnixNano()),
							TimeUnixNano:      uint64(now.UnixNano()),
							Value:             &metricspb.NumberDataPoint_AsInt{AsInt: hardeningSpanCount},
						}},
					}},
				}},
			}},
		}},
	}
	return postSignalTolerant(ctx, endpoint+"/v1/metrics", req)
}

// postHardeningLogs posts one INFO log record.
func postHardeningLogs(ctx context.Context, endpoint, testID string) error {
	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			Resource: evalResource(testID),
			ScopeLogs: []*logspb.ScopeLogs{{
				LogRecords: []*logspb.LogRecord{{
					TimeUnixNano:   uint64(time.Now().UnixNano()),
					SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
					SeverityText:   "INFO",
					Body:           &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: hardeningLogBody}},
				}},
			}},
		}},
	}
	return postSignalTolerant(ctx, endpoint+"/v1/logs", req)
}

// evalResource is the resource stamped on all fed telemetry: the Collector
// scenarios' service.name plus the per-run test.id.
func evalResource(testID string) *resourcepb.Resource {
	return &resourcepb.Resource{Attributes: strAttrs(map[string]string{
		"service.name":           CollectorServiceName,
		otelsink.TestIDAttribute: testID,
	})}
}

// otlpHTTPStatusError is a non-2xx response from an OTLP/HTTP endpoint.
type otlpHTTPStatusError struct {
	url    string
	status int
	body   string
}

func (e *otlpHTTPStatusError) Error() string {
	return fmt.Sprintf("OTLP endpoint %s returned HTTP %d: %s", e.url, e.status, e.body)
}

// postSignalTolerant posts one export request and tolerates HTTP status
// rejections: a Collector without a pipeline for the signal legitimately
// rejects the request, and the sink-side assertions then catch the missing
// signal with the agent-assert class instead of misclassifying it as a
// fixture run failure (agent-build). Transport errors still fail the hook.
func postSignalTolerant(ctx context.Context, url string, msg proto.Message) error {
	err := postOTLPProto(ctx, url, msg)
	var statusErr *otlpHTTPStatusError
	if errors.As(err, &statusErr) {
		return nil
	}
	return err
}

// postOTLPProto posts one protobuf-encoded OTLP export request.
func postOTLPProto(ctx context.Context, url string, msg proto.Message) error {
	body, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal OTLP request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post to %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload := make([]byte, 512)
		n, _ := resp.Body.Read(payload)
		return &otlpHTTPStatusError{url: url, status: resp.StatusCode, body: strings.TrimSpace(string(payload[:n]))}
	}
	return nil
}

// randomBytes returns n cryptographically random bytes for span and trace IDs.
func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

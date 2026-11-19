// The Rust SDK instrumentation scenarios: the HTTP happy path with the
// incoming-parent probes, stdout log correlation, SERVER-span enrichment,
// HTTP server metrics, resource precedence, and gRPC, built on the shared
// sdkHTTPScenario helper (see instr_sdks.go).
package scenarios

import (
	"fmt"
	"testing"
	"time"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

func init() {
	registerScenarios(RustHTTP(), RustLogs(), RustEnrichment(), RustMetrics(), RustResource(), RustGRPC())
}

// Scenario IDs of the Rust scenarios.
const (
	// RustHTTPID is the Rust instrumentation happy path. Like instr-nodejs-http
	// it also probes a sampled and an unsampled incoming traceparent, because
	// Rust has no auto-instrumentation: the SERVER span, the context
	// extraction, and the sampler are all code the agent writes from the rule.
	RustHTTPID = "instr-rust-http"
	// RustLogsID verifies stdout-first log correlation, as instr-go-logs does:
	// tracing-subscriber's JSON formatter knows no OpenTelemetry IDs, so the
	// trace_id and span_id fields come only from the rule's log layer.
	RustLogsID = "instr-rust-logs"
	// RustEnrichmentID demands order.id on the SERVER span itself, as
	// instr-dotnet-enrichment does; in Rust the SERVER span is the one the
	// tower TraceLayer creates, reached through Span::current().
	RustEnrichmentID = "instr-rust-enrichment"
	// RustMetricsID demands the stable http.server.request.duration histogram,
	// which no Rust tracing layer records: it comes only from the metrics
	// layer the rule prescribes, and that layer must not add a second SERVER
	// span next to the TraceLayer one.
	RustMetricsID = "instr-rust-metrics"
	// RustResourceID runs the fixture with service.version and
	// service.instance.id set through OTEL_RESOURCE_ATTRIBUTES, as a
	// Kubernetes pod spec does, and demands they survive the values the
	// service computes itself: in Rust, a detector or attribute added after
	// the environment detectors silently overrides them.
	RustResourceID = "instr-rust-resource"
	// RustGRPCID demands a gRPC CLIENT span and the SERVER span of the call
	// it makes, parent and child in one trace, on a fixture that calls its own
	// tonic service; tonic-tracing-opentelemetry creates its spans at TRACE
	// level, so the info filter drops them unless the rule is followed.
	RustGRPCID = "instr-rust-grpc"
)

// RustServiceName is the service.name the Rust scenarios require.
const RustServiceName = "rust-service-eval"

// rustRuleFile is the dedicated rule file all 3 Rust scenarios cover.
const rustRuleFile = "skills/otel-instrumentation/rules/sdks/rust.md"

// rustFixture and rustDescription are shared by the Rust scenarios that run
// the plain HTTP fixture.
const (
	rustFixture     = "evals/custom/fixtures/rust-service"
	rustDescription = "Rust HTTP service (an axum server calling out with reqwest)"
)

// rustGRPCFixture is the fixture whose /checkout handler reserves stock over
// gRPC through a tonic service the same process hosts.
const (
	rustGRPCFixture     = "evals/custom/fixtures/rust-grpc-service"
	rustGRPCDescription = "Rust HTTP service (an axum server that reserves stock over gRPC with tonic, against an Inventory service the same process hosts, and calls out with reqwest)"
)

// rustMetricsTelemetryTimeout outlasts the 60-second default export interval
// of the Rust SDK's periodic metric reader, so the first export lands.
const rustMetricsTelemetryTimeout = 3 * time.Minute

// rustResourceOverrides are the resource attributes the deployment sets
// through OTEL_RESOURCE_ATTRIBUTES in the resource-precedence scenario.
var rustResourceOverrides = map[string]string{
	"service.version":     "TEST-1.2.3",
	"service.instance.id": "TEST-instance-0001",
}

// rustContainerNote reminds the agent that the instrumentation must be
// effective inside the container the Dockerfile builds.
const rustContainerNote = `- The service runs as the container built by the Dockerfile; make the instrumentation take effect there.`

// RustHTTP is the Rust instrumentation happy-path scenario. Its fixture hooks
// add the parent-context traffic (see fixtureHooks), and its assertions
// require the SERVER span to continue the sampled parent and to record the
// unsampled one.
func RustHTTP() harness.Scenario {
	sc := sdkHTTPScenario(RustHTTPID, rustRuleFile, rustFixture, rustDescription, RustServiceName, rustContainerNote, heavyScenarioTimeout)
	sc.Assert = assertAll(
		assertInboundParentHTTPTraces(RustServiceName),
		assertUnsampledParentRecorded(RustServiceName),
	)
	return sc
}

// RustLogs asserts stdout-first log correlation: the fixture's single-line
// JSON logging on stdout is kept, the "checkout completed" record carries the
// (trace_id, span_id) pair of a span that reached the sink, and application
// logs are not exported over OTLP.
func RustLogs() harness.Scenario {
	sc := sdkHTTPScenario(RustLogsID, rustRuleFile, rustFixture, rustDescription, RustServiceName, rustContainerNote+`
- Keep the service's structured single-line JSON logging on stdout, and make every record it logs while handling a request carry the trace context of the active span: the "checkout completed" record must include the active span's trace_id and span_id as top-level fields.
- Do not export application logs over OTLP: stdout must remain the only delivery channel for application logs.`, heavyScenarioTimeout)
	sc.RuleFiles = append(sc.RuleFiles, "skills/otel-instrumentation/rules/logs.md")
	sc.AssertApp = assertStdoutLogCorrelation(RustServiceName, "checkout completed")
	return sc
}

// RustEnrichment demands the order.id attribute on the SERVER span of the
// request; a child span carrying it fails the assertion.
func RustEnrichment() harness.Scenario {
	sc := sdkHTTPScenario(RustEnrichmentID, rustRuleFile, rustFixture, rustDescription, RustServiceName, rustContainerNote+`
- In the GET /checkout handler, add the attribute order.id with the value "TEST-0001" to the server span of the request itself. Do not create a new child span to carry it.`, heavyScenarioTimeout)
	sc.RuleFiles = append(sc.RuleFiles, "skills/otel-instrumentation/rules/spans.md")
	sc.Assert = assertServerSpanAttribute(RustServiceName, "order.id", "TEST-0001")
	return sc
}

// RustMetrics demands the http.server.request.duration histogram for the
// /checkout route, and still exactly one SERVER span per request.
func RustMetrics() harness.Scenario {
	sc := sdkHTTPScenario(RustMetricsID, rustRuleFile, rustFixture, rustDescription, RustServiceName, rustContainerNote+`
- Also export metrics via OTLP over http/protobuf, including the HTTP server request duration (http.server.request.duration) of GET /checkout.`, heavyScenarioTimeout)
	sc.RuleFiles = append(sc.RuleFiles, "skills/otel-instrumentation/rules/metrics.md")
	sc.TelemetryTimeout = rustMetricsTelemetryTimeout
	sc.Assert = assertAll(
		assertHTTPTraces(RustServiceName),
		assertHTTPServerDurationMetric(RustServiceName, "/checkout"),
		assertOneServerSpanPerRequest(RustServiceName),
	)
	return sc
}

// RustResource demands that the resource attributes the deployment sets win
// over the version and instance ID the service computes.
func RustResource() harness.Scenario {
	sc := sdkHTTPScenario(RustResourceID, rustRuleFile, rustFixture, rustDescription, RustServiceName, rustContainerNote+`
- Report service.version from the crate version in Cargo.toml, and give every process its own service.instance.id.
- Deployments can also set service.version and service.instance.id through OTEL_RESOURCE_ATTRIBUTES; when they do, those values must win over the ones the service computes.`, heavyScenarioTimeout)
	sc.RuleFiles = append(sc.RuleFiles, "skills/otel-instrumentation/rules/resources.md")
	sc.Assert = assertAll(
		assertHTTPTraces(RustServiceName),
		assertServerSpanResource(RustServiceName, rustResourceOverrides),
	)
	return sc
}

// RustGRPC demands the gRPC CLIENT and SERVER spans of the stock reservation,
// on top of the HTTP happy path.
func RustGRPC() harness.Scenario {
	sc := sdkHTTPScenario(RustGRPCID, rustRuleFile, rustGRPCFixture, rustGRPCDescription, RustServiceName, rustContainerNote+`
- Produce a client span for the gRPC Reserve call the handler makes and a server span for the Inventory service handling it, in the same trace as the GET /checkout server span.`, heavyScenarioTimeout)
	sc.Assert = assertAll(
		assertHTTPTraces(RustServiceName),
		assertGRPCClientServerPair(RustServiceName),
	)
	return sc
}

// rpcSystemAttributes are the keys that name the RPC system: the current
// semantic conventions and the older name tonic-tracing-opentelemetry uses.
var rpcSystemAttributes = []string{"rpc.system.name", "rpc.system"}

// assertHTTPServerDurationMetric requires an http.server.request.duration
// histogram in seconds under serviceName with a data point for route.
func assertHTTPServerDurationMetric(serviceName, route string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		all := sink.Metrics(t)
		for _, mv := range all.WithName("http.server.request.duration").Histograms().Metrics() {
			if attrValue(mv.Resource.GetAttributes(), "service.name") != serviceName {
				continue
			}
			if unit := mv.Metric.GetUnit(); unit != "s" {
				return fmt.Errorf("http.server.request.duration has unit %q, want \"s\"", unit)
			}
			for _, dp := range mv.Metric.GetHistogram().GetDataPoints() {
				if attrValue(dp.GetAttributes(), "http.route") == route && dp.GetCount() > 0 {
					return nil
				}
			}
		}
		return fmt.Errorf("no http.server.request.duration histogram with service.name=%q and a data point for http.route=%q (metrics seen: %v)", serviceName, route, all.Names())
	}
}

// assertOneServerSpanPerRequest fails when a trace of serviceName carries more
// than one HTTP SERVER span, the sign of a second server-side layer creating
// its own span next to the TraceLayer one.
func assertOneServerSpanPerRequest(serviceName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		perTrace := map[string]int{}
		for _, sv := range sink.Traces(t).WithResourceAttribute("service.name", serviceName).WithKind(tracepb.Span_SPAN_KIND_SERVER).Spans() {
			if attrValue(sv.Span.GetAttributes(), httpMethodAttributes...) == "" {
				continue
			}
			traceID := fmt.Sprintf("%x", sv.Span.GetTraceId())
			perTrace[traceID]++
			if perTrace[traceID] > 1 {
				return fmt.Errorf("trace %s has %d HTTP SERVER spans; each request must have exactly 1", traceID, perTrace[traceID])
			}
		}
		return nil
	}
}

// assertServerSpanResource requires a SERVER span for GET /checkout under
// serviceName whose resource carries every attribute in want.
func assertServerSpanResource(serviceName string, want map[string]string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		svc := sink.Traces(t).WithResourceAttribute("service.name", serviceName).WithKind(tracepb.Span_SPAN_KIND_SERVER)
		var mismatch string
		for _, sv := range svc.Spans() {
			ok := true
			for key, value := range want {
				if got := attrValue(sv.Resource.GetAttributes(), key); got != value {
					ok = false
					mismatch = fmt.Sprintf("resource attribute %s=%q, want %q", key, got, value)
					break
				}
			}
			if ok {
				return nil
			}
		}
		if mismatch == "" {
			return fmt.Errorf("no SERVER span with service.name=%q", serviceName)
		}
		return fmt.Errorf("no SERVER span with service.name=%q kept the deployment's resource attributes: %s", serviceName, mismatch)
	}
}

// assertGRPCClientServerPair requires a gRPC CLIENT span under serviceName
// and a gRPC SERVER span that is its child in the same trace.
func assertGRPCClientServerPair(serviceName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		svc := sink.Traces(t).WithResourceAttribute("service.name", serviceName)
		var clients, servers []*tracepb.Span
		for _, sv := range svc.Spans() {
			if attrValue(sv.Span.GetAttributes(), rpcSystemAttributes...) != "grpc" {
				continue
			}
			switch sv.Span.GetKind() {
			case tracepb.Span_SPAN_KIND_CLIENT:
				clients = append(clients, sv.Span)
			case tracepb.Span_SPAN_KIND_SERVER:
				servers = append(servers, sv.Span)
			}
		}
		if len(clients) == 0 {
			return fmt.Errorf("no gRPC CLIENT span (rpc.system=grpc) with service.name=%q (span names: %v)", serviceName, svc.Names())
		}
		if len(servers) == 0 {
			return fmt.Errorf("no gRPC SERVER span (rpc.system=grpc) with service.name=%q (span names: %v)", serviceName, svc.Names())
		}
		for _, client := range clients {
			for _, server := range servers {
				if string(server.GetTraceId()) == string(client.GetTraceId()) && string(server.GetParentSpanId()) == string(client.GetSpanId()) {
					return nil
				}
			}
		}
		return fmt.Errorf("found %d gRPC CLIENT and %d gRPC SERVER spans, but no SERVER span is the child of a CLIENT span in the same trace; the client is not injecting traceparent or the server is not extracting it", len(clients), len(servers))
	}
}

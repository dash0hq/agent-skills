// The Rust SDK instrumentation scenarios: the HTTP happy path with the
// incoming-parent probes, stdout log correlation, and SERVER-span enrichment,
// built on the shared sdkHTTPScenario helper (see instr_sdks.go).
package scenarios

import "github.com/dash0hq/agent-skills/evals/custom/harness"

func init() {
	registerScenarios(RustHTTP(), RustLogs(), RustEnrichment())
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
)

// RustServiceName is the service.name the Rust scenarios require.
const RustServiceName = "rust-service-eval"

// rustRuleFile is the dedicated rule file all 3 Rust scenarios cover.
const rustRuleFile = "skills/otel-instrumentation/rules/sdks/rust.md"

// rustFixture and rustDescription are shared by the 3 Rust scenarios.
const (
	rustFixture     = "evals/custom/fixtures/rust-service"
	rustDescription = "Rust HTTP service (an axum server calling out with reqwest)"
)

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

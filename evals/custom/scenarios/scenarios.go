// Package scenarios declares the eval scenarios and registers them on top of
// the harness registry (see evals/custom/harness). Scenario definitions are data:
// the task prompt given to the agent, the fixture it runs against, the rule
// files it covers, and the deterministic assertions over the telemetry that
// reaches the sink. The Go test entrypoint that executes them against the
// real agent and Docker lives in run_test.go.
package scenarios

import (
	"time"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

// Scenario IDs. They are the keys used by quarantine.yaml, CI selection
// output, and the -run / EVAL_SCENARIOS filters of the test entrypoint.
const (
	// GoHTTPID is the Go instrumentation happy path: traces with the
	// expected service.name, a server span, and a client span.
	GoHTTPID = "instr-go-http"
	// GoLogsID verifies stdout-first log correlation, per logs.md: the
	// application's structured stdout records must carry the active span's
	// (trace_id, span_id) pair, and application logs must NOT be exported
	// over OTLP. The scenario originally demanded the opposite (logs over
	// OTLP via a log/slog bridge), contradicting logs.md's stdout-first
	// delivery rule.
	GoLogsID = "instr-go-logs"
	// NodeHTTPID is the Node.js instrumentation happy path.
	NodeHTTPID = "instr-nodejs-http"
)

// Fixture service names the prompts demand and the assertions check (AE4).
const (
	// GoServiceName is the service.name the Go scenarios require.
	GoServiceName = "go-service-eval"
	// NodeServiceName is the service.name the Node.js scenario requires.
	NodeServiceName = "nodejs-service-eval"
)

// Per-scenario budgets: the wall clock covers the agent run plus the Docker
// image build and container run; the telemetry timeout leaves room for the
// SDK batch processors' export intervals.
const (
	scenarioTimeout  = 25 * time.Minute
	telemetryTimeout = 60 * time.Second
)

// Default returns the populated scenario registry: the harness default
// registry (rule-file classification) plus every scenario declared in this
// package. CI selection and the registry completeness test operate on it.
func Default() *harness.Registry {
	r := harness.DefaultRegistry()
	for _, sc := range All() {
		r.MustRegister(sc)
	}
	return r
}

// All returns every declared scenario in registration order: the base set
// declared in this file, followed by the sets that later units register from
// their own files via registerScenarios (see register.go), so landing a unit
// never edits this file again.
func All() []harness.Scenario {
	base := []harness.Scenario{GoHTTP(), GoLogs(), NodeHTTP()}
	return append(base, extraScenarios...)
}

// promptCommonRequirements is shared prompt text enforcing the placeholder
// contract: exporter configuration comes from the environment the harness
// composes at run time, never hardcoded by the agent.
const promptCommonRequirements = `
Requirements:
- Read exporter configuration from the standard OTEL_* environment variables; the runtime environment provides OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_EXPORTER_OTLP_PROTOCOL (http/protobuf), OTEL_EXPORTER_OTLP_HEADERS, and OTEL_RESOURCE_ATTRIBUTES. Do not hardcode endpoints, tokens, or resource attributes for them.
- Preserve the existing behavior: the service must keep serving GET /checkout, keep calling the URL from the DOWNSTREAM_URL environment variable while handling it, and keep honoring the PORT environment variable.
- The Dockerfile must keep building successfully; update it if the instrumentation needs it.`

// GoHTTP is the Go instrumentation happy-path scenario. It covers AE4: the
// assertion set includes service.name, so an agent run that omits it fails
// deterministically.
func GoHTTP() harness.Scenario {
	return harness.Scenario{
		ID:        GoHTTPID,
		Skill:     harness.SkillInstrumentation,
		RuleFiles: []string{"skills/otel-instrumentation/rules/sdks/go.md"},
		// The smoke scenario for otel-instrumentation: harness-code
		// changes select it.
		Smoke:       true,
		FixturePath: "evals/custom/fixtures/go-service",
		Prompt: `Instrument the Go HTTP service in the current directory with OpenTelemetry, using the otel-instrumentation skill (dash0-agent-skills:otel-instrumentation).

Instrumentation goals:
- Export traces via OTLP over http/protobuf.
- Set the service name to "` + GoServiceName + `".
- Produce a server span for the inbound GET /checkout request and a client span for the outbound HTTP call the handler makes.
` + promptCommonRequirements,
		Timeout:          scenarioTimeout,
		TelemetryTimeout: telemetryTimeout,
		Assert:           assertHTTPTraces(GoServiceName),
	}
}

// GoLogs asserts stdout-first log correlation, the delivery strategy logs.md
// prescribes: the fixture's structured single-line JSON logging on stdout is
// kept, the "checkout completed" record carries the active span's trace
// context — its (trace_id, span_id) pair must name a span that actually
// arrived at the sink — and application logs are NOT exported over OTLP.
// Agent0's A/B evals measured the skill regressing this task (3/3 unaided vs
// 1/3 with the skill loaded, both failures go compile errors) while the go
// rules carried no bridge-free correlation recipe; the recipe in go.md and
// this scenario land together.
func GoLogs() harness.Scenario {
	return harness.Scenario{
		ID:    GoLogsID,
		Skill: harness.SkillInstrumentation,
		RuleFiles: []string{
			"skills/otel-instrumentation/rules/sdks/go.md",
			"skills/otel-instrumentation/rules/logs.md",
		},
		FixturePath: "evals/custom/fixtures/go-service",
		Prompt: `Instrument the Go HTTP service in the current directory with OpenTelemetry, using the otel-instrumentation skill (dash0-agent-skills:otel-instrumentation).

Instrumentation goals:
- Export traces via OTLP over http/protobuf.
- Set the service name to "` + GoServiceName + `".
- Produce a server span for the inbound GET /checkout request and a client span for the outbound HTTP call the handler makes.
- Keep the service's structured single-line JSON logging on stdout, and make every record it logs while handling a request carry the trace context of the active span: the "checkout completed" record must include the active span's trace_id and span_id as fields.
- Do not export application logs over OTLP: stdout must remain the only delivery channel for application logs.
` + promptCommonRequirements,
		Timeout:          scenarioTimeout,
		TelemetryTimeout: telemetryTimeout,
		Assert:           assertHTTPTraces(GoServiceName),
		AssertApp:        assertStdoutLogCorrelation(GoServiceName, "checkout completed"),
	}
}

// NodeHTTP is the Node.js instrumentation happy-path scenario.
func NodeHTTP() harness.Scenario {
	return harness.Scenario{
		ID:          NodeHTTPID,
		Skill:       harness.SkillInstrumentation,
		RuleFiles:   []string{"skills/otel-instrumentation/rules/sdks/nodejs.md"},
		FixturePath: "evals/custom/fixtures/nodejs-service",
		Prompt: `Instrument the Node.js HTTP service in the current directory with OpenTelemetry, using the otel-instrumentation skill (dash0-agent-skills:otel-instrumentation).

Instrumentation goals:
- Export traces via OTLP over http/protobuf.
- Set the service name to "` + NodeServiceName + `".
- Produce a server span for the inbound GET /checkout request and a client span for the outbound HTTP call the handler makes.
` + promptCommonRequirements,
		Timeout:          scenarioTimeout,
		TelemetryTimeout: telemetryTimeout,
		Assert:           assertHTTPTraces(NodeServiceName),
	}
}

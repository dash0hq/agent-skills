// Package scenarios declares the eval scenarios and registers them on top of
// the harness registry (see evals/harness). Scenario definitions are data:
// the task prompt given to the agent, the fixture it runs against, the rule
// files it covers, and the deterministic assertions over the telemetry that
// reaches the sink. The Go test entrypoint that executes them against the
// real agent and Docker lives in run_test.go.
package scenarios

import (
	"time"

	"github.com/dash0hq/agent-skills/evals/harness"
)

// Scenario IDs. They are the keys used by quarantine.yaml, CI selection
// output, and the -run / EVAL_SCENARIOS filters of the test entrypoint.
const (
	// GoHTTPID is the Go instrumentation happy path: traces with the
	// expected service.name, a server span, and a client span.
	GoHTTPID = "instr-go-http"
	// GoLogsID verifies application logs reach the sink over OTLP. Real
	// runs pass: the agent wires the log/slog -> OTel bridge itself even
	// though go.md does not spell it out, so the scenario does not by
	// itself prove the skill documents the bridge (that gap is tracked in
	// TODO.md).
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
		FixturePath: "evals/fixtures/go-service",
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

// GoLogs asserts that the application logs the service emits through log/slog
// are exported over OTLP and arrive at the sink. Real runs pass: a capable
// agent wires the slog -> OTel logger-provider bridge itself, even though
// go.md does not document it, so this scenario confirms working log telemetry
// but does not verify the skill teaches the bridge (the TODO.md gap).
func GoLogs() harness.Scenario {
	return harness.Scenario{
		ID:    GoLogsID,
		Skill: harness.SkillInstrumentation,
		RuleFiles: []string{
			"skills/otel-instrumentation/rules/sdks/go.md",
			"skills/otel-instrumentation/rules/logs.md",
		},
		FixturePath: "evals/fixtures/go-service",
		Prompt: `Instrument the Go HTTP service in the current directory with OpenTelemetry, using the otel-instrumentation skill (dash0-agent-skills:otel-instrumentation).

Instrumentation goals:
- Export traces and logs via OTLP over http/protobuf.
- Set the service name to "` + GoServiceName + `".
- Produce a server span for the inbound GET /checkout request and a client span for the outbound HTTP call the handler makes.
- The application logs the service emits through log/slog (for example the "checkout completed" record) must be exported over OTLP and arrive at the same endpoint as the traces.
` + promptCommonRequirements,
		Timeout:          scenarioTimeout,
		TelemetryTimeout: telemetryTimeout,
		Assert:           assertLogsPresent(GoServiceName),
	}
}

// NodeHTTP is the Node.js instrumentation happy-path scenario.
func NodeHTTP() harness.Scenario {
	return harness.Scenario{
		ID:          NodeHTTPID,
		Skill:       harness.SkillInstrumentation,
		RuleFiles:   []string{"skills/otel-instrumentation/rules/sdks/nodejs.md"},
		FixturePath: "evals/fixtures/nodejs-service",
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

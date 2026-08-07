// The Ruby SDK instrumentation scenario: the per-language HTTP happy path
// built on the shared sdkHTTPScenario helper (see instr_sdks.go).
package scenarios

import "github.com/dash0hq/agent-skills/evals/custom/harness"

func init() {
	registerScenarios(RubyHTTP())
}

// RubyHTTPID is the Ruby (Rack on WEBrick) instrumentation happy path (see
// scenarios.go for how IDs are used).
const RubyHTTPID = "instr-ruby-http"

// RubyServiceName is the service.name the Ruby scenario's prompt demands and
// its assertions check (AE4).
const RubyServiceName = "ruby-service-eval"

// RubyHTTP is the Ruby instrumentation happy-path scenario. The fixture is a
// plain Rack application served by WEBrick via rackup.
func RubyHTTP() harness.Scenario {
	return sdkHTTPScenario(
		RubyHTTPID,
		"skills/otel-instrumentation/rules/sdks/ruby.md",
		"evals/custom/fixtures/ruby-service",
		"Ruby HTTP service (a plain Rack application served by WEBrick via rackup)",
		RubyServiceName,
		`- The service runs as the container built by the Dockerfile; make the instrumentation take effect there (for example by adjusting the Gemfile and initializing the SDK before the Rack application starts).`,
		scenarioTimeout,
	)
}

// The Next.js SDK instrumentation scenario: the per-language HTTP happy path
// built on the shared sdkHTTPScenario helper (see instr_sdks.go).
package scenarios

import "github.com/dash0hq/agent-skills/evals/custom/harness"

func init() {
	registerScenarios(NextJSHTTP())
}

// NextJSHTTPID is the Next.js (app router) instrumentation happy path; the
// fixture's outbound call happens server-side in a route handler (see
// scenarios.go for how IDs are used).
const NextJSHTTPID = "instr-nextjs-http"

// NextJSServiceName is the service.name the Next.js scenario's prompt demands
// and its assertions check (AE4).
const NextJSServiceName = "nextjs-service-eval"

// NextJSHTTP is the Next.js instrumentation happy-path scenario. The
// assertions target server-side telemetry: the route handler's server span
// and the client span of its outbound fetch.
func NextJSHTTP() harness.Scenario {
	return sdkHTTPScenario(
		NextJSHTTPID,
		"skills/otel-instrumentation/rules/sdks/nextjs.md",
		"evals/custom/fixtures/nextjs-service",
		"Next.js app-router application (the GET /checkout route handler makes the outbound call server-side)",
		NextJSServiceName,
		`- Only server-side instrumentation is verified in this task: the server span of the GET /checkout route handler and the client span of its outbound fetch. Client (browser) instrumentation is not exercised.
- The service runs as the container built by the Dockerfile; make the instrumentation take effect in the production build ("next build" followed by "next start").`,
		heavyScenarioTimeout,
	)
}

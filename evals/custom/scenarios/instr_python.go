// The Python SDK instrumentation scenario: the per-language HTTP happy path
// built on the shared sdkHTTPScenario helper (see instr_sdks.go).
package scenarios

import "github.com/dash0hq/agent-skills/evals/custom/harness"

func init() {
	registerScenarios(PythonHTTP())
}

// PythonHTTPID is the Python (Flask) instrumentation happy path (see
// scenarios.go for how IDs are used).
const PythonHTTPID = "instr-python-http"

// PythonServiceName is the service.name the Python scenario's prompt demands
// and its assertions check (AE4).
const PythonServiceName = "python-service-eval"

// PythonHTTP is the Python instrumentation happy-path scenario.
func PythonHTTP() harness.Scenario {
	return sdkHTTPScenario(
		PythonHTTPID,
		"skills/otel-instrumentation/rules/sdks/python.md",
		"evals/custom/fixtures/python-service",
		"Python Flask HTTP service",
		PythonServiceName,
		`- The service runs as the container built by the Dockerfile; make the instrumentation take effect there (for example by adjusting the dependency list and the container command).`,
		scenarioTimeout,
	)
}

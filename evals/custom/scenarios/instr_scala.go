// The Scala SDK instrumentation scenario: the per-language HTTP happy path
// built on the shared sdkHTTPScenario helper (see instr_sdks.go).
package scenarios

import "github.com/dash0hq/agent-skills/evals/custom/harness"

func init() {
	registerScenarios(ScalaHTTP())
}

// ScalaHTTPID is the Scala (sbt, javaagent) instrumentation happy path (see
// scenarios.go for how IDs are used).
const ScalaHTTPID = "instr-scala-http"

// ScalaServiceName is the service.name the Scala scenario's prompt demands
// and its assertions check (AE4).
const ScalaServiceName = "scala-service-eval"

// ScalaHTTP is the Scala instrumentation happy-path scenario. The fixture is
// an sbt project run via "sbt run" in the container, so sbt-level agent
// wiring (as the Scala rule file teaches) takes effect at start.
func ScalaHTTP() harness.Scenario {
	return sdkHTTPScenario(
		ScalaHTTPID,
		"skills/otel-instrumentation/rules/sdks/scala.md",
		"evals/custom/fixtures/scala-service",
		"Scala HTTP service (an sbt project; the container runs it via \"sbt run\")",
		ScalaServiceName,
		`- The service runs as the container built by the Dockerfile via "sbt run"; make the instrumentation take effect there (the build may resolve new dependencies, but the running container has no network access beyond the telemetry endpoint).`,
		heavyScenarioTimeout,
	)
}

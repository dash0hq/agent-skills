// The Java SDK instrumentation scenario: the per-language HTTP happy path
// built on the shared sdkHTTPScenario helper (see instr_sdks.go).
package scenarios

import "github.com/dash0hq/agent-skills/evals/custom/harness"

func init() {
	registerScenarios(JavaHTTP())
}

// JavaHTTPID is the Java (javaagent) instrumentation happy path (see
// scenarios.go for how IDs are used).
const JavaHTTPID = "instr-java-http"

// JavaServiceName is the service.name the Java scenario's prompt demands and
// its assertions check (AE4).
const JavaServiceName = "java-service-eval"

// JavaHTTP is the Java instrumentation happy-path scenario. The fixture is a
// Spring Boot Web service (embedded Tomcat plus Spring WebMVC) built with Maven.
func JavaHTTP() harness.Scenario {
	return sdkHTTPScenario(
		JavaHTTPID,
		"skills/otel-instrumentation/rules/sdks/java.md",
		"evals/custom/fixtures/java-service",
		"Java HTTP service (Maven build, Spring Boot Web)",
		JavaServiceName,
		`- The service runs as the container built by the Dockerfile; make the instrumentation take effect there (for example by fetching what the instrumentation needs during the image build and wiring it into the container entrypoint).`,
		heavyScenarioTimeout,
	)
}

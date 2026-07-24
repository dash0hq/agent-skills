// The U4 SDK instrumentation scenarios: the remaining 8 SDK rule files after
// the U3 Go and Node.js proving ground. This file declares the additive
// registration mechanism (registerScenarios) plus the 6 straightforward
// per-language HTTP happy paths; the .NET scenarios (including the 2 TODO.md
// regressions) live in instr_dotnet.go and the browser scenario in
// instr_browser.go.
package scenarios

import (
	"time"

	"github.com/dash0hq/agent-skills/evals/harness"
)

func init() {
	registerScenarios(
		PythonHTTP(), JavaHTTP(), RubyHTTP(), PHPHTTP(), ScalaHTTP(),
		DotnetHTTP(), DotnetNuGet(), DotnetEnrichment(),
		NextJSHTTP(), BrowserHTTP(),
	)
}

// Scenario IDs of the U4 per-language HTTP happy paths (see scenarios.go for
// how IDs are used).
const (
	// PythonHTTPID is the Python (Flask) instrumentation happy path.
	PythonHTTPID = "instr-python-http"
	// JavaHTTPID is the Java (javaagent) instrumentation happy path.
	JavaHTTPID = "instr-java-http"
	// RubyHTTPID is the Ruby (Rack on WEBrick) instrumentation happy path.
	RubyHTTPID = "instr-ruby-http"
	// PHPHTTPID is the PHP (built-in server) instrumentation happy path.
	PHPHTTPID = "instr-php-http"
	// ScalaHTTPID is the Scala (sbt, javaagent) instrumentation happy path.
	ScalaHTTPID = "instr-scala-http"
	// NextJSHTTPID is the Next.js (app router) instrumentation happy path;
	// the fixture's outbound call happens server-side in a route handler.
	NextJSHTTPID = "instr-nextjs-http"
)

// Service names the U4 prompts demand and the assertions check (AE4).
const (
	// PythonServiceName is the service.name the Python scenario requires.
	PythonServiceName = "python-service-eval"
	// JavaServiceName is the service.name the Java scenario requires.
	JavaServiceName = "java-service-eval"
	// RubyServiceName is the service.name the Ruby scenario requires.
	RubyServiceName = "ruby-service-eval"
	// PHPServiceName is the service.name the PHP scenario requires.
	PHPServiceName = "php-service-eval"
	// ScalaServiceName is the service.name the Scala scenario requires.
	ScalaServiceName = "scala-service-eval"
	// NextJSServiceName is the service.name the Next.js scenario requires.
	NextJSServiceName = "nextjs-service-eval"
)

// heavyScenarioTimeout is the wall-clock budget for the scenarios whose
// Docker builds are slow (JVM dependency resolution, sbt, Next.js production
// builds, dotnet restore, PHP extension compilation, browser bundling).
const heavyScenarioTimeout = 30 * time.Minute

// sdkHTTPScenario builds one per-language HTTP happy-path scenario following
// the instr-go-http pattern: the agent instruments the fixture, and the
// assertions demand the declared service.name (AE4), a server span for
// GET /checkout, and a client span for the outbound call.
func sdkHTTPScenario(id, ruleFile, fixturePath, description, serviceName, languageNote string, timeout time.Duration) harness.Scenario {
	prompt := `Instrument the ` + description + ` in the current directory with OpenTelemetry, using the otel-instrumentation skill (dash0-agent-skills:otel-instrumentation).

Instrumentation goals:
- Export traces via OTLP over http/protobuf.
- Set the service name to "` + serviceName + `".
- Produce a server span for the inbound GET /checkout request and a client span for the outbound HTTP call the handler makes.
` + promptCommonRequirements
	if languageNote != "" {
		prompt += "\n" + languageNote
	}
	return harness.Scenario{
		ID:               id,
		Skill:            harness.SkillInstrumentation,
		RuleFiles:        []string{ruleFile},
		FixturePath:      fixturePath,
		Prompt:           prompt,
		Timeout:          timeout,
		TelemetryTimeout: telemetryTimeout,
		Assert:           assertHTTPTraces(serviceName),
	}
}

// PythonHTTP is the Python instrumentation happy-path scenario.
func PythonHTTP() harness.Scenario {
	return sdkHTTPScenario(
		PythonHTTPID,
		"skills/otel-instrumentation/rules/sdks/python.md",
		"evals/fixtures/python-service",
		"Python Flask HTTP service",
		PythonServiceName,
		`- The service runs as the container built by the Dockerfile; make the instrumentation take effect there (for example by adjusting the dependency list and the container command).`,
		scenarioTimeout,
	)
}

// JavaHTTP is the Java instrumentation happy-path scenario. The fixture is a
// Spring Boot Web service (embedded Tomcat plus Spring WebMVC) built with Maven.
func JavaHTTP() harness.Scenario {
	return sdkHTTPScenario(
		JavaHTTPID,
		"skills/otel-instrumentation/rules/sdks/java.md",
		"evals/fixtures/java-service",
		"Java HTTP service (Maven build, Spring Boot Web)",
		JavaServiceName,
		`- The service runs as the container built by the Dockerfile; make the instrumentation take effect there (for example by fetching what the instrumentation needs during the image build and wiring it into the container entrypoint).`,
		heavyScenarioTimeout,
	)
}

// RubyHTTP is the Ruby instrumentation happy-path scenario. The fixture is a
// plain Rack application served by WEBrick via rackup.
func RubyHTTP() harness.Scenario {
	return sdkHTTPScenario(
		RubyHTTPID,
		"skills/otel-instrumentation/rules/sdks/ruby.md",
		"evals/fixtures/ruby-service",
		"Ruby HTTP service (a plain Rack application served by WEBrick via rackup)",
		RubyServiceName,
		`- The service runs as the container built by the Dockerfile; make the instrumentation take effect there (for example by adjusting the Gemfile and initializing the SDK before the Rack application starts).`,
		scenarioTimeout,
	)
}

// PHPHTTP is the PHP instrumentation happy-path scenario. The fixture is a
// single index.php served by the PHP built-in web server.
func PHPHTTP() harness.Scenario {
	return sdkHTTPScenario(
		PHPHTTPID,
		"skills/otel-instrumentation/rules/sdks/php.md",
		"evals/fixtures/php-service",
		"PHP HTTP service (a single index.php served by the PHP built-in web server)",
		PHPServiceName,
		`- The service runs as the container built by the Dockerfile; make the instrumentation take effect there (for example by installing Composer packages and any required PHP extension during the image build).`,
		heavyScenarioTimeout,
	)
}

// ScalaHTTP is the Scala instrumentation happy-path scenario. The fixture is
// an sbt project run via "sbt run" in the container, so sbt-level agent
// wiring (as the Scala rule file teaches) takes effect at start.
func ScalaHTTP() harness.Scenario {
	return sdkHTTPScenario(
		ScalaHTTPID,
		"skills/otel-instrumentation/rules/sdks/scala.md",
		"evals/fixtures/scala-service",
		"Scala HTTP service (an sbt project; the container runs it via \"sbt run\")",
		ScalaServiceName,
		`- The service runs as the container built by the Dockerfile via "sbt run"; make the instrumentation take effect there (the build may resolve new dependencies, but the running container has no network access beyond the telemetry endpoint).`,
		heavyScenarioTimeout,
	)
}

// NextJSHTTP is the Next.js instrumentation happy-path scenario. The
// assertions target server-side telemetry: the route handler's server span
// and the client span of its outbound fetch.
func NextJSHTTP() harness.Scenario {
	return sdkHTTPScenario(
		NextJSHTTPID,
		"skills/otel-instrumentation/rules/sdks/nextjs.md",
		"evals/fixtures/nextjs-service",
		"Next.js app-router application (the GET /checkout route handler makes the outbound call server-side)",
		NextJSServiceName,
		`- Only server-side instrumentation is verified in this task: the server span of the GET /checkout route handler and the client span of its outbound fetch. Client (browser) instrumentation is not exercised.
- The service runs as the container built by the Dockerfile; make the instrumentation take effect in the production build ("next build" followed by "next start").`,
		heavyScenarioTimeout,
	)
}

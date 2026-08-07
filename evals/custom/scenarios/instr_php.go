// The PHP SDK instrumentation scenario: the per-language HTTP happy path
// built on the shared sdkHTTPScenario helper (see instr_sdks.go).
package scenarios

import "github.com/dash0hq/agent-skills/evals/custom/harness"

func init() {
	registerScenarios(PHPHTTP())
}

// PHPHTTPID is the PHP (built-in server) instrumentation happy path (see
// scenarios.go for how IDs are used).
const PHPHTTPID = "instr-php-http"

// PHPServiceName is the service.name the PHP scenario's prompt demands and
// its assertions check (AE4).
const PHPServiceName = "php-service-eval"

// PHPHTTP is the PHP instrumentation happy-path scenario. The fixture is a
// single index.php served by the PHP built-in web server.
func PHPHTTP() harness.Scenario {
	return sdkHTTPScenario(
		PHPHTTPID,
		"skills/otel-instrumentation/rules/sdks/php.md",
		"evals/custom/fixtures/php-service",
		"PHP HTTP service (a single index.php served by the PHP built-in web server)",
		PHPServiceName,
		`- The service runs as the container built by the Dockerfile; make the instrumentation take effect there (for example by installing Composer packages and any required PHP extension during the image build).`,
		heavyScenarioTimeout,
	)
}

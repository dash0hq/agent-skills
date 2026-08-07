// Shared helpers of the per-language SDK instrumentation scenarios. Each
// language declares its scenarios in its own instr_<language>.go file and
// self-registers them via registerScenarios (see register.go); this file
// carries only what those files share: the heavy-build timeout and the
// sdkHTTPScenario happy-path builder.
package scenarios

import (
	"time"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
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

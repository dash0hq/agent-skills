// The verify-dependencies scenario: exercises the rule that instrumentation
// packages and versions must be verified against the package registry before
// being added or bumped, never written from memory
// (skills/otel-instrumentation/rules/verify-dependencies.md).
//
// The fixture is a Python Flask service pre-instrumented with the 1.43.0 /
// 0.64b0 OpenTelemetry release line, including the elasticsearch
// instrumentation — which upstream retired at exactly that line: 0.64b0
// (June 2026) is its last release, it exact-pins
// opentelemetry-instrumentation==0.64b0, and opentelemetry-bootstrap in the
// current contrib line no longer maps the elasticsearch package at all. The
// task is an upgrade to the newest SDK, so every path must confront the
// retired package, and memory-written bumps fail deterministically at image
// build time:
//
//   - bumping every opentelemetry-instrumentation-* to the current contrib
//     version includes an elasticsearch release that does not exist on PyPI,
//     and pip fails with "No matching distribution found";
//   - bumping the rest while keeping elasticsearch at 0.64b0 trips its exact
//     pins against the upgraded core packages, and pip fails resolution.
//
// An agent following the rule (verify each pin against the registry, notice
// the retired instrumentation, drop it — falling back to manual
// instrumentation for the library — and upgrade the remainder) produces a
// working build; the assertion additionally requires telemetry.sdk.version
// to have moved off the fixture's pinned 1.43.0, so an agent that changes
// nothing (or resolves the conflict by staying on the old line) fails.
// Interactively the rule requires confirming the drop with the user first,
// because removed instrumentation is telemetry that disappears; this
// headless run exercises the rule's non-interactive fallback (proceed only
// because the task cannot complete otherwise, and report the removal).
package scenarios

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

func init() {
	registerScenarios(PythonRetiredInstrumentationUpgrade())
}

// PythonRetiredInstrumentationUpgradeID is the verify-dependencies scenario
// built around upgrading past the retired Python elasticsearch
// instrumentation.
const PythonRetiredInstrumentationUpgradeID = "upgrade-python-retired-instrumentation"

// PythonRetiredServiceName is the service.name the scenario's prompt demands
// and its assertions check.
const PythonRetiredServiceName = "python-search-service-eval"

// pythonRetiredFixtureSDKVersion is the opentelemetry-sdk version the fixture
// pins (the 0.64b0 contrib line). The assertion requires the exported
// telemetry.sdk.version to differ from it, proving the upgrade happened
// without hardcoding what "newest" is at eval time.
const pythonRetiredFixtureSDKVersion = "1.43.0"

// PythonRetiredInstrumentationUpgrade is the verify-dependencies scenario:
// the fixture's dependency set contains a retired instrumentation package,
// and only a registry-verified upgrade builds and moves the SDK version.
func PythonRetiredInstrumentationUpgrade() harness.Scenario {
	return harness.Scenario{
		ID:    PythonRetiredInstrumentationUpgradeID,
		Skill: harness.SkillInstrumentation,
		RuleFiles: []string{
			"skills/otel-instrumentation/rules/verify-dependencies.md",
			"skills/otel-instrumentation/rules/sdks/python.md",
		},
		FixturePath: "evals/custom/fixtures/python-elasticsearch-service",
		Prompt: `The Python Flask HTTP service in the current directory is already instrumented with OpenTelemetry through zero-code auto-instrumentation: the dependencies are pinned in requirements.txt and the container command runs opentelemetry-instrument. Upgrade its OpenTelemetry dependencies to the newest published releases, using the otel-instrumentation skill (dash0-agent-skills:otel-instrumentation).

Upgrade goals:
- Move to the newest published OpenTelemetry SDK and matching instrumentation releases.
- After the upgrade the service must still export traces via OTLP over http/protobuf, with the service name "` + PythonRetiredServiceName + `", a server span for the inbound GET /checkout request, and a client span for the outbound HTTP call the handler makes.
- Keep the application code and its functionality intact.
` + promptCommonRequirements + `
- The service runs as the container built by the Dockerfile; make the upgrade take effect there.`,
		Timeout:          scenarioTimeout,
		TelemetryTimeout: telemetryTimeout,
		Assert:           assertUpgradedHTTPTraces(PythonRetiredServiceName, pythonRetiredFixtureSDKVersion),
	}
}

// assertUpgradedHTTPTraces wraps assertHTTPTraces and additionally requires
// the service's spans to carry a telemetry.sdk.version resource attribute
// different from the fixture's pre-upgrade pin: telemetry that still (or
// again) reports the old SDK version means the upgrade did not happen.
func assertUpgradedHTTPTraces(serviceName, oldSDKVersion string) harness.Assertion {
	httpAssert := assertHTTPTraces(serviceName)
	return func(t *testing.T, sink *otelsink.Sink) error {
		if err := httpAssert(t, sink); err != nil {
			return err
		}
		svc := sink.Traces(t).WithResourceAttribute("service.name", serviceName)
		version := ""
		for _, sv := range svc.Spans() {
			version = attrValue(sv.Resource.GetAttributes(), "telemetry.sdk.version")
			if version != "" {
				break
			}
		}
		if version == "" {
			return fmt.Errorf("no telemetry.sdk.version resource attribute on the spans of service.name=%q", serviceName)
		}
		if version == oldSDKVersion {
			return fmt.Errorf("telemetry.sdk.version is still %q, the fixture's pre-upgrade pin: the OpenTelemetry upgrade did not take effect", oldSDKVersion)
		}
		return nil
	}
}

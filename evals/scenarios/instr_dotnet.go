// The .NET instrumentation scenarios: the per-language HTTP happy path plus
// the 2 regression scenarios recorded in TODO.md — the NuGet-package setup
// route (the documented install script is not the only viable path) and
// enrichment of the auto-instrumented server span via Activity.Current.
package scenarios

import (
	"fmt"
	"strings"
	"testing"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/harness"
)

// Scenario IDs of the .NET scenarios.
const (
	// DotnetHTTPID is the .NET instrumentation happy path.
	DotnetHTTPID = "instr-dotnet-http"
	// DotnetNuGetID pins the NuGet-package setup route from TODO.md: the
	// install script is forbidden, so the scenario fails while the .NET
	// rule file documents no package-based path.
	DotnetNuGetID = "instr-dotnet-nuget"
	// DotnetEnrichmentID is the TODO.md enrichment regression: the task
	// demands order.id on the auto-instrumented SERVER span itself (the
	// Activity.Current pattern), not on a new child span.
	DotnetEnrichmentID = "instr-dotnet-enrichment"
)

// DotnetServiceName is the service.name the .NET scenarios require.
const DotnetServiceName = "dotnet-service-eval"

// dotnetRuleFile is the dedicated rule file all 3 .NET scenarios cover.
const dotnetRuleFile = "skills/otel-instrumentation/rules/sdks/dotnet.md"

// dotnetContainerNote reminds the agent that the instrumentation must be
// effective inside the container the Dockerfile builds.
const dotnetContainerNote = `- The service runs as the container built by the Dockerfile; make the instrumentation take effect there.`

// DotnetHTTP is the .NET instrumentation happy-path scenario: an ASP.NET Core
// minimal API, no setup route pinned.
func DotnetHTTP() harness.Scenario {
	return sdkHTTPScenario(
		DotnetHTTPID,
		dotnetRuleFile,
		"evals/fixtures/dotnet-service",
		".NET service (an ASP.NET Core minimal API)",
		DotnetServiceName,
		dotnetContainerNote,
		heavyScenarioTimeout,
	)
}

// DotnetNuGet pins the NuGet-package setup route: the TODO.md regression
// where the skill documents only the zero-code install script, leaving an
// agent with no package-based path. The prompt forbids the script; the
// scenario passes only when the skill-guided agent produces working traces
// through NuGet packages configured in code.
func DotnetNuGet() harness.Scenario {
	sc := sdkHTTPScenario(
		DotnetNuGetID,
		dotnetRuleFile,
		"evals/fixtures/dotnet-service",
		".NET service (an ASP.NET Core minimal API)",
		DotnetServiceName,
		dotnetContainerNote+`
- Set up OpenTelemetry through NuGet packages added to the project and configured in code (the SDK route).
- Do not download or run the otel-dotnet-auto-install.sh script or any other zero-code auto-instrumentation installer.`,
		heavyScenarioTimeout,
	)
	return sc
}

// DotnetEnrichment is the TODO.md enrichment regression: business attributes
// belong on the auto-instrumented SERVER span (Activity.Current), and the
// assertion rejects runs that only put order.id on a child span.
func DotnetEnrichment() harness.Scenario {
	sc := sdkHTTPScenario(
		DotnetEnrichmentID,
		dotnetRuleFile,
		"evals/fixtures/dotnet-service",
		".NET service (an ASP.NET Core minimal API)",
		DotnetServiceName,
		dotnetContainerNote+`
- In the GET /checkout handler, add the attribute order.id with the value "TEST-0001" to the auto-instrumented server span of the request itself. Do not create a new child span to carry it.`,
		heavyScenarioTimeout,
	)
	// The enrichment task also exercises the shared guidance on adding
	// attributes to auto-instrumented spans.
	sc.RuleFiles = append(sc.RuleFiles, "skills/otel-instrumentation/rules/spans.md")
	sc.Assert = assertServerSpanAttribute(DotnetServiceName, "order.id", "TEST-0001")
	return sc
}

// assertServerSpanAttribute builds the enrichment assertion: on top of the
// shared HTTP-traces assertion, the SERVER span for GET /checkout itself must
// carry the given attribute value. Checking only SERVER spans is what rejects
// the child-span shortcut the TODO.md regression describes.
func assertServerSpanAttribute(serviceName, key, want string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		if err := assertHTTPTraces(serviceName)(t, sink); err != nil {
			return err
		}
		svc := sink.Traces(t).WithResourceAttribute("service.name", serviceName)
		for _, sv := range svc.WithKind(tracepb.Span_SPAN_KIND_SERVER).Spans() {
			if !spanReferencesPath(sv.Span, "/checkout") {
				continue
			}
			if attrValue(sv.Span.GetAttributes(), key) == want {
				return nil
			}
		}
		return fmt.Errorf("no SERVER span for GET /checkout with service.name=%q carries %s=%q on the span itself (the attribute must enrich the auto-instrumented server span, not a child span)", serviceName, key, want)
	}
}

// spanReferencesPath reports whether the span's name or one of its
// path-bearing HTTP attributes contains pathSubstr.
func spanReferencesPath(span *tracepb.Span, pathSubstr string) bool {
	if strings.Contains(span.GetName(), pathSubstr) {
		return true
	}
	return strings.Contains(attrValue(span.GetAttributes(), httpPathAttributes...), pathSubstr)
}

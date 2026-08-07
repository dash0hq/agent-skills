package scenarios

import (
	"testing"

	"github.com/stretchr/testify/require"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"
)

// The enrichment assertion tests feed the sink through its real OTLP/HTTP
// endpoint in-process (no Docker, no agent) and evaluate
// assertServerSpanAttribute on what arrived.

func TestAssertServerSpanAttributePassesWhenServerSpanCarriesIt(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": DotnetServiceName},
		serverSpan(map[string]string{
			"http.request.method": "GET",
			"url.path":            "/checkout",
			"order.id":            "TEST-0001",
		}),
		clientSpan(map[string]string{"http.request.method": "GET", "url.full": "http://downstream:9090/inventory"}),
	)
	require.NoError(t, assertServerSpanAttribute(DotnetServiceName, "order.id", "TEST-0001")(t, sink))
}

// The manually-found regression shape: the agent creates a child span to carry the
// business attribute instead of enriching the auto-instrumented SERVER span
// via Activity.Current. The assertion must reject it.
func TestAssertServerSpanAttributeRejectsTheChildSpanShortcut(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": DotnetServiceName},
		serverSpan(map[string]string{
			"http.request.method": "GET",
			"url.path":            "/checkout",
		}),
		clientSpan(map[string]string{"http.request.method": "GET", "url.full": "http://downstream:9090/inventory"}),
		spanSpec{
			name: "checkout order",
			kind: tracepb.Span_SPAN_KIND_INTERNAL,
			attrs: map[string]string{
				"url.path": "/checkout",
				"order.id": "TEST-0001",
			},
		},
	)
	err := assertServerSpanAttribute(DotnetServiceName, "order.id", "TEST-0001")(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "order.id")
	require.Contains(t, err.Error(), "not a child span")
}

func TestAssertServerSpanAttributeRejectsWrongValue(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": DotnetServiceName},
		serverSpan(map[string]string{
			"http.request.method": "GET",
			"url.path":            "/checkout",
			"order.id":            "TEST-9999",
		}),
		clientSpan(map[string]string{"http.request.method": "GET", "url.full": "http://downstream:9090/inventory"}),
	)
	err := assertServerSpanAttribute(DotnetServiceName, "order.id", "TEST-0001")(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TEST-0001")
}

// assertServerSpanAttribute layers on assertHTTPTraces: with no client span,
// the shared HTTP assertion fails first even when the server span carries the
// attribute.
func TestAssertServerSpanAttributeStillRequiresTheClientSpan(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": DotnetServiceName},
		serverSpan(map[string]string{
			"http.request.method": "GET",
			"url.path":            "/checkout",
			"order.id":            "TEST-0001",
		}),
	)
	err := assertServerSpanAttribute(DotnetServiceName, "order.id", "TEST-0001")(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "CLIENT")
}

// The NuGet scenario shares the happy-path assertions but must pin the
// package route in its prompt and forbid the install script.
func TestDotnetNuGetScenarioPinsThePackageRoute(t *testing.T) {
	sc := DotnetNuGet()
	require.Contains(t, sc.Prompt, "NuGet")
	require.Contains(t, sc.Prompt, "Do not download or run the otel-dotnet-auto-install.sh script")
	require.NotNil(t, sc.Assert)
	require.Equal(t, []string{dotnetRuleFile}, sc.RuleFiles)
}

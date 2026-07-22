package scenarios

import (
	"testing"

	"github.com/stretchr/testify/require"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"
)

// The browser assertion tests feed the sink through its real OTLP/HTTP
// endpoint in-process — the same signal path the page's exporter uses through
// the relay — and evaluate assertBrowserSpans on what arrived.

func TestAssertBrowserSpansPassesOnPageSpans(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": BrowserServiceName},
		spanSpec{name: "documentLoad", kind: tracepb.Span_SPAN_KIND_INTERNAL},
		spanSpec{
			name: "HTTP GET",
			kind: tracepb.Span_SPAN_KIND_CLIENT,
			attrs: map[string]string{
				"http.method": "GET",
				"http.url":    "http://app:8080/checkout-data",
			},
		},
	)
	require.NoError(t, assertBrowserSpans(BrowserServiceName)(t, sink))
}

// A span whose name carries the fetched path (no URL attribute) also
// satisfies the assertion: browser instrumentations differ in where they
// record the target.
func TestAssertBrowserSpansAcceptsThePathInTheSpanName(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": BrowserServiceName},
		spanSpec{name: "GET /checkout-data", kind: tracepb.Span_SPAN_KIND_CLIENT},
	)
	require.NoError(t, assertBrowserSpans(BrowserServiceName)(t, sink))
}

func TestAssertBrowserSpansFailsWithoutAnySpans(t *testing.T) {
	sink := otelsink.Start(t)
	err := assertBrowserSpans(BrowserServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), BrowserServiceName)
	require.Contains(t, err.Error(), "service.name")
}

// Server-side spans under another service.name (for example an accidentally
// instrumented server.js) must not satisfy the browser assertion.
func TestAssertBrowserSpansIgnoresOtherServices(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": "server-side-service"},
		serverSpan(map[string]string{"http.request.method": "GET", "url.path": "/checkout-data"}),
	)
	err := assertBrowserSpans(BrowserServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), BrowserServiceName)
}

// Page spans that never reference the /checkout-data fetch (for example only
// a document load) do not prove the page's activity was captured end to end.
func TestAssertBrowserSpansRequiresTheCheckoutDataFetch(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": BrowserServiceName},
		spanSpec{name: "documentLoad", kind: tracepb.Span_SPAN_KIND_INTERNAL},
	)
	err := assertBrowserSpans(BrowserServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "/checkout-data")
}

// fixtureHooks routes the browser scenario to the Chromium-driven topology
// and every other scenario to the HTTP-probe topology.
func TestFixtureHooksSelectsTheBrowserTopology(t *testing.T) {
	fix := NewDockerFixture(t, t.TempDir())

	browser := fixtureHooks(fix, BrowserHTTP())
	require.NotNil(t, browser.Build)
	require.NotNil(t, browser.Run)

	other := fixtureHooks(fix, GoHTTP())
	require.NotNil(t, other.Build)
	require.NotNil(t, other.Run)
}

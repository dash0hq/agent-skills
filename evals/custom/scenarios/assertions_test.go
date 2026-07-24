package scenarios

import (
	"testing"

	"github.com/stretchr/testify/require"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"
)

// Assertion tests feed the sink through its real OTLP/HTTP endpoints
// in-process and then evaluate the scenario assertions on what arrived.

func serverSpan(attrs map[string]string) spanSpec {
	return spanSpec{name: "GET /checkout", kind: tracepb.Span_SPAN_KIND_SERVER, attrs: attrs}
}

func clientSpan(attrs map[string]string) spanSpec {
	return spanSpec{name: "GET", kind: tracepb.Span_SPAN_KIND_CLIENT, attrs: attrs}
}

func TestAssertHTTPTracesPassesOnStableSemconv(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": GoServiceName},
		serverSpan(map[string]string{"http.request.method": "GET", "url.path": "/checkout"}),
		clientSpan(map[string]string{"http.request.method": "GET", "url.full": "http://downstream:9090/inventory"}),
	)
	require.NoError(t, assertHTTPTraces(GoServiceName)(t, sink))
}

func TestAssertHTTPTracesPassesOnLegacySemconv(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": NodeServiceName},
		serverSpan(map[string]string{"http.method": "GET", "http.target": "/checkout"}),
		clientSpan(map[string]string{"http.method": "GET", "http.url": "http://downstream:9090/inventory"}),
	)
	require.NoError(t, assertHTTPTraces(NodeServiceName)(t, sink))
}

// Covers AE4: the assertion set includes service.name, so telemetry arriving
// under the wrong (or default) service name fails deterministically.
func TestAssertHTTPTracesFailsOnWrongServiceName(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": "unknown_service:go"},
		serverSpan(map[string]string{"http.request.method": "GET", "url.path": "/checkout"}),
		clientSpan(map[string]string{"http.request.method": "GET"}),
	)
	err := assertHTTPTraces(GoServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), GoServiceName)
	require.Contains(t, err.Error(), "service.name")
}

func TestAssertHTTPTracesFailsWithoutClientSpan(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": GoServiceName},
		serverSpan(map[string]string{"http.request.method": "GET", "url.path": "/checkout"}),
	)
	err := assertHTTPTraces(GoServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "CLIENT")
}

func TestAssertHTTPTracesFailsWithoutServerCheckoutSpan(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": GoServiceName},
		spanSpec{name: "GET /health", kind: tracepb.Span_SPAN_KIND_SERVER, attrs: map[string]string{"http.request.method": "GET", "url.path": "/health"}},
		clientSpan(map[string]string{"http.request.method": "GET"}),
	)
	err := assertHTTPTraces(GoServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "/checkout")
}

func TestAssertLogsPresent(t *testing.T) {
	sink := otelsink.Start(t)

	err := assertLogsPresent(GoServiceName)(t, sink)
	require.Error(t, err, "no records at all must fail")

	feedLog(t, sink, map[string]string{"service.name": "some-other-service"}, "checkout completed")
	err = assertLogsPresent(GoServiceName)(t, sink)
	require.Error(t, err, "records under another service.name must not satisfy the assertion")
	require.Contains(t, err.Error(), GoServiceName)

	feedLog(t, sink, map[string]string{"service.name": GoServiceName}, "checkout completed")
	require.NoError(t, assertLogsPresent(GoServiceName)(t, sink))
}

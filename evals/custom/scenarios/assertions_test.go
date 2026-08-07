package scenarios

import (
	"encoding/hex"
	"fmt"
	"strings"
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

// The stdout correlation assertion of the go-logs scenario, end to end
// against a real sink: the record's (trace_id, span_id) pair must name an
// exported span of the right service, and OTLP application logs are refused.
func TestAssertStdoutLogCorrelation(t *testing.T) {
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	const spanID = "00f067aa0ba902b7"
	feed := func(t *testing.T) *otelsink.Sink {
		sink := otelsink.Start(t)
		rawTrace, err := hex.DecodeString(traceID)
		require.NoError(t, err)
		rawSpan, err := hex.DecodeString(spanID)
		require.NoError(t, err)
		server := serverSpan(map[string]string{"http.request.method": "GET", "url.path": "/checkout"})
		server.traceID, server.spanID = rawTrace, rawSpan
		feedSpans(t, sink, map[string]string{"service.name": GoServiceName}, server,
			clientSpan(map[string]string{"http.request.method": "GET"}))
		return sink
	}
	record := func(traceKey, traceVal, spanKey, spanVal string) string {
		return fmt.Sprintf(`{"level":"INFO","msg":"checkout completed","%s":%q,"%s":%q}`,
			traceKey, traceVal, spanKey, spanVal)
	}
	assert := assertStdoutLogCorrelation(GoServiceName, "checkout completed")

	t.Run("matching pair passes, snake_case keys", func(t *testing.T) {
		sink := feed(t)
		require.NoError(t, assert(t, sink, record("trace_id", traceID, "span_id", spanID)))
	})
	t.Run("matching pair passes, camelCase keys and uppercase hex", func(t *testing.T) {
		sink := feed(t)
		require.NoError(t, assert(t, sink, record("traceId", strings.ToUpper(traceID), "spanId", strings.ToUpper(spanID))))
	})
	t.Run("non-JSON noise around the record is skipped", func(t *testing.T) {
		sink := feed(t)
		output := "boot noise\n" + record("trace_id", traceID, "span_id", spanID) + "\ntrailing noise"
		require.NoError(t, assert(t, sink, output))
	})
	t.Run("no structured record fails", func(t *testing.T) {
		sink := feed(t)
		err := assert(t, sink, "checkout completed but not JSON")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no single-line JSON record")
	})
	t.Run("missing span id fails", func(t *testing.T) {
		sink := feed(t)
		err := assert(t, sink, fmt.Sprintf(`{"msg":"checkout completed","trace_id":%q}`, traceID))
		require.Error(t, err)
		require.Contains(t, err.Error(), "no span id field")
	})
	t.Run("invalid hex ids fail", func(t *testing.T) {
		sink := feed(t)
		err := assert(t, sink, record("trace_id", "not-hex", "span_id", spanID))
		require.Error(t, err)
		require.Contains(t, err.Error(), "not valid hex")
	})
	t.Run("right trace, wrong span fails", func(t *testing.T) {
		sink := feed(t)
		err := assert(t, sink, record("trace_id", traceID, "span_id", "ffffffffffffffff"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "matches no span")
	})
	t.Run("OTLP application logs are refused", func(t *testing.T) {
		sink := feed(t)
		feedLog(t, sink, map[string]string{"service.name": GoServiceName}, "checkout completed")
		err := assert(t, sink, record("trace_id", traceID, "span_id", spanID))
		require.Error(t, err)
		require.Contains(t, err.Error(), "arrived over OTLP")
	})
	t.Run("OTLP logs of another service do not interfere", func(t *testing.T) {
		sink := feed(t)
		feedLog(t, sink, map[string]string{"service.name": "some-other-service"}, "unrelated")
		require.NoError(t, assert(t, sink, record("trace_id", traceID, "span_id", spanID)))
	})
}

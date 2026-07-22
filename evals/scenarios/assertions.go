package scenarios

import (
	"fmt"
	"strings"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/harness"
)

// Attribute keys accepted for HTTP method and path: instrumentation libraries
// still straddle stable HTTP semantic conventions and their pre-stabilization
// names, and both prove the skill produced working HTTP telemetry.
var (
	httpMethodAttributes = []string{"http.request.method", "http.method"}
	httpPathAttributes   = []string{"url.path", "http.route", "http.target", "url.full", "http.url"}
)

// assertHTTPTraces builds the assertion shared by the HTTP instrumentation
// scenarios: under the given service.name (AE4), a server span for
// GET /checkout and a client span for the outbound call must be present.
func assertHTTPTraces(serviceName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		all := sink.Traces(t)
		svc := all.WithResourceAttribute("service.name", serviceName)
		if svc.Len() == 0 {
			return fmt.Errorf("no spans with resource attribute service.name=%q at the sink (%d spans total, names: %v)", serviceName, all.Len(), all.Names())
		}
		if !hasHTTPSpan(svc, tracepb.Span_SPAN_KIND_SERVER, "GET", "/checkout") {
			return fmt.Errorf("no SERVER span for GET /checkout with service.name=%q (span names: %v)", serviceName, svc.Names())
		}
		if !hasHTTPSpan(svc, tracepb.Span_SPAN_KIND_CLIENT, "GET", "") {
			return fmt.Errorf("no CLIENT span for the outbound GET call with service.name=%q (span names: %v)", serviceName, svc.Names())
		}
		return nil
	}
}

// assertLogsPresent builds the assertion of the logs scenarios: at least 1
// log record carrying the fixture's service.name must reach the sink.
func assertLogsPresent(serviceName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		logs := sink.Logs(t)
		for _, lv := range logs.Records() {
			if attrValue(lv.Resource.GetAttributes(), "service.name") == serviceName {
				return nil
			}
		}
		return fmt.Errorf("no log record with resource attribute service.name=%q at the sink (%d records total)", serviceName, logs.Len())
	}
}

// hasHTTPSpan reports whether the view contains a span of the given kind
// whose HTTP method attribute equals method and, when pathSubstr is
// non-empty, whose path-bearing attribute or span name contains pathSubstr.
func hasHTTPSpan(view *otelsink.Traces, kind tracepb.Span_SpanKind, method, pathSubstr string) bool {
	for _, sv := range view.WithKind(kind).Spans() {
		if attrValue(sv.Span.GetAttributes(), httpMethodAttributes...) != method {
			continue
		}
		if pathSubstr == "" {
			return true
		}
		if strings.Contains(sv.Span.GetName(), pathSubstr) {
			return true
		}
		if strings.Contains(attrValue(sv.Span.GetAttributes(), httpPathAttributes...), pathSubstr) {
			return true
		}
	}
	return false
}

// attrValue returns the rendered value of the first of keys present in attrs,
// or the empty string when none is.
func attrValue(attrs []*commonpb.KeyValue, keys ...string) string {
	for _, key := range keys {
		for _, kv := range attrs {
			if kv.GetKey() == key {
				return otelsink.AttrString(kv.GetValue())
			}
		}
	}
	return ""
}

// strAttrs converts a string map to OTLP KeyValues.
func strAttrs(attrs map[string]string) []*commonpb.KeyValue {
	out := make([]*commonpb.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		out = append(out, &commonpb.KeyValue{
			Key:   k,
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}},
		})
	}
	return out
}

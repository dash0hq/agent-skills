package scenarios

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

// Attribute keys accepted for HTTP method and path: instrumentation libraries
// still straddle stable HTTP semantic conventions and their pre-stabilization
// names, and both prove the skill produced working HTTP telemetry.
var (
	httpMethodAttributes = []string{"http.request.method", "http.method"}
	httpPathAttributes   = []string{"url.path", "http.route", "http.target", "url.full", "http.url"}
)

// Synthetic W3C IDs shared by the Node.js traffic driver and assertions.
// Keep these distinct from skill examples: the agent's validation traffic
// can reach the same sink before the harness runs the final fixture.
const (
	sampledTraceID     = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sampledParentID    = "bbbbbbbbbbbbbbbb"
	sampledTraceparent = "00-" + sampledTraceID + "-" + sampledParentID + "-01"
)

// The same W3C identifiers for the unsampled probe: identical shape, sampled
// flag cleared. A default `parentbased_always_on` sampler drops the resulting
// server span; the `always_on` sampler the skill prescribes records it.
const (
	unsampledTraceID     = "cccccccccccccccccccccccccccccccc"
	unsampledParentID    = "dddddddddddddddd"
	unsampledTraceparent = "00-" + unsampledTraceID + "-" + unsampledParentID + "-00"
)

// assertUnsampledParentRecorded requires a SERVER span for GET /checkout that
// continues the unsampled remote parent. This is the only assertion in the
// suite that distinguishes the samplers: it fails under the SDK default
// (`parentbased_always_on`, which honors the cleared sampled flag and drops
// the span) and passes only when the application overrides the sampler to
// `always_on` as skills/otel-instrumentation/rules/spans.md prescribes.
func assertUnsampledParentRecorded(serviceName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		spans := sink.Traces(t).WithResourceAttribute("service.name", serviceName).Spans()
		for _, sv := range spans {
			span := sv.Span
			if span.GetKind() != tracepb.Span_SPAN_KIND_SERVER ||
				fmt.Sprintf("%x", span.GetTraceId()) != unsampledTraceID ||
				attrValue(span.GetAttributes(), httpMethodAttributes...) != "GET" ||
				!spanReferencesPath(span, "/checkout") {
				continue
			}
			if parent := fmt.Sprintf("%x", span.GetParentSpanId()); parent != unsampledParentID {
				return fmt.Errorf("SERVER span in unsampled trace has parent %q, want %q", parent, unsampledParentID)
			}
			return nil
		}
		return fmt.Errorf("no SERVER span for GET /checkout continuing the unsampled remote parent %s: "+
			"the application honored the incoming sampled=0 flag instead of overriding the sampler to always_on",
			unsampledTraceID)
	}
}

// assertAll runs assertions in order and returns the first failure, so a
// scenario can carry more than 1 independent requirement.
func assertAll(assertions ...harness.Assertion) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		for _, assert := range assertions {
			if err := assert(t, sink); err != nil {
				return err
			}
		}
		return nil
	}
}

// assertInboundParentHTTPTraces preserves the root-request check while also
// requiring the exact sampled remote parent. Both cases are independent of
// whether the application uses AlwaysOn or ParentBased(root=AlwaysOn).
func assertInboundParentHTTPTraces(serviceName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		if err := assertHTTPTraces(serviceName)(t, sink); err != nil {
			return err
		}
		var root bool
		remoteServers := make(map[string]bool)
		parents := make(map[string]string)
		spans := sink.Traces(t).WithResourceAttribute("service.name", serviceName).Spans()
		for _, sv := range spans {
			span := sv.Span
			traceID := fmt.Sprintf("%x", span.GetTraceId())
			if traceID == sampledTraceID {
				parents[fmt.Sprintf("%x", span.GetSpanId())] = fmt.Sprintf("%x", span.GetParentSpanId())
			}
			if span.GetKind() != tracepb.Span_SPAN_KIND_SERVER ||
				attrValue(span.GetAttributes(), httpMethodAttributes...) != "GET" ||
				!spanReferencesPath(span, "/checkout") {
				continue
			}
			if traceID == sampledTraceID {
				if parent := fmt.Sprintf("%x", span.GetParentSpanId()); parent != sampledParentID {
					return fmt.Errorf("SERVER span in sampled trace has parent %q, want %q", parent, sampledParentID)
				}
				remoteServers[fmt.Sprintf("%x", span.GetSpanId())] = true
			} else if len(span.GetTraceId()) == 16 && len(span.GetParentSpanId()) == 0 {
				root = true
			}
		}
		if !root {
			return fmt.Errorf("no root SERVER span for GET /checkout without incoming context")
		}
		if len(remoteServers) == 0 {
			return fmt.Errorf("no SERVER span for GET /checkout continuing the sampled remote parent")
		}
		// Instrumentation may wrap the outbound call in INTERNAL spans. Follow
		// only same-service, same-trace parents; do not require a direct child.
		for _, sv := range spans {
			span := sv.Span
			if span.GetKind() != tracepb.Span_SPAN_KIND_CLIENT ||
				fmt.Sprintf("%x", span.GetTraceId()) != sampledTraceID ||
				attrValue(span.GetAttributes(), httpMethodAttributes...) != "GET" {
				continue
			}
			seen := make(map[string]bool)
			for parent := fmt.Sprintf("%x", span.GetParentSpanId()); parent != "" && !seen[parent]; parent = parents[parent] {
				if remoteServers[parent] {
					return nil
				}
				seen[parent] = true
			}
		}
		return fmt.Errorf("no outbound CLIENT span descending from the sampled SERVER span")
	}
}

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

// Keys accepted for the trace-context fields on stdout log records: logging
// setups differ in casing conventions, and every spelling proves the same
// correlation.
var (
	stdoutTraceIDKeys = []string{"trace_id", "traceId", "traceID", "trace.id"}
	stdoutSpanIDKeys  = []string{"span_id", "spanId", "spanID", "span.id"}
)

// W3C trace-context ids as lowercase hex: 16 bytes for the trace, 8 for the span.
var (
	hexTraceIDRe = regexp.MustCompile(`^[0-9a-f]{32}$`)
	hexSpanIDRe  = regexp.MustCompile(`^[0-9a-f]{16}$`)
)

// assertStdoutLogCorrelation builds the fixture-output assertion of the
// go-logs scenario: the record containing message on the fixture's stdout
// must be single-line JSON carrying a (trace_id, span_id) pair that names a
// span actually exported for serviceName — matching the pair, not just the
// trace, proves the record was emitted inside the span's context rather than
// stamped loosely. Application logs additionally must NOT arrive over OTLP:
// logs.md prescribes stdout as the only application-log delivery channel.
func assertStdoutLogCorrelation(serviceName, message string) harness.AppAssertion {
	return func(t *testing.T, sink *otelsink.Sink, appOutput string) error {
		for _, lv := range sink.Logs(t).Records() {
			if attrValue(lv.Resource.GetAttributes(), "service.name") == serviceName {
				return fmt.Errorf("application log records for service.name=%q arrived over OTLP; stdout must be the only application-log delivery channel", serviceName)
			}
		}

		records := stdoutJSONRecords(appOutput, message)
		if len(records) == 0 {
			return fmt.Errorf("no single-line JSON record containing %q on the fixture's stdout", message)
		}

		exported := map[string]bool{}
		for _, sv := range sink.Traces(t).WithResourceAttribute("service.name", serviceName).Spans() {
			exported[fmt.Sprintf("%x:%x", sv.Span.GetTraceId(), sv.Span.GetSpanId())] = true
		}

		var lastProblem error
		for _, rec := range records {
			traceID := strings.ToLower(stringField(rec, stdoutTraceIDKeys))
			spanID := strings.ToLower(stringField(rec, stdoutSpanIDKeys))
			switch {
			case traceID == "":
				lastProblem = fmt.Errorf("the %q record carries no trace id field (accepted keys: %s)", message, strings.Join(stdoutTraceIDKeys, ", "))
			case spanID == "":
				lastProblem = fmt.Errorf("the %q record carries no span id field (accepted keys: %s)", message, strings.Join(stdoutSpanIDKeys, ", "))
			case !hexTraceIDRe.MatchString(traceID) || !hexSpanIDRe.MatchString(spanID):
				lastProblem = fmt.Errorf("the %q record's ids are not valid hex trace/span ids: trace_id=%q span_id=%q", message, traceID, spanID)
			case !exported[traceID+":"+spanID]:
				lastProblem = fmt.Errorf("the %q record's (trace_id, span_id) pair (%s, %s) matches no span exported for service.name=%q", message, traceID, spanID, serviceName)
			default:
				return nil
			}
		}
		return lastProblem
	}
}

// stdoutJSONRecords parses each line of output as JSON and returns the
// records whose message field (msg or message) contains message. Non-JSON
// lines are skipped: container output interleaves runtime noise with the
// application's structured records.
func stdoutJSONRecords(output, message string) []map[string]any {
	var records []map[string]any
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if strings.Contains(stringField(rec, []string{"msg", "message"}), message) {
			records = append(records, rec)
		}
	}
	return records
}

// stringField returns the first of keys present in rec with a string value.
func stringField(rec map[string]any, keys []string) string {
	for _, key := range keys {
		if v, ok := rec[key].(string); ok {
			return v
		}
	}
	return ""
}

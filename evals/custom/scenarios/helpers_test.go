package scenarios

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

// --- OTLP feeders: exercise assertions through the sink's real endpoints ---

// spanSpec describes one span to feed the sink with.
type spanSpec struct {
	name  string
	kind  tracepb.Span_SpanKind
	attrs map[string]string
}

func resourceWith(testID string, attrs map[string]string) *resourcepb.Resource {
	kvs := strAttrs(attrs)
	kvs = append(kvs, strAttrs(map[string]string{otelsink.TestIDAttribute: testID})...)
	return &resourcepb.Resource{Attributes: kvs}
}

// feedSpans posts spans to the sink's OTLP/HTTP traces endpoint in-process.
func feedSpans(t *testing.T, sink *otelsink.Sink, resourceAttrs map[string]string, spans ...spanSpec) {
	t.Helper()
	var pbSpans []*tracepb.Span
	for _, s := range spans {
		pbSpans = append(pbSpans, &tracepb.Span{
			Name:       s.name,
			Kind:       s.kind,
			Attributes: strAttrs(s.attrs),
		})
	}
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource:   resourceWith(sink.TestID(), resourceAttrs),
			ScopeSpans: []*tracepb.ScopeSpans{{Spans: pbSpans}},
		}},
	}
	postOTLP(t, sink.HTTPEndpoint()+"/v1/traces", req)
}

// feedLog posts one log record to the sink's OTLP/HTTP logs endpoint.
func feedLog(t *testing.T, sink *otelsink.Sink, resourceAttrs map[string]string, body string) {
	t.Helper()
	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			Resource: resourceWith(sink.TestID(), resourceAttrs),
			ScopeLogs: []*logspb.ScopeLogs{{
				LogRecords: []*logspb.LogRecord{{
					SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
					Body:           &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: body}},
				}},
			}},
		}},
	}
	postOTLP(t, sink.HTTPEndpoint()+"/v1/logs", req)
}

func postOTLP(t *testing.T, url string, msg proto.Message) {
	t.Helper()
	body, err := protojson.Marshal(msg)
	require.NoError(t, err)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// scenarioIDs projects scenarios to their IDs.
func scenarioIDs(scs []harness.Scenario) []string {
	out := make([]string, 0, len(scs))
	for _, sc := range scs {
		out = append(out, sc.ID)
	}
	return out
}

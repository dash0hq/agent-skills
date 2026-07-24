package harness

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"
)

// strKV builds a string-valued OTLP attribute.
func strKV(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}},
	}
}

// makeTraceRequest builds an OTLP trace export request carrying the given
// test.id and resource attributes with a single named span.
func makeTraceRequest(testID, spanName string, resourceAttrs map[string]string) *coltracepb.ExportTraceServiceRequest {
	attrs := []*commonpb.KeyValue{strKV(otelsink.TestIDAttribute, testID)}
	for k, v := range resourceAttrs {
		attrs = append(attrs, strKV(k, v))
	}
	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{{
			Resource: &resourcepb.Resource{Attributes: attrs},
			ScopeSpans: []*tracepb.ScopeSpans{{
				Spans: []*tracepb.Span{{
					Name: spanName,
					Kind: tracepb.Span_SPAN_KIND_SERVER,
				}},
			}},
		}},
	}
}

// sendTestSpan exports a span through the composed fixture environment (relay
// endpoint plus bearer token), standing in for a real fixture workload.
func sendTestSpan(ctx context.Context, env map[string]string, spanName string, resourceAttrs map[string]string) error {
	testID := strings.TrimPrefix(env["OTEL_RESOURCE_ATTRIBUTES"], otelsink.TestIDAttribute+"=")
	body, err := protojson.Marshal(makeTraceRequest(testID, spanName, resourceAttrs))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, env[EnvOTLPEndpoint]+"/v1/traces", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env[EnvOTLPToken])
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send span: relay returned status %d", resp.StatusCode)
	}
	return nil
}

// sendSpanHooks returns fixture hooks whose Run step exports one span, the
// telemetry stand-in for the fixtures that arrive in U3.
func sendSpanHooks(spanName string, resourceAttrs map[string]string) FixtureHooks {
	return FixtureHooks{
		Run: func(ctx context.Context, _ string, env map[string]string) error {
			return sendTestSpan(ctx, env, spanName, resourceAttrs)
		},
	}
}

// scenarioIDs projects scenarios to their IDs.
func scenarioIDs(scs []Scenario) []string {
	out := make([]string, 0, len(scs))
	for _, sc := range scs {
		out = append(out, sc.ID)
	}
	return out
}

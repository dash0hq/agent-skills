package scenarios

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/custom/harness"

	"github.com/dash0hq/agent-skills/evals/custom/internal/testutil"
)

// collectorResource is the resource the assertion tests feed under: the
// Collector scenarios' service.name, as the feeders stamp it.
func collectorResource() map[string]string {
	return map[string]string{"service.name": CollectorServiceName}
}

// feedSumMetric posts one cumulative Sum metric to the sink's OTLP/HTTP
// metrics endpoint in-process.
func feedSumMetric(t *testing.T, sink *otelsink.Sink, resourceAttrs map[string]string, name string) {
	t.Helper()
	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{{
			Resource: resourceWith(sink.TestID(), resourceAttrs),
			ScopeMetrics: []*metricspb.ScopeMetrics{{
				Metrics: []*metricspb.Metric{{
					Name: name,
					Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{
						AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
						IsMonotonic:            true,
						DataPoints:             []*metricspb.NumberDataPoint{{Value: &metricspb.NumberDataPoint_AsInt{AsInt: 1}}},
					}},
				}},
			}},
		}},
	}
	postOTLP(t, sink.HTTPEndpoint()+"/v1/metrics", req)
}

// hardeningSpans returns hardeningSpanCount server spans shaped like the
// pipeline-hardening feeder's.
func hardeningSpans() []spanSpec {
	spans := make([]spanSpec, 0, hardeningSpanCount)
	for i := 0; i < hardeningSpanCount; i++ {
		spans = append(spans, serverSpan(map[string]string{"url.path": "/checkout"}))
	}
	return spans
}

// --- registration and selection ---

func TestU5Registration(t *testing.T) {
	byID := map[string]harness.Scenario{}
	for _, sc := range Default().Scenarios() {
		byID[sc.ID] = sc
	}

	root := testutil.RepoRoot(t)
	for _, id := range []string{CollectorPipelineHardeningID, OTTLRedactionID, OTTLEnrichmentID, SemconvAttributesID} {
		sc, ok := byID[id]
		require.True(t, ok, "scenario %s must be registered", id)
		require.NotNil(t, sc.Assert, "scenario %s must declare an assertion", id)
		require.NotEmpty(t, sc.RuleFiles, "scenario %s must declare rule files", id)
		require.NotEmpty(t, sc.FixturePath)
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(sc.FixturePath)))
		require.NoError(t, err, "fixture path of %s", id)
	}

	// One smoke scenario per newly covered skill.
	require.True(t, byID[CollectorPipelineHardeningID].Smoke, "the pipeline-hardening scenario is the otel-collector smoke scenario")
	require.True(t, byID[OTTLRedactionID].Smoke, "the redaction scenario is the otel-ottl smoke scenario")
	require.True(t, byID[SemconvAttributesID].Smoke, "the semconv scenario is the otel-semantic-conventions smoke scenario")

	// The Collector-backed scenarios must have host-process execution specs
	// with feeders; the semconv scenario runs the Docker topology instead.
	for _, id := range []string{CollectorPipelineHardeningID, OTTLRedactionID, OTTLEnrichmentID} {
		spec, ok := collectorScenarioSpecs[id]
		require.True(t, ok, "scenario %s must have a Collector execution spec", id)
		require.NotNil(t, spec.Feed, "scenario %s must feed synthetic telemetry", id)
	}
	_, ok := collectorScenarioSpecs[SemconvAttributesID]
	require.False(t, ok, "the semconv scenario reuses the Docker-backed Go fixture")
}

// Covers AE2 for the U5 rule files: each dedicated file selects exactly the
// scenario declaring it.
func TestU5Selection(t *testing.T) {
	reg := Default()
	tests := []struct {
		changed string
		want    []string
	}{
		{"skills/otel-collector/rules/pipelines.md", []string{CollectorPipelineHardeningID}},
		{"skills/otel-ottl/rules/redaction.md", []string{OTTLRedactionID}},
		{"skills/otel-ottl/rules/enrichment.md", []string{OTTLEnrichmentID}},
		{"skills/otel-semantic-conventions/rules/attributes.md", []string{SemconvAttributesID}},
		{"evals/custom/fixtures/collector-workspace/config.yaml", []string{CollectorPipelineHardeningID, OTTLRedactionID, OTTLEnrichmentID}},
	}
	for _, tt := range tests {
		t.Run(tt.changed, func(t *testing.T) {
			require.Equal(t, tt.want, scenarioIDs(reg.Select([]string{tt.changed})))
		})
	}
}

// --- pipeline hardening assertion ---

func TestAssertAllSignalsFlow(t *testing.T) {
	sink := otelsink.Start(t)
	assertion := assertAllSignalsFlow(CollectorServiceName, hardeningSpanCount)

	err := assertion(t, sink)
	require.Error(t, err, "an empty sink must fail")
	require.Contains(t, err.Error(), "traces pipeline")

	feedSpans(t, sink, collectorResource(), hardeningSpans()...)
	err = assertion(t, sink)
	require.Error(t, err, "spans without metrics must fail")
	require.Contains(t, err.Error(), "metrics pipeline")

	feedSumMetric(t, sink, collectorResource(), hardeningMetricName)
	err = assertion(t, sink)
	require.Error(t, err, "spans and metrics without logs must fail")
	require.Contains(t, err.Error(), "logs pipeline")

	feedLog(t, sink, collectorResource(), hardeningLogBody)
	require.NoError(t, assertion(t, sink))
}

func TestAssertAllSignalsFlowRejectsDroppedSpans(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, collectorResource(), hardeningSpans()[:hardeningSpanCount-1]...)
	feedSumMetric(t, sink, collectorResource(), hardeningMetricName)
	feedLog(t, sink, collectorResource(), hardeningLogBody)

	err := assertAllSignalsFlow(CollectorServiceName, hardeningSpanCount)(t, sink)
	require.Error(t, err, "a pipeline that drops spans must fail")
	require.Contains(t, err.Error(), "traces pipeline")
}

// --- redaction assertion ---

func redactedSpan(attrs map[string]string) spanSpec {
	base := map[string]string{"user.id": "TEST-0001", "url.path": "/checkout"}
	for k, v := range attrs {
		base[k] = v
	}
	return serverSpan(base)
}

func TestAssertEmailRedactedPasses(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, collectorResource(), redactedSpan(nil))
	require.NoError(t, assertEmailRedacted(CollectorServiceName)(t, sink))
}

func TestAssertEmailRedactedFailsOnLeak(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, collectorResource(), redactedSpan(map[string]string{"user.email": "user@example.test"}))
	err := assertEmailRedacted(CollectorServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "user.email")
}

func TestAssertEmailRedactedFailsWhenOtherAttributesDropped(t *testing.T) {
	sink := otelsink.Start(t)
	// The over-eager redaction case: user.email is gone, but user.id too.
	feedSpans(t, sink, collectorResource(), serverSpan(map[string]string{"url.path": "/checkout"}))
	err := assertEmailRedacted(CollectorServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "user.id")
}

func TestAssertEmailRedactedFailsWithoutSpans(t *testing.T) {
	sink := otelsink.Start(t)
	require.Error(t, assertEmailRedacted(CollectorServiceName)(t, sink))
}

// --- enrichment assertion ---

func TestAssertEnvironmentEnrichmentPasses(t *testing.T) {
	sink := otelsink.Start(t)
	resource := collectorResource()
	resource["deployment.environment.name"] = enrichmentEnvironmentName
	feedSpans(t, sink, resource, serverSpan(map[string]string{
		"user.id":                     "TEST-0001",
		"deployment.environment.name": enrichmentEnvironmentName,
	}))
	require.NoError(t, assertEnvironmentEnrichment(CollectorServiceName, enrichmentEnvironmentName)(t, sink))
}

func TestAssertEnvironmentEnrichmentFailsWithoutResourceAttribute(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, collectorResource(), serverSpan(map[string]string{
		"user.id":                     "TEST-0001",
		"deployment.environment.name": enrichmentEnvironmentName,
	}))
	err := assertEnvironmentEnrichment(CollectorServiceName, enrichmentEnvironmentName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "resource attribute")
}

func TestAssertEnvironmentEnrichmentFailsWithoutCopyDown(t *testing.T) {
	sink := otelsink.Start(t)
	resource := collectorResource()
	resource["deployment.environment.name"] = enrichmentEnvironmentName
	feedSpans(t, sink, resource, serverSpan(map[string]string{"user.id": "TEST-0001"}))
	err := assertEnvironmentEnrichment(CollectorServiceName, enrichmentEnvironmentName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "copied down")
}

func TestAssertEnvironmentEnrichmentFailsWhenPreexistingAttributesDropped(t *testing.T) {
	sink := otelsink.Start(t)
	resource := collectorResource()
	resource["deployment.environment.name"] = enrichmentEnvironmentName
	feedSpans(t, sink, resource, serverSpan(map[string]string{
		"deployment.environment.name": enrichmentEnvironmentName,
	}))
	err := assertEnvironmentEnrichment(CollectorServiceName, enrichmentEnvironmentName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "user.id")
}

// --- semconv assertion ---

// semconvSpan is a SERVER span carrying the pinned synthetic values under the
// given attribute names.
func semconvSpan(customerKey, orderKey string) spanSpec {
	return serverSpan(map[string]string{
		customerKey: semconvCustomerID,
		orderKey:    semconvOrderValue,
	})
}

func TestAssertCheckoutDomainAttributesPasses(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": SemconvServiceName},
		semconvSpan("app.customer.id", "app.order.value"))
	require.NoError(t, assertCheckoutDomainAttributes(SemconvServiceName)(t, sink))
}

func TestAssertCheckoutDomainAttributesPassesOnReverseDNSNamespace(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": SemconvServiceName},
		semconvSpan("com.acme.checkout.customer_id", "com.acme.checkout.order_value"))
	require.NoError(t, assertCheckoutDomainAttributes(SemconvServiceName)(t, sink))
}

func TestAssertCheckoutDomainAttributesRejectsCamelCase(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": SemconvServiceName},
		semconvSpan("customerId", "orderValue"))
	err := assertCheckoutDomainAttributes(SemconvServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "customerId")
	require.Contains(t, err.Error(), "not a lowercase dot-namespaced name")
}

func TestAssertCheckoutDomainAttributesRejectsBareNames(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": SemconvServiceName},
		semconvSpan("customer_id", "order_value"))
	err := assertCheckoutDomainAttributes(SemconvServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "customer_id")
}

func TestAssertCheckoutDomainAttributesFailsWithoutValues(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": SemconvServiceName},
		serverSpan(map[string]string{"app.customer.id": semconvCustomerID}))
	err := assertCheckoutDomainAttributes(SemconvServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "order value")
}

func TestAssertCheckoutDomainAttributesRequiresServerSpan(t *testing.T) {
	sink := otelsink.Start(t)
	feedSpans(t, sink, map[string]string{"service.name": SemconvServiceName},
		spanSpec{name: "GET", kind: tracepb.Span_SPAN_KIND_CLIENT, attrs: map[string]string{
			"app.customer.id": semconvCustomerID,
			"app.order.value": semconvOrderValue,
		}})
	err := assertCheckoutDomainAttributes(SemconvServiceName)(t, sink)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SERVER")
}

func TestValidateAttributeName(t *testing.T) {
	valid := []string{
		"user.id",
		"app.customer.id",
		"com.acme.checkout.order_value",
		"dash0.queue.depth",
	}
	for _, name := range valid {
		require.NoError(t, validateAttributeName(name), "name %q must be accepted", name)
	}

	invalid := []string{
		"customerId",         // camelCase
		"Order.Value",        // uppercase segments
		"order_value",        // bare snake_case without a namespace
		"ordervalue",         // bare name
		"otel.order.value",   // reserved namespace
		"app..order",         // empty segment
		"app.order.value ",   // trailing whitespace
		".app.order",         // leading dot
		"app.order-value.id", // hyphens
	}
	for _, name := range invalid {
		require.Error(t, validateAttributeName(name), "name %q must be rejected", name)
	}
}

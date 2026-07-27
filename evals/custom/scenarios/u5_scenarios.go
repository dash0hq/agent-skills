// U5 scenarios: end-to-end evals for the otel-collector, otel-ottl, and
// otel-semantic-conventions skills. The Collector and OTTL scenarios run the
// agent against the collector-workspace fixture and execute the resulting
// configuration with the pinned otelcol-contrib binary as a host process (no
// Docker; see collector_hooks.go). The semconv scenario reuses the Go service
// fixture and the standard Docker hooks.
package scenarios

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/open-telemetry/opentelemetry-packaging/testutil/otelsink"

	"github.com/dash0hq/agent-skills/evals/custom/harness"
)

// U5 scenario IDs.
const (
	// CollectorPipelineHardeningID hardens the seed Collector pipeline:
	// memory_limiter placed first everywhere, all 3 signal pipelines wired.
	CollectorPipelineHardeningID = "collector-pipeline-hardening"
	// OTTLRedactionID removes user.email from spans via the transform
	// processor while other attributes survive.
	OTTLRedactionID = "ottl-redaction"
	// OTTLEnrichmentID adds a static resource attribute and copies it down
	// to span level.
	OTTLEnrichmentID = "ottl-enrichment"
	// SemconvAttributesID adds checkout domain attributes to the Go
	// fixture's server span with semconv-compliant names.
	SemconvAttributesID = "semconv-attributes"
)

// Service names the U5 feeders and prompts use and the assertions check.
const (
	// CollectorServiceName is the service.name stamped on all synthetic
	// telemetry fed through the Collector under eval.
	CollectorServiceName = "collector-service-eval"
	// SemconvServiceName is the service.name the semconv scenario requires.
	SemconvServiceName = "semconv-service-eval"
)

// Synthetic values the semconv prompt pins so the assertion can locate the
// agent-chosen attributes by value.
const (
	semconvCustomerID = "TEST-0001"
	semconvOrderValue = "99.95"
)

// enrichmentEnvironmentName is the deployment.environment.name value the
// enrichment scenario requires.
const enrichmentEnvironmentName = "eval-environment"

// The U5 scenario set self-registers in declaration order (see register.go),
// so this unit lands without editing scenarios.go.
func init() {
	registerScenarios(
		CollectorPipelineHardening(),
		OTTLRedaction(),
		OTTLEnrichment(),
		SemconvAttributes(),
	)
}

// collectorPromptRequirements is shared prompt text for the scenarios that
// edit the collector-workspace fixture, enforcing the placeholder contract
// and the host-process execution constraints (see the fixture's README.md).
const collectorPromptRequirements = `
Requirements:
- Keep reading the runtime configuration from the environment: the OTLP receiver port comes from EVAL_OTLP_RECEIVER_PORT, the export endpoint from EVAL_OTLP_ENDPOINT, and the export bearer token from EVAL_OTLP_TOKEN, referenced as ${env:...} placeholders as in the seed configuration. Do not hardcode ports, endpoints, or tokens.
- The configuration runs verbatim as an unprivileged local process with the workspace as its working directory: any directory it references must be created inside the workspace and referenced by a relative path, there are no writable system paths, and there is no metrics scrape target, so keep the internal telemetry metrics level at "none".
- The export endpoint does not accept compressed payloads: keep "compression: none" on the exporter, as in the seed configuration.
- The result must pass "otelcol-contrib validate" and start successfully.`

// CollectorPipelineHardening asks the agent to harden the seed pipeline per
// the otel-collector skill. The Build hook lints what telemetry cannot prove
// (memory_limiter first, no batch processor); the sink assertion proves all 3
// signal pipelines are wired and preserve resource attributes.
func CollectorPipelineHardening() harness.Scenario {
	return harness.Scenario{
		ID:        CollectorPipelineHardeningID,
		Skill:     harness.SkillCollector,
		RuleFiles: []string{"skills/otel-collector/rules/pipelines.md"},
		// The smoke scenario for otel-collector: harness-code changes
		// select it.
		Smoke:       true,
		FixturePath: "evals/custom/fixtures/collector-workspace",
		Prompt: `Harden the OpenTelemetry Collector configuration in config.yaml in the current directory, following the otel-collector skill (dash0-agent-skills:otel-collector).

Goals:
- Protect the Collector against memory exhaustion the way the skill teaches, with the relevant processor placed correctly in every pipeline.
- Wire metrics and logs pipelines alongside the existing traces pipeline, so telemetry of all 3 signals flows from the existing OTLP receiver to the existing OTLP/HTTP exporter.
- Preserve resource attributes end to end; the harness feeds telemetry through the receiver and verifies what leaves the exporter.
` + collectorPromptRequirements,
		Timeout:          scenarioTimeout,
		TelemetryTimeout: telemetryTimeout,
		Assert:           assertAllSignalsFlow(CollectorServiceName, hardeningSpanCount),
	}
}

// OTTLRedaction asks the agent to remove user.email from spans via the
// transform processor, following the otel-ottl redaction rule ("Delete" row
// of its strategy table: the attribute must never leave the Collector).
func OTTLRedaction() harness.Scenario {
	return harness.Scenario{
		ID:        OTTLRedactionID,
		Skill:     harness.SkillOTTL,
		RuleFiles: []string{"skills/otel-ottl/rules/redaction.md"},
		// The smoke scenario for otel-ottl.
		Smoke:       true,
		FixturePath: "evals/custom/fixtures/collector-workspace",
		Prompt: `Edit the OpenTelemetry Collector configuration in config.yaml in the current directory, following the otel-ottl skill (dash0-agent-skills:otel-ottl).

Goals:
- The "user.email" span attribute must never leave this Collector: remove it from every span passing through the traces pipeline, using the transform processor.
- Every other span attribute (for example "user.id" and "url.path") and all resource attributes must be preserved unchanged.
` + collectorPromptRequirements,
		Timeout:          scenarioTimeout,
		TelemetryTimeout: telemetryTimeout,
		Assert:           assertEmailRedacted(CollectorServiceName),
	}
}

// OTTLEnrichment asks the agent to add a static resource attribute and copy
// it down to span level, the 2 patterns the otel-ottl enrichment rule
// teaches (resource processor for static resource attributes, transform
// processor for the copy-down).
func OTTLEnrichment() harness.Scenario {
	return harness.Scenario{
		ID:          OTTLEnrichmentID,
		Skill:       harness.SkillOTTL,
		RuleFiles:   []string{"skills/otel-ottl/rules/enrichment.md"},
		FixturePath: "evals/custom/fixtures/collector-workspace",
		Prompt: `Edit the OpenTelemetry Collector configuration in config.yaml in the current directory, following the otel-ottl skill (dash0-agent-skills:otel-ottl).

Goals:
- Enrich all telemetry passing through this Collector with the static resource attribute "deployment.environment.name" set to "` + enrichmentEnvironmentName + `".
- Copy "deployment.environment.name" down to every span as a span attribute with the same value.
- All pre-existing span and resource attributes must be preserved unchanged.
` + collectorPromptRequirements,
		Timeout:          scenarioTimeout,
		TelemetryTimeout: telemetryTimeout,
		Assert:           assertEnvironmentEnrichment(CollectorServiceName, enrichmentEnvironmentName),
	}
}

// SemconvAttributes asks the agent to add checkout domain attributes to the
// Go fixture's server span; the assertion accepts any attribute names that
// follow the conventions the otel-semantic-conventions skill teaches
// (lowercase dot-namespaced, outside the reserved otel.* namespace) and
// locates them by their pinned synthetic values.
func SemconvAttributes() harness.Scenario {
	return harness.Scenario{
		ID:        SemconvAttributesID,
		Skill:     harness.SkillSemConv,
		RuleFiles: []string{"skills/otel-semantic-conventions/rules/attributes.md"},
		// The smoke scenario for otel-semantic-conventions.
		Smoke:       true,
		FixturePath: "evals/custom/fixtures/go-service",
		Prompt: `Instrument the Go HTTP service in the current directory with OpenTelemetry so it exports traces via OTLP over http/protobuf, and set the service name to "` + SemconvServiceName + `". Produce a server span for the inbound GET /checkout request.

Then add domain attributes for the checkout flow to that SERVER span, following the otel-semantic-conventions skill (dash0-agent-skills:otel-semantic-conventions):
- the identifier of the customer checking out, with the string value "` + semconvCustomerID + `";
- the total value of the order, with the value ` + semconvOrderValue + `.

Pick attribute names and placement per the semantic conventions the skill teaches: search the attribute registry first, and correctly namespace any custom attribute you must create.
` + promptCommonRequirements,
		Timeout:          scenarioTimeout,
		TelemetryTimeout: telemetryTimeout,
		Assert:           assertCheckoutDomainAttributes(SemconvServiceName),
	}
}

// --- assertions ---

// assertAllSignalsFlow verifies the hardened pipeline forwards every signal:
// all fed spans, the Sum metric, and the log record must reach the sink under
// the fed service.name. The sink views are scoped to the per-run test.id, so
// any visible telemetry also proves the pipeline preserved that resource
// attribute.
func assertAllSignalsFlow(serviceName string, minSpans int) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		spans := sink.Traces(t).WithResourceAttribute("service.name", serviceName)
		if spans.Len() < minSpans {
			return fmt.Errorf("traces pipeline: %d of %d fed spans with service.name=%q reached the sink", spans.Len(), minSpans, serviceName)
		}
		if !metricPresent(sink.Metrics(t), hardeningMetricName, serviceName) {
			return fmt.Errorf("metrics pipeline: metric %q with service.name=%q did not reach the sink (metrics seen: %v)", hardeningMetricName, serviceName, sink.Metrics(t).Names())
		}
		if !logPresent(sink.Logs(t), hardeningLogBody, serviceName) {
			return fmt.Errorf("logs pipeline: log record %q with service.name=%q did not reach the sink (%d records seen)", hardeningLogBody, serviceName, sink.Logs(t).Len())
		}
		return nil
	}
}

// assertEmailRedacted verifies spans flowed through the Collector with
// user.email removed and the other attributes intact.
func assertEmailRedacted(serviceName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		spans := sink.Traces(t).WithResourceAttribute("service.name", serviceName)
		if spans.Len() == 0 {
			return fmt.Errorf("no spans with service.name=%q reached the sink", serviceName)
		}
		if leaked := spans.WithSpanAttribute("user.email").Len(); leaked > 0 {
			return fmt.Errorf("user.email is still present on %d of %d spans at the sink; it must never leave the Collector", leaked, spans.Len())
		}
		if spans.WithSpanAttributeValue("user.id", "TEST-0001").Len() == 0 {
			return fmt.Errorf("the user.id attribute did not survive the redaction; only user.email may be removed")
		}
		if spans.WithSpanAttributeValue("url.path", "/checkout").Len() == 0 {
			return fmt.Errorf("the url.path attribute did not survive the redaction; only user.email may be removed")
		}
		return nil
	}
}

// assertEnvironmentEnrichment verifies the derived attribute is present at
// both levels with the expected value and pre-existing attributes survived.
func assertEnvironmentEnrichment(serviceName, environment string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		spans := sink.Traces(t).WithResourceAttribute("service.name", serviceName)
		if spans.Len() == 0 {
			return fmt.Errorf("no spans with service.name=%q reached the sink", serviceName)
		}
		enriched := spans.WithResourceAttribute("deployment.environment.name", environment)
		if enriched.Len() == 0 {
			return fmt.Errorf("no span carries the resource attribute deployment.environment.name=%q", environment)
		}
		if enriched.WithSpanAttributeValue("deployment.environment.name", environment).Len() == 0 {
			return fmt.Errorf("deployment.environment.name=%q was not copied down to the span attributes", environment)
		}
		if spans.WithSpanAttributeValue("user.id", "TEST-0001").Len() == 0 {
			return fmt.Errorf("the pre-existing user.id attribute did not survive the enrichment")
		}
		return nil
	}
}

// assertCheckoutDomainAttributes verifies the agent-added domain attributes:
// they must sit on the SERVER span under the expected service.name, carry the
// pinned synthetic values, and use names that follow the conventions the
// otel-semantic-conventions skill teaches (see validateAttributeName).
func assertCheckoutDomainAttributes(serviceName string) harness.Assertion {
	return func(t *testing.T, sink *otelsink.Sink) error {
		spans := sink.Traces(t).WithResourceAttribute("service.name", serviceName)
		if spans.Len() == 0 {
			return fmt.Errorf("no spans with resource attribute service.name=%q at the sink (%d spans total)", serviceName, sink.Traces(t).Len())
		}
		servers := spans.WithKind(tracepb.Span_SPAN_KIND_SERVER)
		if servers.Len() == 0 {
			return fmt.Errorf("no SERVER span with service.name=%q at the sink (span names: %v)", serviceName, spans.Names())
		}

		customerKey, err := domainAttributeKey(servers, semconvCustomerID)
		if err != nil {
			return fmt.Errorf("customer id: %w", err)
		}
		if err := validateAttributeName(customerKey); err != nil {
			return fmt.Errorf("customer id attribute: %w", err)
		}

		orderKey, err := domainAttributeKey(servers, semconvOrderValue)
		if err != nil {
			return fmt.Errorf("order value: %w", err)
		}
		if err := validateAttributeName(orderKey); err != nil {
			return fmt.Errorf("order value attribute: %w", err)
		}
		return nil
	}
}

// domainAttributeKey finds the span attribute carrying the given rendered
// value on any span of the view and returns its key.
func domainAttributeKey(view *otelsink.Traces, value string) (string, error) {
	for _, sv := range view.Spans() {
		for _, kv := range sv.Span.GetAttributes() {
			if otelsink.AttrString(kv.GetValue()) == value {
				return kv.GetKey(), nil
			}
		}
	}
	return "", fmt.Errorf("no SERVER span attribute carries the value %q", value)
}

// attributeNamePattern is the naming shape the otel-semantic-conventions
// skill teaches: lowercase dot-separated segments (snake_case within a
// segment), with at least 1 namespace segment before the attribute name.
// It rejects camelCase (uppercase letters), bare snake_case without a
// namespace (order_value), and any other unnamespaced name.
var attributeNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)*(?:\.[a-z0-9]+(?:_[a-z0-9]+)*)+$`)

// validateAttributeName checks an attribute name against the conventions the
// otel-semantic-conventions skill teaches, returning an error naming the
// violation.
func validateAttributeName(key string) error {
	if !attributeNamePattern.MatchString(key) {
		return fmt.Errorf("attribute name %q is not a lowercase dot-namespaced name (expected namespace.attribute_name, for example user.id or com.acme.order.value)", key)
	}
	if strings.HasPrefix(key, "otel.") {
		return fmt.Errorf("attribute name %q uses the reserved otel.* namespace", key)
	}
	return nil
}

// metricPresent reports whether a metric with the given name and resource
// service.name is in the view.
func metricPresent(view *otelsink.Metrics, name, serviceName string) bool {
	for _, mv := range view.WithName(name).Metrics() {
		if attrValue(mv.Resource.GetAttributes(), "service.name") == serviceName {
			return true
		}
	}
	return false
}

// logPresent reports whether a log record whose body contains bodySubstr and
// whose resource service.name matches is in the view.
func logPresent(view *otelsink.Logs, bodySubstr, serviceName string) bool {
	for _, lv := range view.WithBodyContaining(bodySubstr).Records() {
		if attrValue(lv.Resource.GetAttributes(), "service.name") == serviceName {
			return true
		}
	}
	return false
}

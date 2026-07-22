---
title: 'Resource attributes'
impact: CRITICAL
tags:
  - resources
  - service-name
  - service-version
  - environment
---

# Resource attributes

Resource attributes identify *what* system is described by the telemetry, which very often is also the system producing the telemetry (but not always, like in case of network monitoring).
They are stable for the lifetime of the process and are attached to every signal (traces, metrics, and logs) automatically.
Getting them right is the single highest-impact thing you can do for observability — without them, telemetry cannot be attributed to a service, environment, or instance.

For guidance on *where* to place attributes across telemetry levels, see [attributes](../../otel-semantic-conventions/rules/attributes.md).

## Service identity

Three resource attributes together form the *service identity* of every telemetry record, written `[service.namespace] <service.name> [service.instance.id]` (angle brackets denote always-required attributes, square brackets denote attributes required whenever the deployment target contains their concept — multiple products sharing a backend, or multiple instances of the same service).
The recommended `service.version` provides important context for troubleshooting.

| Attribute | Value lifecycle | Where to set |
|-----------|-----------------|--------------|
| [`service.name`](#servicename) | Same across environments and instances | Application code, shared `.env` file, or deployment descriptor |
| [`service.namespace`](#servicenamespace) | Same across environments and instances | Application code or shared `.env` file |
| [`service.version`](#serviceversion) | Changes per release | Build pipeline (git tag, CI build number) |
| [`service.instance.id`](#serviceinstanceid) | Changes per instance | Generated at process startup or injected from the deployment platform |

Treat the four attributes as a single unit.
The following rules apply in every skill (instrumentation, Collector, OTTL) and to every configuration surface (SDK, deployment manifests, Collector processors, connectors, and exporters):

1. **Set every applicable identity attribute together.**
   When configuring instrumentation, resolve and set all four attributes that apply to the deployment target — do not set `service.name` alone.
   See [resolving configuration values](./resolve-values.md#service-identity) for the ordered lookup strategy per attribute.
2. **Preserve identity attributes end-to-end.**
   Do not drop, replace, mask, hash, or redact any identity attribute in Collector processors, OTTL statements, filter rules, sampling policies, or exporter transformations.
   Even for sensitive-data pipelines, identity attributes stay intact — see [OTTL redaction rules](../../otel-ottl/rules/redaction.md).
3. **Do not overwrite SDK-set identity in the Collector.**
   In the `resource` processor, never use `action: upsert` or `action: update` on an identity attribute.
   Use `action: insert` so SDK-set values win.
   See [resource processor guidance](../../otel-collector/rules/processors.md#resource-processor).
4. **Propagate identity to derived telemetry.**
   When deriving metrics from spans (e.g., `signaltometricsconnector`) or fanning telemetry to secondary pipelines, include every identity attribute in `include_resource_attributes` so downstream users can filter by namespace, version, and instance.
   See [RED metrics from traces](../../otel-collector/rules/red-metrics.md#resource-attributes).
5. **Verify all four whenever one changes.**
   Any edit that touches a service identity attribute (SDK config, `OTEL_RESOURCE_ATTRIBUTES`, Collector processor, OTTL statement) must be checked against the remaining three — they must all still be populated on the resulting telemetry.

### `service.name`

Every service must set `service.name`.
Without it, all telemetry falls into `unknown_service`, making it impossible to attribute to a service in dashboards, alerts, or service maps.

Set it via the `OTEL_SERVICE_NAME` environment variable:

```bash
export OTEL_SERVICE_NAME="order-api"
```

> [!NOTE]
> The value of `service.name` is the same in every environment and instance — set it in application code, a shared `.env` file, or a deployment descriptor.

Choose a name that is:
- **Stable** — does not change across deployments or restarts.
- **Unique per logical service** — two different services must not share the same name.
- **Human-readable** — descriptive enough to identify the service at a glance (e.g., `checkout-service`, not `svc-42`).
- **Case-consistent** — use the exact same case-sensitive string across all deployments and environments.
  Variations like `checkout` vs `CheckOut` in different environments complicate querying, especially during outages.

Pick a naming convention (kebab-case, snake_case, or camelCase) and apply it consistently across the entire service fleet.

### `service.namespace`

Groups related services within the same application or product.
Use it to scope services that belong together — for example, a `checkout`, `payment`, and `inventory` service might all share the namespace `acme-webstore`.

```bash
export OTEL_RESOURCE_ATTRIBUTES="service.namespace=acme-webstore"
```

Without a namespace, identically named services across different products become ambiguous.

> [!NOTE]
> The value of `service.namespace` is the same in every environment and instance — set it in application code or a shared `.env` file, like `service.name`.

### `service.version`

Set the service version to enable deployment tracking, regression detection, and version-aware analysis.
This attribute is invaluable during rollouts to compare how the new version and the old one behave.

```bash
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.4.2"
```

See [resolving configuration values](./resolve-values.md#serviceversion) for ordered lookup strategies to find the version value.

### `service.instance.id`

Uniquely identifies a single instance of the service.
The triplet (`service.namespace`, `service.name`, `service.instance.id`) must be globally unique.
Without it, instance-level analysis (e.g., identifying a single unhealthy pod) is not possible.

The value must be stable for the lifetime of the process and should be an opaque identifier — do not expose infrastructure details like pod names or container IDs directly.
See [resolving configuration values](./resolve-values.md#serviceinstanceid) for generation strategies (UUID v4, UUID v5, and common pitfalls).

Setting `service.instance.id` does not replace the need to also set `k8s.pod.uid` in Kubernetes.
Both attributes serve different purposes: `service.instance.id` is a logical, opaque identifier, while `k8s.pod.uid` is used by the `k8sattributes` processor for Kubernetes metadata enrichment.

## Other recommended resource attributes

### `deployment.environment.name`

Distinguish production from staging, development, and other environments.
Without it, production and test telemetry are mixed together, making dashboards and alerts unreliable.

```bash
export OTEL_RESOURCE_ATTRIBUTES="deployment.environment.name=production"
```

> [!NOTE]
> The value of `deployment.environment.name` changes per environment — inject it from the deployment pipeline (e.g., Helm values, CI/CD variable), not from application code.

See [resolving configuration values](./resolve-values.md#deploymentenvironmentname) for ordered lookup strategies.

### `service.criticality`

Ranks how business-critical the service is, so alert routing, on-call escalation, and dashboards can prioritize incidents by impact.

Set exactly one of the following enumerated values:

| Value | Meaning |
|-------|---------|
| `critical` | Outage causes immediate customer-visible impact, revenue loss, or safety concerns (e.g., checkout, payment, auth). |
| `high` | Outage causes noticeable customer-facing degradation but not full unavailability (e.g., recommendations, search). |
| `medium` | Internal function or supporting service; outage tolerable for hours (e.g., admin tooling, batch reporting). |
| `low` | Experimental, sandbox, or non-production functionality; no SLA. |

```bash
export OTEL_RESOURCE_ATTRIBUTES="service.criticality=critical"
```

The value is set by the owning team and does not change across environments — set it in application code or a shared `.env` file.

**Agents must ask the user for `service.criticality`.**
Do not infer it from the codebase, service name, or heuristics — misclassification silently distorts alerting and on-call priority.
See [resolving `service.criticality`](./resolve-values.md#servicecriticality) for the ordered lookup strategy.

### Kubernetes attributes

`k8s.*` attributes describe the infrastructure running the service, not the service itself.

> [!NOTE]
> Set `k8s.*` attributes via the Kubernetes downward API in pod specs or let the `k8sattributes` Collector processor resolve them automatically — never set them in application code.

Follow the guidance issued in [Kubernetes deployment](platforms/k8s.md).
For configuring the `k8sattributes` processor in the Collector, see the [processors](../../otel-collector/rules/processors.md#kubernetes-attributes) rule in the `otel-collector` skill.

## Setting resource attributes

Resource attributes are set via the `OTEL_RESOURCE_ATTRIBUTES` environment variable as a comma-separated list of `key=value` pairs.

```bash
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.4.2,deployment.environment.name=production,service.instance.id=$(uuidgen)"
```

Combine this with `OTEL_SERVICE_NAME` (which takes precedence over `service.name` in `OTEL_RESOURCE_ATTRIBUTES`):

```bash
export OTEL_SERVICE_NAME="order-api"
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.4.2,deployment.environment.name=production"
```

### Complete `.env.local` example

This example is suitable for local development.
In a production deployment descriptor (e.g., Kubernetes manifest, Docker Compose, or CI/CD pipeline), override `service.version` and `deployment.environment.name` with values derived from the build and deployment pipeline.

```bash
OTEL_SERVICE_NAME=order-api
OTEL_RESOURCE_ATTRIBUTES=service.namespace=acme-webstore,service.version=local-dev,deployment.environment.name=development

# Exporter configuration
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=https://<OTLP_ENDPOINT>
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer YOUR_AUTH_TOKEN
NODE_OPTIONS=--import @opentelemetry/auto-instrumentations-node/register
```

## References

- [`service.instance.id` semantic conventions](https://opentelemetry.io/docs/specs/semconv/registry/attributes/service/#service-instance-id)
- [Resource semantic conventions](https://opentelemetry.io/docs/specs/semconv/resource/)
- [OTel SDK configuration](https://opentelemetry.io/docs/languages/sdk-configuration/general/)
- [Service attributes best practices](https://www.dash0.com/guides/opentelemetry-service-attributes-best-practices)
- [Attribute placement guidance](../../otel-semantic-conventions/rules/attributes.md)
- [Kubernetes deployment](platforms/k8s.md)

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

## Required attributes

### `service.name`

Every service must set `service.name`.
Without it, all telemetry falls into `unknown_service`, making it impossible to attribute to a service in dashboards, alerts, or service maps.

Set it via the `OTEL_SERVICE_NAME` environment variable:

```bash
export OTEL_SERVICE_NAME="order-api"
```

Choose a name that is:
- **Stable** — does not change across deployments or restarts.
- **Unique per logical service** — two different services must not share the same name.
- **Human-readable** — descriptive enough to identify the service at a glance (e.g., `checkout-service`, not `svc-42`).
- **Case-consistent** — use the exact same case-sensitive string across all deployments and environments.
  Variations like `checkout` vs `CheckOut` in different environments complicate querying, especially during outages.

Pick a naming convention (kebab-case, snake_case, or camelCase) and apply it consistently across the entire service fleet.

## Recommended attributes

### `deployment.environment.name`

Distinguish production from staging, development, and other environments.
Without it, production and test telemetry are mixed together, making dashboards and alerts unreliable.

```bash
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.4.2,deployment.environment.name=production"
```

Do not hardcode a deployment environment string — derive it from the deployment pipeline or existing framework variables.
See [resolving configuration values](./resolve-values.md#deploymentenvironmentname) for ordered lookup strategies.

### `service.version`

Set the service version to enable deployment tracking, regression detection, and version-aware analysis.
This attribute is invaluable during rollouts to compare how the new version and the old one behave.

```bash
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.4.2"
```

Never hardcode a version string that must be updated manually — derive it from the build pipeline.
See [resolving configuration values](./resolve-values.md#serviceversion) for ordered lookup strategies to find the version value.

### `service.namespace`

Groups related services within the same application or product.
Use it to scope services that belong together — for example, a `checkout`, `payment`, and `inventory` service might all share the namespace `acme-webstore`.

```bash
export OTEL_RESOURCE_ATTRIBUTES="service.namespace=acme-webstore,service.version=1.4.2"
```

Without a namespace, identically named services across different products become ambiguous.

### `service.instance.id`

Uniquely identifies a single instance of the service.
The triplet (`service.namespace`, `service.name`, `service.instance.id`) must be globally unique.
Without it, instance-level analysis (e.g., identifying a single unhealthy pod) is not possible.

The value must be stable for the lifetime of the process and should be an opaque identifier — do not expose infrastructure details like pod names or container IDs directly.
See [resolving configuration values](./resolve-values.md#serviceinstanceid) for generation strategies (UUID v4, UUID v5, and common pitfalls).

Setting `service.instance.id` does not replace the need to also set `k8s.pod.uid` in Kubernetes.
Both attributes serve different purposes: `service.instance.id` is a logical, opaque identifier, while `k8s.pod.uid` is used by the `k8sattributes` processor for Kubernetes metadata enrichment.

## Kubernetes attributes

For applications running in Kubernetes, resource attributes must include enough metadata for telemetry to be correlated with the correct pod, workload, node, and cluster.
See [Kubernetes deployment](platforms/k8s.md) for the full list of Kubernetes attributes, downward API configuration, and a complete pod spec example.

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

```bash
OTEL_SERVICE_NAME=order-api
OTEL_RESOURCE_ATTRIBUTES=service.namespace=acme-webstore,service.version=1.4.2,deployment.environment.name=production
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=https://<OTLP_ENDPOINT>
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer YOUR_AUTH_TOKEN
NODE_OPTIONS=--import @opentelemetry/auto-instrumentations-node/register
```

## Anti-patterns

- **Missing `service.name`.**
  Telemetry appears as `unknown_service` and cannot be attributed in service maps, dashboards, or alerts.
- **Inconsistent `service.name` casing across environments.**
  Variations like `checkout` vs `Checkout` create duplicate entries in service maps and break cross-environment queries.
- **Same `service.instance.id` across multiple pods.**
  Instance-level queries return aggregated data from all pods instead of a single instance.
- **Hardcoded `service.version`.**
  Falls out of sync after the first deployment.
  Derive it from the build pipeline, git tags, or commit SHAs.
- **Missing `deployment.environment.name`.**
  Production and staging telemetry are mixed, making dashboards and alerts unreliable.

## References

- [`service.instance.id` semantic conventions](https://opentelemetry.io/docs/specs/semconv/registry/attributes/service/#service-instance-id)
- [Resource semantic conventions](https://opentelemetry.io/docs/specs/semconv/resource/)
- [OTel SDK configuration](https://opentelemetry.io/docs/languages/sdk-configuration/general/)
- [Service attributes best practices](https://www.dash0.com/guides/opentelemetry-service-attributes-best-practices)
- [Attribute placement guidance](../../otel-semantic-conventions/rules/attributes.md)
- [Kubernetes deployment](platforms/k8s.md)

---
title: "RED metrics from traces"
impact: HIGH
tags:
  - red-metrics
  - signaltometrics
  - connectors
  - histograms
  - sampling
  - semantic-conventions
---

# RED metrics from traces

RED metrics (request rate, error rate, duration distributions) derived from traces provide accurate service-level indicators without application-code changes.
Use the `signaltometricsconnector` to materialize these metrics in the Collector with metric names and attributes that match the OpenTelemetry semantic conventions.

## Choosing a connector

| Connector | Semconv alignment | Configuration | Use when |
|-----------|-------------------|---------------|----------|
| `signaltometricsconnector` | Exact — metric names, units, and attributes match the semantic conventions | OTTL expressions define each metric explicitly | Semconv compliance matters (recommended) |
| `spanmetricsconnector` | Partial — attributes can align, but metric names are generic (`{namespace}.duration`, `{namespace}.calls`) | Minimal — RED metrics are built in | You need a quick setup and do not require semconv-compliant metric names |

Use `signaltometricsconnector` for production deployments where dashboards, alerts, and SLOs rely on standard metric names.
Use `spanmetricsconnector` only for prototyping or environments where semconv metric names are not required.

## Why materialization matters

Accurate RED metrics cannot be computed from sampled traces.
Head sampling underestimates counts and skews histograms.
Tail sampling overrepresents errors and slow requests.

Generate metrics from *all* spans before any sampling occurs.
Place the metrics connector upstream of the sampling processor so it sees every span.

## Metric definitions

The [HTTP metric semantic conventions](https://opentelemetry.io/docs/specs/semconv/http/http-metrics/) and [RPC metric semantic conventions](https://opentelemetry.io/docs/specs/semconv/rpc/rpc-metrics/) define four duration histograms.
Each metric targets a specific combination of protocol and span kind.

| Metric name | Signal | Condition |
|-------------|--------|-----------|
| `http.server.request.duration` | HTTP server spans | `kind == SPAN_KIND_SERVER` and `http.request.method` present |
| `http.client.request.duration` | HTTP client spans | `kind == SPAN_KIND_CLIENT` and `http.request.method` present |
| `rpc.server.call.duration` | RPC server spans | `kind == SPAN_KIND_SERVER` and `rpc.system.name` present |
| `rpc.client.call.duration` | RPC client spans | `kind == SPAN_KIND_CLIENT` and `rpc.system.name` present |

All four metrics are histograms measured in seconds (`s`).

### Resource attributes

The `include_resource_attributes` setting controls which resource attributes are attached to the generated metrics.
Include the [required and recommended resource attributes](../../otel-instrumentation/rules/resources.md) so that generated metrics can be filtered by service, version, environment, and Kubernetes context.

| Attribute | Why |
|-----------|-----|
| `service.name` | Identifies the service producing the metric |
| `service.version` | Enables version-aware dashboards and regression detection |
| `service.namespace` | Scopes services within the same product |
| `service.instance.id` | Distinguishes individual instances |
| `deployment.environment.name` | Separates production from staging and development |
| `k8s.namespace.name` | Kubernetes namespace (set by `k8sattributes` processor) |
| `k8s.deployment.name` | Kubernetes workload name (set by `k8sattributes` processor) |

### HTTP metrics configuration

```yaml
connectors:
  signaltometrics:
    spans:
      # HTTP server duration — https://opentelemetry.io/docs/specs/semconv/http/http-metrics/
      - name: http.server.request.duration
        description: "Duration of HTTP server requests."
        unit: s
        conditions:
          - kind == SPAN_KIND_SERVER and attributes["http.request.method"] != nil
        attributes:
          - key: http.request.method
          - key: http.response.status_code
            optional: true
          - key: http.route
            optional: true
          - key: url.scheme
            optional: true
          - key: error.type
            optional: true
          - key: network.protocol.version
            optional: true
        include_resource_attributes:
          - key: service.name
          - key: service.version
            optional: true
          - key: service.namespace
            optional: true
          - key: service.instance.id
            optional: true
          - key: deployment.environment.name
            optional: true
          - key: k8s.namespace.name
            optional: true
          - key: k8s.deployment.name
            optional: true
        exponential_histogram:
          max_size: 160
          value: Seconds(end_time - start_time)
          count: "1"

      # HTTP client duration — https://opentelemetry.io/docs/specs/semconv/http/http-metrics/
      - name: http.client.request.duration
        description: "Duration of HTTP client requests."
        unit: s
        conditions:
          - kind == SPAN_KIND_CLIENT and attributes["http.request.method"] != nil
        attributes:
          - key: http.request.method
          - key: http.response.status_code
            optional: true
          - key: error.type
            optional: true
          - key: server.address
            optional: true
          - key: server.port
            optional: true
          - key: network.protocol.version
            optional: true
        include_resource_attributes:
          - key: service.name
          - key: service.version
            optional: true
          - key: service.namespace
            optional: true
          - key: service.instance.id
            optional: true
          - key: deployment.environment.name
            optional: true
          - key: k8s.namespace.name
            optional: true
          - key: k8s.deployment.name
            optional: true
        exponential_histogram:
          max_size: 160
          value: Seconds(end_time - start_time)
          count: "1"
```

### RPC metrics configuration

```yaml
connectors:
  signaltometrics:
    spans:
      # RPC server duration — https://opentelemetry.io/docs/specs/semconv/rpc/rpc-metrics/
      - name: rpc.server.call.duration
        description: "Duration of inbound RPC calls."
        unit: s
        conditions:
          - kind == SPAN_KIND_SERVER and attributes["rpc.system.name"] != nil
        attributes:
          - key: rpc.system.name
          - key: rpc.method
            optional: true
          - key: rpc.response.status_code
            optional: true
          - key: error.type
            optional: true
          - key: server.address
            optional: true
          - key: server.port
            optional: true
        include_resource_attributes:
          - key: service.name
          - key: service.version
            optional: true
          - key: service.namespace
            optional: true
          - key: service.instance.id
            optional: true
          - key: deployment.environment.name
            optional: true
          - key: k8s.namespace.name
            optional: true
          - key: k8s.deployment.name
            optional: true
        exponential_histogram:
          max_size: 160
          value: Seconds(end_time - start_time)
          count: "1"

      # RPC client duration — https://opentelemetry.io/docs/specs/semconv/rpc/rpc-metrics/
      - name: rpc.client.call.duration
        description: "Duration of outbound RPC calls."
        unit: s
        conditions:
          - kind == SPAN_KIND_CLIENT and attributes["rpc.system.name"] != nil
        attributes:
          - key: rpc.system.name
          - key: rpc.method
            optional: true
          - key: rpc.response.status_code
            optional: true
          - key: error.type
            optional: true
          - key: server.address
            optional: true
          - key: server.port
            optional: true
        include_resource_attributes:
          - key: service.name
          - key: service.version
            optional: true
          - key: service.namespace
            optional: true
          - key: service.instance.id
            optional: true
          - key: deployment.environment.name
            optional: true
          - key: k8s.namespace.name
            optional: true
          - key: k8s.deployment.name
            optional: true
        exponential_histogram:
          max_size: 160
          value: Seconds(end_time - start_time)
          count: "1"
```

### Explicit-bucket fallback

Use explicit-bucket histograms only when the backend does not support exponential histograms (e.g., when exporting to Prometheus via `prometheusremotewrite`).
Replace the `exponential_histogram` block with a `histogram` block using the bucket boundaries from the semantic conventions:

```yaml
        histogram:
          buckets: [0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10]
          value: Seconds(end_time - start_time)
          count: "1"
```

## Pipeline wiring

The `signaltometrics` connector acts as an exporter in the traces pipeline and a receiver in a metrics pipeline.
It sees every span before the backend receives the (possibly sampled) subset, producing accurate counts and duration histograms.

```yaml
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [signaltometrics, otlp]
    metrics/red:
      receivers: [signaltometrics]
      processors: [memory_limiter]
      exporters: [otlp]
```

List the `signaltometrics` connector *before* the backend exporter in the traces pipeline's `exporters` list.
The connector receives a copy of every span independently of the backend exporter.

## Full configuration with tail sampling

When combining RED metric materialization with tail sampling, the gateway must run both the `signaltometricsconnector` and the `tailsamplingprocessor`.
The connector sees all spans; only afterward does the `tailsamplingprocessor` discard unsampled traces.

```yaml
connectors:
  signaltometrics:
    spans:
      - name: http.server.request.duration
        description: "Duration of HTTP server requests."
        unit: s
        conditions:
          - kind == SPAN_KIND_SERVER and attributes["http.request.method"] != nil
        attributes:
          - key: http.request.method
          - key: http.response.status_code
            optional: true
          - key: http.route
            optional: true
          - key: url.scheme
            optional: true
          - key: error.type
            optional: true
        include_resource_attributes:
          - key: service.name
          - key: service.version
            optional: true
          - key: deployment.environment.name
            optional: true
        exponential_histogram:
          max_size: 160
          value: Seconds(end_time - start_time)
          count: "1"

      - name: http.client.request.duration
        description: "Duration of HTTP client requests."
        unit: s
        conditions:
          - kind == SPAN_KIND_CLIENT and attributes["http.request.method"] != nil
        attributes:
          - key: http.request.method
          - key: http.response.status_code
            optional: true
          - key: error.type
            optional: true
          - key: server.address
            optional: true
          - key: server.port
            optional: true
        include_resource_attributes:
          - key: service.name
          - key: service.version
            optional: true
          - key: deployment.environment.name
            optional: true
        exponential_histogram:
          max_size: 160
          value: Seconds(end_time - start_time)
          count: "1"

      - name: rpc.server.call.duration
        description: "Duration of inbound RPC calls."
        unit: s
        conditions:
          - kind == SPAN_KIND_SERVER and attributes["rpc.system.name"] != nil
        attributes:
          - key: rpc.system.name
          - key: rpc.method
            optional: true
          - key: rpc.response.status_code
            optional: true
          - key: error.type
            optional: true
        include_resource_attributes:
          - key: service.name
          - key: service.version
            optional: true
          - key: deployment.environment.name
            optional: true
        exponential_histogram:
          max_size: 160
          value: Seconds(end_time - start_time)
          count: "1"

      - name: rpc.client.call.duration
        description: "Duration of outbound RPC calls."
        unit: s
        conditions:
          - kind == SPAN_KIND_CLIENT and attributes["rpc.system.name"] != nil
        attributes:
          - key: rpc.system.name
          - key: rpc.method
            optional: true
          - key: rpc.response.status_code
            optional: true
          - key: error.type
            optional: true
          - key: server.address
            optional: true
          - key: server.port
            optional: true
        include_resource_attributes:
          - key: service.name
          - key: service.version
            optional: true
          - key: deployment.environment.name
            optional: true
        exponential_histogram:
          max_size: 160
          value: Seconds(end_time - start_time)
          count: "1"

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 1638
    spike_limit_mib: 400
  tail_sampling:
    decision_wait: 30s
    num_traces: 100000
    policies:
      - name: errors
        type: status_code
        status_code:
          status_codes:
            - ERROR
      - name: slow-traces
        type: latency
        latency:
          threshold_ms: 1000
      - name: baseline
        type: probabilistic
        probabilistic:
          sampling_percentage: 10

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, tail_sampling]
      exporters: [signaltometrics, otlp]
    metrics/red:
      receivers: [signaltometrics]
      processors: [memory_limiter]
      exporters: [otlp]
```

## Anti-patterns

- **Computing RED metrics from sampled traces.**
  Dashboards and alerts that depend on trace-derived metrics show skewed data.
  Always generate metrics from spans before the sampling step.
- **Using generic metric names instead of semconv names.**
  Metrics named `span.metrics.duration` cannot be correlated with SDK-generated metrics like `http.server.request.duration`.
  Use the `signaltometricsconnector` to produce metrics with exact semantic convention names.
- **Missing semconv attributes on generated metrics.**
  Without `http.request.method`, `http.response.status_code`, and `http.route`, the generated metrics cannot be filtered the same way as SDK-produced metrics.
  Always include the semantic convention attributes for each metric.
- **Adding high-cardinality attributes.**
  Attributes like `url.full`, `http.target`, or user IDs produce unbounded cardinality.
  Only add low-cardinality attributes listed in the semantic conventions.
- **Omitting resource attributes.**
  Without `service.version` and `deployment.environment.name` in `include_resource_attributes`, generated metrics cannot be filtered by version or environment.
  Include the [required and recommended resource attributes](../../otel-instrumentation/rules/resources.md).

## References

- [Signal to metrics connector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/signaltometricsconnector)
- [Span metrics connector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/spanmetricsconnector)
- [HTTP metric semantic conventions](https://opentelemetry.io/docs/specs/semconv/http/http-metrics/)
- [RPC metric semantic conventions](https://opentelemetry.io/docs/specs/semconv/rpc/rpc-metrics/)
- [Resource attributes](../../otel-instrumentation/rules/resources.md)
- [Sampling](./sampling.md)

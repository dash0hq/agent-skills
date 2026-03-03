---
title: "Pipelines"
impact: CRITICAL
tags:
  - pipelines
  - service
  - configuration
---

# Pipelines

Pipelines wire receivers, processors, and exporters together in the `service` section.
Every component declared in the configuration must appear in at least one pipeline — unused components cause a startup error.

## Service section structure

The `service` section has three subsections: `extensions`, `pipelines`, and `telemetry`.

```yaml
service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlp]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlp]
    logs:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlp]
  telemetry:
    logs:
      level: info
    metrics:
      address: 0.0.0.0:8888
```

## One pipeline per signal type

Define separate pipelines for traces, metrics, and logs.
Each pipeline processes exactly one signal type.

```yaml
# GOOD — one pipeline per signal
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlp]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlp]
    logs:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlp]
```

Use named pipelines when you need multiple pipelines for the same signal type (e.g., different processing for different sources):

```yaml
service:
  pipelines:
    traces/application:
      receivers: [otlp]
      processors: [memory_limiter, k8sattributes]
      exporters: [otlp/dash0]
    traces/infrastructure:
      receivers: [otlp/infra]
      processors: [memory_limiter]
      exporters: [otlp/dash0]
```

## Complete working configuration

This configuration accepts OTLP telemetry, applies recommended processors, and exports to Dash0.

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
  resourcedetection:
    detectors:
      - env
      - system
    timeout: 5s
    override: false
  resource:
    attributes:
      - key: deployment.environment.name
        value: "production"
        action: insert
exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
    compression: gzip
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
      storage: file_storage

extensions:
  health_check:
    endpoint: 0.0.0.0:13133
  file_storage:
    directory: /var/lib/otelcol/queue

service:
  extensions: [health_check, file_storage]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, resourcedetection, resource]
      exporters: [otlp]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, resourcedetection, resource]
      exporters: [otlp]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, resourcedetection, resource]
      exporters: [otlp]
  telemetry:
    logs:
      level: info
    metrics:
      address: 0.0.0.0:8888
```

Replace `<OTLP_ENDPOINT>` with your Dash0 OTLP endpoint.
Set the `DASH0_AUTH_TOKEN` environment variable from a Kubernetes secret or your deployment configuration.

## Connectors

Connectors act as both an exporter in one pipeline and a receiver in another, enabling cross-signal derivation.
Use connectors to generate metrics from spans (e.g., request rate, error rate, duration histograms) without modifying application code.

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
          - key: error.type
            optional: true
        include_resource_attributes:
          - key: service.name
        exponential_histogram:
          max_size: 160
          value: Seconds(end_time - start_time)
          count: "1"

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

The `signaltometrics` connector uses OTTL conditions to select spans and produces metrics with exact semantic convention names.
See [RED metrics from traces](./red-metrics.md) for the complete set of HTTP and RPC metric definitions, resource attributes, and histogram type guidance.
For complex telemetry transformations within connectors, see the [OTTL skill](../../otel-ottl/SKILL.md).

## Fan-out to multiple exporters

Send the same telemetry to multiple backends by listing multiple exporters in a single pipeline.
Each exporter receives an independent copy of the data.

```yaml
exporters:
  otlp/dash0:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
  otlp/secondary:
    endpoint: secondary-backend:4317
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlp/dash0, otlp/secondary]
```

Fan-out doubles memory and network usage per additional exporter.
Monitor the Collector's own metrics (see "Internal telemetry" below) to detect resource pressure.

## Internal telemetry

The Collector exposes its own metrics on port 8888 by default.
Use these metrics to monitor pipeline health, dropped data, and queue depth.

```yaml
service:
  telemetry:
    logs:
      level: info
      encoding: json
    metrics:
      address: 0.0.0.0:8888
      level: detailed
```

| Level | Metrics emitted |
|-------|----------------|
| `none` | No internal metrics |
| `basic` | Core pipeline metrics (items received, sent, dropped) |
| `normal` | Basic plus queue depth and exporter details |
| `detailed` | All available internal metrics |

Key metrics to monitor:

| Metric | What it indicates |
|--------|-------------------|
| `otelcol_receiver_accepted_spans` | Spans successfully received |
| `otelcol_receiver_refused_spans` | Spans rejected (backpressure) |
| `otelcol_exporter_sent_spans` | Spans successfully exported |
| `otelcol_exporter_send_failed_spans` | Export failures (network, auth) |
| `otelcol_exporter_queue_size` | Current queue depth |
| `otelcol_exporter_queue_capacity` | Maximum queue capacity |

Set log encoding to `json` in production for structured log ingestion.

## Validating the setup

Each deployment method has specific instructions for adding the `debug` exporter and verifying pipeline correctness.
See the validation section in your deployment model:

- [Collector Helm chart](./deployment/collector-helm-chart.md#validating-the-setup-with-the-debug-exporter)
- [OpenTelemetry Operator](./deployment/opentelemetry-operator.md#validating-the-setup-with-the-debug-exporter)
- [Raw Kubernetes manifests](./deployment/raw-manifests.md#validating-the-setup-with-the-debug-exporter)
- [Dash0 Operator](./deployment/dash0-operator.md#validating-the-setup)

For debug exporter configuration and verbosity levels, see [exporters](./exporters.md#debug-exporter).

## Anti-patterns

- **Single pipeline for all signals.**
  Pipelines are typed by signal (traces, metrics, logs).
  Attempting to mix signals in one pipeline causes a configuration error.
- **Unused components in the configuration.**
  The Collector rejects configurations with declared receivers, processors, or exporters not referenced by any pipeline.
  Remove unused components or add them to a pipeline.
- **Missing `health_check` extension.**
  Without a health check endpoint, Kubernetes liveness and readiness probes have no target.
  Always include the `health_check` extension in production deployments.
- **Internal telemetry bound to `localhost` in Kubernetes.**
  Prometheus cannot scrape metrics from `localhost` inside a pod.
  Bind to `0.0.0.0:8888` for the metrics endpoint.

## References

- [Collector configuration](https://opentelemetry.io/docs/collector/configuration/)
- [Connectors](https://opentelemetry.io/docs/collector/connectors/)
- [Signal to metrics connector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/signaltometricsconnector)
- [Health check extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/healthcheckextension)

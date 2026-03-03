---
title: "Sampling"
impact: HIGH
tags:
  - sampling
  - head-sampling
  - tail-sampling
  - probabilistic-sampler
  - load-balancing
---

# Sampling

Sampling reduces the volume of trace data by selectively retaining only a subset of traces.
It is an unavoidable necessity in large distributed systems, but every sampling strategy involves trade-offs.

Application SDKs should use the `AlwaysOn` sampler (the default) and export every span.
All sampling decisions belong in the Collector or observability pipeline, where they can be changed centrally without redeploying applications.
See [SDK sampling guidance](../../otel-instrumentation/rules/spans.md#sampling) for why.

## Decision table

| Requirement | Approach | Collector component |
|-------------|----------|---------------------|
| Reduce trace volume by a fixed percentage | Head sampling | [`probabilisticsamplerprocessor`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/probabilisticsamplerprocessor) |
| Keep all errors, slow traces, and a baseline of normal traces | Tail sampling | [`tailsamplingprocessor`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor) |
| Route all spans of a trace to the same Collector instance | Trace-aware load balancing | [`loadbalancingexporter`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/loadbalancingexporter) |
| Generate accurate RED metrics from all traces before sampling | [Metric materialization](./red-metrics.md) | [`signaltometricsconnector`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/signaltometricsconnector) |

## Head sampling

Head sampling discards traces by a fixed probability, evaluated deterministically from the trace ID.
Every Collector instance in the fleet reaches the same decision for the same trace without coordination.

Use the `probabilisticsamplerprocessor` in the traces pipeline:

```yaml
processors:
  probabilistic_sampler:
    sampling_percentage: 10

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, probabilistic_sampler]
      exporters: [otlp]
```

Head sampling is simple to deploy and scales horizontally, but it has a fundamental limitation: the decision is made before the outcome of the request is known.
At low sampling rates, errors, latency spikes, and rarely exercised code paths are likely to be missed.

Use head sampling only when tail sampling is too complex for the environment, or as a coarse volume reduction before tail sampling.

## Tail sampling

Tail sampling collects all spans for a trace, then decides whether the trace is worth keeping based on its full content.
It can retain errors, slow requests, unusual traces, and a configurable baseline of normal traffic.

### Architecture

Tail sampling requires all spans of a trace to converge at a single Collector instance.
This requires a two-tier architecture:

```
Applications → Agent (DaemonSet) → Gateway (sampling layer) → Backend
                     ↑                        ↑
              loadbalancingexporter    tailsamplingprocessor
              routes by trace ID      buffers and decides
```

1. **Agent tier** (DaemonSet): receives spans from local applications via the OTLP receiver.
   Uses the `loadbalancingexporter` to consistently hash the trace ID and route all spans of a trace to the same gateway instance.
2. **Gateway tier** (Deployment): runs the `tailsamplingprocessor`, which buffers spans, evaluates sampling policies, and either forwards or drops the trace.

### Agent configuration

The agent uses the `loadbalancingexporter` instead of a standard OTLP exporter for traces.
Metrics and logs bypass the sampling layer and are exported directly.

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
    limit_mib: 410
    spike_limit_mib: 100

exporters:
  loadbalancing:
    protocol:
      otlp:
        tls:
          insecure: true # Validate if this is needed, and avoid if possible
    resolver:
      dns:
        hostname: otel-collector-gateway-headless.otel.svc.cluster.local
        port: 4317
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [loadbalancing]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlp]
    logs:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlp]
```

The `loadbalancingexporter` uses DNS to discover gateway instances.
Create a [headless Service](https://kubernetes.io/docs/concepts/services-networking/headless-services/) (with `clusterIP: None`) for the gateway so that DNS returns individual pod IPs rather than a virtual IP.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: otel-collector-gateway-headless
  namespace: otel
spec:
  clusterIP: None
  ports:
    - name: otlp-grpc
      port: 4317
      targetPort: otlp-grpc
      protocol: TCP
  selector:
    app.kubernetes.io/name: otel-collector
    app.kubernetes.io/component: gateway
```

### Gateway configuration

The gateway runs the `tailsamplingprocessor` with policies that determine which traces to keep.

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 1638
    spike_limit_mib: 400
  tail_sampling:
    decision_wait: 30s
    num_traces: 100000
    expected_new_traces_per_sec: 1000
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

exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
    compression: gzip
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
      storage: file_storage

extensions:
  file_storage:
    directory: /var/lib/otelcol/queue

service:
  extensions: [file_storage]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, tail_sampling]
      exporters: [otlp]
```

### Policy design

Design sampling policies around how *interesting* a trace is, not just whether it contains errors.

| Policy type | What it keeps | When to use |
|-------------|---------------|-------------|
| `status_code` | Traces with at least one `ERROR` span | Always — errors are high-signal |
| `latency` | Traces exceeding a duration threshold | Always — slow requests indicate problems |
| `probabilistic` | A random baseline of normal traces | Always — baseline is needed for comparison |
| `string_attribute` | Traces matching specific attribute values | When specific operations or tenants are high-priority |
| `rate_limiting` | A fixed number of traces per second | When you need predictable volume limits |

Combine multiple policies.
The `tailsamplingprocessor` evaluates all policies and keeps a trace if *any* policy matches.

### Sizing the tail sampling buffer

The `tailsamplingprocessor` buffers spans in memory while waiting for traces to complete.
Distributed traces have no end-of-trace marker — the processor waits for `decision_wait` seconds, then makes a decision with whatever spans have arrived.

| Setting | Description | Guidance |
|---------|-------------|----------|
| `decision_wait` | Time to wait before making a sampling decision | Start with `30s`.  Increase for environments with long-running operations, but be aware that longer waits increase memory usage. |
| `num_traces` | Maximum number of traces held in memory | Set based on expected trace throughput and `decision_wait`. If exceeded, the oldest traces are force-decided. |
| `expected_new_traces_per_sec` | Hint for internal sizing | Set to approximate traces per second to pre-allocate internal structures. |

Memory usage scales linearly with `decision_wait * traces_per_second * average_spans_per_trace * average_span_size`.
Set the Collector container memory limit accordingly, and set `memory_limiter.limit_mib` to 80 percent of that limit.

### Operational complexity

Tail sampling adds significant operational complexity:

- The two tiers must be scaled and monitored independently.
- Consistent hashing must remain stable during gateway scaling events.
  DNS-based discovery (used by the `loadbalancingexporter`) is eventually consistent — scaling the gateway may temporarily route spans of the same trace to different instances.
- The sampling layer is stateful.
  Gateway pods cannot be terminated without losing buffered spans.
  Use `preStop` hooks and graceful shutdown periods to drain in-flight data.
- Cross-availability-zone traffic increases networking costs.
  The `loadbalancingexporter` routes by trace ID, not by zone, so spans frequently cross zone boundaries.

## Materialize RED metrics before sampling

Accurate RED metrics cannot be computed from sampled traces — head sampling skews counts, tail sampling overrepresents errors.
Always generate metrics from all spans before any sampling occurs.

See [RED metrics from traces](./red-metrics.md) for connector configuration, histogram type selection, and a full configuration example combining RED metric materialization with tail sampling.

## Anti-patterns

- **Sampling in the SDK.**
  Configuring `TraceIdRatioBased` or other non-`AlwaysOn` samplers in the application loses traces before their outcome is known.
  Use `AlwaysOn` in all SDKs and sample in the Collector.
- **Tail sampling without RED metric materialization.**
  Dashboards and alerts that depend on trace-derived metrics show skewed data.
  Always generate metrics from spans before the sampling step — see [RED metrics from traces](./red-metrics.md).
- **Single-tier tail sampling.**
  Running `tailsamplingprocessor` on agents (DaemonSet) without a `loadbalancingexporter` means each agent only sees spans from local pods.
  Traces that span multiple nodes are split across agents, and sampling decisions are made on incomplete data.
- **Short `decision_wait` with long-running operations.**
  A `decision_wait` of 10 seconds misses spans from batch jobs or async workflows that take minutes.
  Traces are force-decided before all spans arrive, leading to incomplete or incorrect sampling decisions.
- **Gateway without a headless Service.**
  The `loadbalancingexporter` needs individual pod IPs from DNS.
  A regular ClusterIP Service returns a virtual IP, defeating consistent hashing.

## References

- [Probabilistic sampler processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/probabilisticsamplerprocessor)
- [Tail sampling processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor)
- [Load balancing exporter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/loadbalancingexporter)
- [RED metrics from traces](./red-metrics.md)
- [SDK sampling guidance](../../otel-instrumentation/rules/spans.md#sampling)

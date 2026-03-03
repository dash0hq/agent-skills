# otel-collector

Expert guidance for configuring and deploying the OpenTelemetry Collector to receive, process, and export telemetry to Dash0 or any OTLP-compatible backend.

## Structure

```
otel-collector/
├── SKILL.md              # Skill manifest and entry point
├── README.md             # This file
└── rules/
    ├── receivers.md      # OTLP, Prometheus, filelog, hostmetrics
    ├── exporters.md      # OTLP/gRPC to Dash0, debug, authentication
    ├── processors.md     # Memory limiter, resource detection, ordering
    ├── pipelines.md      # Service section, per-signal config, connectors
    ├── deployment.md     # Agent vs gateway patterns, deployment method selection
    ├── deployment/
    │   ├── collector-helm-chart.md    # Collector Helm chart
    │   ├── opentelemetry-operator.md  # OTel Operator and auto-instrumentation
    │   ├── dash0-operator.md         # Dash0 Kubernetes Operator
    │   └── raw-manifests.md         # DaemonSet, Deployment, RBAC, Docker Compose
    ├── sampling.md       # Head sampling, tail sampling, load balancing
    └── red-metrics.md    # Semconv-aligned RED metrics from traces
```

## Getting started

Install the skill:

```bash
npx skills add dash0/otel-collector
```

The skill activates automatically when working on Collector configuration tasks.

## Rules

| Rule | Impact | Description |
|------|--------|-------------|
| [receivers](./rules/receivers.md) | CRITICAL | OTLP, Prometheus, filelog, and hostmetrics receiver configuration |
| [exporters](./rules/exporters.md) | CRITICAL | OTLP/gRPC exporter to Dash0 with authentication, retry, and queuing |
| [processors](./rules/processors.md) | HIGH | Required and recommended processors with ordering rules |
| [pipelines](./rules/pipelines.md) | CRITICAL | Service section, per-signal pipelines, complete working config |
| [deployment](./rules/deployment.md) | HIGH | Agent vs gateway patterns, deployment method selection |
| [raw-manifests](./rules/deployment/raw-manifests.md) | HIGH | Raw Kubernetes manifests — DaemonSet, Deployment, RBAC, Docker Compose |
| [dash0-operator](./rules/deployment/dash0-operator.md) | HIGH | Dash0 Kubernetes Operator — automated instrumentation and Collector management |
| [sampling](./rules/sampling.md) | HIGH | Head sampling, tail sampling, load balancing |
| [red-metrics](./rules/red-metrics.md) | HIGH | Semconv-aligned RED metrics from traces |

## Quick start

**Get your credentials:**
- **OTLP Endpoint**: In Dash0: [Settings → Organization → Endpoints → OTLP via HTTP](https://app.dash0.com/goto/settings/endpoints?endpoint_type=otlp_http) or [Settings → Organization → Endpoints → OTLP via gRPC](https://app.dash0.com/goto/settings/endpoints?endpoint_type=otlp_grpc)
- **Auth Token**: In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)

**Minimal working configuration:**

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

exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer <AUTH_TOKEN>"
    sending_queue:
      enabled: true
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

Replace the placeholders with values from your Dash0 account:
- `<OTLP_ENDPOINT>`: copy the gRPC endpoint from [Settings → Endpoints → OTLP via gRPC](https://app.dash0.com/goto/settings/endpoints?endpoint_type=otlp_grpc) (e.g., `ingress.eu-west-1.aws.dash0.com:4317`).
- `<AUTH_TOKEN>`: create or copy a token from [Settings → Auth Tokens](https://app.dash0.com/settings/auth-tokens).

## Key principles

- **Processor ordering matters** — `memory_limiter` first in every pipeline.
- **One pipeline per signal** — separate pipelines for traces, metrics, and logs.
- **Memory safety is non-negotiable** — always configure `memory_limiter` in production.
- **Every component must be in a pipeline** — declared but unused components cause startup failure.

## Resources

- [OpenTelemetry Collector docs](https://opentelemetry.io/docs/collector/)
- [Dash0 Integration Hub](https://www.dash0.com/hub/integrations)
- [Dash0 Guides](https://www.dash0.com/guides?category=opentelemetry)

## License

MIT

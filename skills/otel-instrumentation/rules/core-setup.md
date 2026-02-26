---
title: "SDK Setup and Backend Integration"
impact: CRITICAL
tags:
  - setup
  - sdk
  - configuration
  - otlp
  - backend
---

# Setup Reference

General setup guide for OpenTelemetry SDK and backend integration.

## Essential Environment Variables

Copy-paste ready configuration:

```bash
# Required
export OTEL_SERVICE_NAME="my-service"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer ${DASH0_AUTH_TOKEN}"

# Recommended
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.0.0,deployment.environment.name=production"
export OTEL_TRACES_SAMPLER="parentbased_traceidratio"
export OTEL_TRACES_SAMPLER_ARG="0.1"
```

---

## Resource Attributes Quick Reference

| Attribute | Required | Example | Purpose |
|-----------|----------|---------|---------|
| `service.name` | **Yes** | `payment-service` | Identifies your service in traces |
| `service.version` | Recommended | `2.1.0` | Correlate issues with deployments |
| `deployment.environment.name` | Recommended | `production` | Filter by environment |
| `service.namespace` | Optional | `checkout-team` | Group related services |
| `service.instance.id` | Optional | `pod-abc-123` | Identify specific instance |

### service.namespace

Groups related services together. Useful when you have many services and want to filter by team or domain:

```bash
# All services in the checkout domain
OTEL_RESOURCE_ATTRIBUTES="service.namespace=checkout,service.version=1.0.0"
```

### deployment.environment.name

Identifies the deployment environment. Standard values:

- `development` - Local development
- `staging` - Pre-production testing
- `production` - Live production

```bash
OTEL_RESOURCE_ATTRIBUTES="deployment.environment.name=production"
```

---

## Official Documentation

- [Getting Started](https://opentelemetry.io/docs/getting-started/)
- [SDK Configuration](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Resource Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/resource/)

---

## Protocol Selection

| Protocol | Port | Use When |
|----------|------|----------|
| gRPC | 4317 | Default, best performance |
| HTTP/protobuf | 4318 | Firewalls blocking gRPC |
| HTTP/JSON | 4318 | Debugging only |

```bash
# gRPC (default)
export OTEL_EXPORTER_OTLP_PROTOCOL="grpc"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://collector:4317"

# HTTP/protobuf
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://collector:4318"
```

---

## Authentication

### Bearer Token (Dash0, most backends)

```bash
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer ${DASH0_AUTH_TOKEN}"
```

### API Key

```bash
export OTEL_EXPORTER_OTLP_HEADERS="x-api-key=${API_KEY}"
```

---

## Sampling Configuration

```bash
# Recommended: respect parent decisions, sample 10% of new traces
export OTEL_TRACES_SAMPLER="parentbased_traceidratio"
export OTEL_TRACES_SAMPLER_ARG="0.1"
```

| Sampler | Description |
|---------|-------------|
| `always_on` | Sample everything (dev only) |
| `always_off` | Disable tracing |
| `traceidratio` | Sample X% of traces |
| `parentbased_traceidratio` | Follow parent, default X% |

---

## Batch Export Tuning

```bash
# Traces
export OTEL_BSP_SCHEDULE_DELAY="5000"         # Export every 5s
export OTEL_BSP_MAX_QUEUE_SIZE="2048"         # Max queued spans
export OTEL_BSP_MAX_EXPORT_BATCH_SIZE="512"   # Spans per batch

# Metrics
export OTEL_METRIC_EXPORT_INTERVAL="60000"    # Export every 60s
```

---

## Dash0 Integration

### Endpoint

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
# or for US: https://ingress.us-east-1.dash0.com:4317
```

### Complete Setup

```bash
export OTEL_SERVICE_NAME="my-service"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer ${DASH0_AUTH_TOKEN}"
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.0.0,deployment.environment.name=production"
```

### Collector Configuration (if using OTel Collector)

```yaml
exporters:
  otlp/dash0:
    endpoint: ingress.eu-west-1.dash0.com:4317
    headers:
      Authorization: "Bearer ${DASH0_AUTH_TOKEN}"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/dash0]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/dash0]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/dash0]
```

---

## Verification

### 1. Check Startup Logs

```
[otel] Tracer provider initialized
[otel] Exporting to https://ingress.eu-west-1.dash0.com:4317
```

### 2. Test Connectivity

```bash
# gRPC endpoint
grpcurl ingress.eu-west-1.dash0.com:4317 list

# HTTP endpoint
curl -v https://ingress.eu-west-1.dash0.com:4318/v1/traces \
  -H "Authorization: Bearer ${DASH0_AUTH_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{}'
```

### 3. Generate Test Span

```javascript
const span = tracer.startSpan("test.verification");
span.setAttribute("test", true);
span.end();
await provider.forceFlush();
```

---

## Troubleshooting

| Issue | Check | Fix |
|-------|-------|-----|
| No data in backend | `OTEL_SERVICE_NAME` set? | Set the variable |
| Connection refused | Endpoint URL correct? | Verify port (4317 vs 4318) |
| 401 errors | Auth header format? | Use `Authorization=Bearer token` |
| Data delayed | Batch settings? | Reduce `OTEL_BSP_SCHEDULE_DELAY` |
| Missing attributes | Resource attributes set? | Check `OTEL_RESOURCE_ATTRIBUTES` |

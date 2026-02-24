# Setup Reference

General setup guide for OpenTelemetry SDK and backend integration.

## Official Documentation

- [Getting Started](https://opentelemetry.io/docs/getting-started/)
- [SDK Configuration](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [OTLP Specification](https://opentelemetry.io/docs/specs/otlp/)
- [Resource Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/resource/)
- [Language SDKs](https://opentelemetry.io/docs/languages/)

## Setup Checklist

### 1. Resource Configuration

Every telemetry signal needs resource attributes to identify the source:

```yaml
# Required
resource.attributes:
  service.name: my-service          # REQUIRED - identifies your service
  service.version: 1.2.3            # Deployment version
  deployment.environment: production # prod, staging, dev

# Recommended
resource.attributes:
  service.namespace: my-team        # Logical grouping
  service.instance.id: pod-abc123   # Unique instance identifier

# Auto-detected (SDK usually handles these)
resource.attributes:
  host.name: hostname
  process.pid: 12345
  telemetry.sdk.name: opentelemetry
  telemetry.sdk.language: nodejs
  telemetry.sdk.version: 1.x.x
```

### 2. OTLP Endpoint Configuration

```yaml
# Environment variables
OTEL_EXPORTER_OTLP_ENDPOINT: https://collector.example.com:4317
OTEL_EXPORTER_OTLP_HEADERS: "authorization=Bearer token123"

# Or protocol-specific
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: https://traces.example.com:4317
OTEL_EXPORTER_OTLP_METRICS_ENDPOINT: https://metrics.example.com:4317
OTEL_EXPORTER_OTLP_LOGS_ENDPOINT: https://logs.example.com:4317
```

### 3. Protocol Selection

```yaml
# gRPC (default, recommended)
OTEL_EXPORTER_OTLP_PROTOCOL: grpc
endpoint: collector:4317

# HTTP/protobuf (for environments blocking gRPC)
OTEL_EXPORTER_OTLP_PROTOCOL: http/protobuf
endpoint: https://collector:4318/v1/traces

# HTTP/JSON (debugging, lowest performance)
OTEL_EXPORTER_OTLP_PROTOCOL: http/json
endpoint: https://collector:4318/v1/traces
```

### 4. Sampling Configuration

```yaml
# Trace sampling
OTEL_TRACES_SAMPLER: parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG: 0.1  # 10% sampling

# Available samplers
samplers:
  always_on: Sample all traces
  always_off: Sample no traces
  traceidratio: Sample X% of traces
  parentbased_always_on: Respect parent, default on
  parentbased_always_off: Respect parent, default off
  parentbased_traceidratio: Respect parent, default X%
```

### 5. Batch Export Configuration

```yaml
# Traces
OTEL_BSP_SCHEDULE_DELAY: 5000        # Export interval (ms)
OTEL_BSP_MAX_QUEUE_SIZE: 2048        # Max queued spans
OTEL_BSP_MAX_EXPORT_BATCH_SIZE: 512  # Spans per batch
OTEL_BSP_EXPORT_TIMEOUT: 30000       # Export timeout (ms)

# Metrics
OTEL_METRIC_EXPORT_INTERVAL: 60000   # Collection interval (ms)
OTEL_METRIC_EXPORT_TIMEOUT: 30000    # Export timeout (ms)
```

## Authentication Patterns

### Bearer Token

```yaml
# Environment variable
OTEL_EXPORTER_OTLP_HEADERS: "authorization=Bearer your-token-here"

# Or in code
exporterOptions:
  headers:
    authorization: "Bearer ${API_TOKEN}"
```

### API Key

```yaml
# Custom header
OTEL_EXPORTER_OTLP_HEADERS: "x-api-key=your-api-key"

# Or vendor-specific
OTEL_EXPORTER_OTLP_HEADERS: "dd-api-key=your-datadog-key"
```

### mTLS (Mutual TLS)

```yaml
# Certificate configuration
OTEL_EXPORTER_OTLP_CERTIFICATE: /path/to/ca.crt
OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE: /path/to/client.crt
OTEL_EXPORTER_OTLP_CLIENT_KEY: /path/to/client.key
```

## Common Environment Variables

```bash
# Core
export OTEL_SERVICE_NAME="my-service"
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.0.0,deployment.environment=production"

# Exporter
export OTEL_EXPORTER_OTLP_ENDPOINT="https://collector:4317"
export OTEL_EXPORTER_OTLP_HEADERS="authorization=Bearer token"
export OTEL_EXPORTER_OTLP_PROTOCOL="grpc"

# Sampling
export OTEL_TRACES_SAMPLER="parentbased_traceidratio"
export OTEL_TRACES_SAMPLER_ARG="0.1"

# Propagation
export OTEL_PROPAGATORS="tracecontext,baggage"

# Debug
export OTEL_LOG_LEVEL="info"  # debug, info, warn, error
```

## Verification Steps

### 1. Check SDK Initialization

Look for startup logs indicating OTel is configured:

```
[otel] Tracer provider initialized
[otel] Meter provider initialized
[otel] Logger provider initialized
[otel] Exporting to https://collector:4317
```

### 2. Verify Network Connectivity

```bash
# Test OTLP gRPC endpoint
grpcurl -plaintext collector:4317 list

# Test OTLP HTTP endpoint
curl -v https://collector:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{}'

# Should return 200 or 400 (not connection error)
```

### 3. Generate Test Telemetry

```javascript
// Create a test span
const span = tracer.startSpan('test.span');
span.setAttribute('test', true);
span.end();

// Force flush to ensure export
await provider.forceFlush();
```

### 4. Verify in Backend

- Check backend UI for test data
- Verify resource attributes are correct
- Confirm sampling is working as expected

## Troubleshooting

### No Data Appearing

```yaml
checklist:
  - [ ] OTEL_SERVICE_NAME is set
  - [ ] OTEL_EXPORTER_OTLP_ENDPOINT is correct
  - [ ] Network allows outbound to collector
  - [ ] Authentication headers are set
  - [ ] SDK is initialized before application code
  - [ ] Sampling rate > 0
  - [ ] forceFlush() called on shutdown
```

### Partial Data

```yaml
checklist:
  - [ ] Auto-instrumentation enabled for all libraries
  - [ ] Context propagation working (check headers)
  - [ ] Trace context not lost in async code
  - [ ] All services using same propagator format
```

### High Memory/CPU

```yaml
checklist:
  - [ ] Batch processor configured appropriately
  - [ ] Not creating spans in tight loops
  - [ ] Attribute count/size within limits
  - [ ] Sampling enabled for high-volume endpoints
```

---

## Dash0 Integration

### Overview

[Dash0](https://dash0.com) is an OpenTelemetry-native observability platform that accepts OTLP data directly.

### OTLP Endpoint

```yaml
# Dash0 ingress endpoint
endpoint: https://ingress.${REGION}.dash0.com:4317

# Available regions
regions:
  - eu-west-1
  - us-east-1
```

### Authentication

```yaml
# Dash0 uses Bearer token authentication
headers:
  authorization: "Bearer ${DASH0_AUTH_TOKEN}"

# Environment variable
OTEL_EXPORTER_OTLP_HEADERS: "authorization=Bearer ${DASH0_AUTH_TOKEN}"
```

### Quick Setup

```bash
# Set environment variables
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
export OTEL_EXPORTER_OTLP_HEADERS="authorization=Bearer your-dash0-token"
export OTEL_SERVICE_NAME="my-service"
```

### Collector Configuration for Dash0

```yaml
exporters:
  otlp/dash0:
    endpoint: ingress.eu-west-1.dash0.com:4317
    headers:
      authorization: "Bearer ${DASH0_AUTH_TOKEN}"
    tls:
      insecure: false

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

### Dash0 Dashboard Recommendations

```yaml
# Key dashboards to create
dashboards:
  - name: Service Overview
    panels:
      - Request rate (from traces)
      - Error rate (from traces)
      - Latency percentiles (from traces)
      - Active instances (from metrics)

  - name: Error Analysis
    panels:
      - Error traces by service
      - Error distribution by type
      - Error trends over time

  - name: Performance
    panels:
      - Latency by endpoint
      - Slow trace analysis
      - Database query performance
```

### Dash0 Resources

- [Dash0 Documentation](https://www.dash0.com/documentation)
- [OTLP Integration Guide](https://www.dash0.com/documentation/opentelemetry)
- [Dash0 Support](https://www.dash0.com/support)

---

## Other Backend Integrations

### Grafana Cloud

```yaml
# Tempo (traces)
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: https://tempo-prod-XX.grafana.net:443

# Mimir (metrics)
OTEL_EXPORTER_OTLP_METRICS_ENDPOINT: https://mimir-prod-XX.grafana.net:443

# Loki (logs)
OTEL_EXPORTER_OTLP_LOGS_ENDPOINT: https://logs-prod-XX.grafana.net:443

# Authentication
OTEL_EXPORTER_OTLP_HEADERS: "authorization=Basic $(echo -n 'user:api-key' | base64)"
```

### Datadog

```yaml
# Use Datadog Agent as collector
OTEL_EXPORTER_OTLP_ENDPOINT: http://datadog-agent:4317

# Or direct to Datadog
DD_API_KEY: your-datadog-api-key
DD_SITE: datadoghq.com
```

### Honeycomb

```yaml
OTEL_EXPORTER_OTLP_ENDPOINT: https://api.honeycomb.io:443
OTEL_EXPORTER_OTLP_HEADERS: "x-honeycomb-team=your-api-key"
```

### New Relic

```yaml
OTEL_EXPORTER_OTLP_ENDPOINT: https://otlp.nr-data.net:4317
OTEL_EXPORTER_OTLP_HEADERS: "api-key=your-new-relic-key"
```

### Self-Hosted (Tempo + Prometheus)

```yaml
# Traces to Grafana Tempo
traces:
  endpoint: http://tempo:4317

# Metrics to Prometheus
metrics:
  endpoint: http://prometheus:9090/api/v1/otlp
  # Or use prometheus exporter
```

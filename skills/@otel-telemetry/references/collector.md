# Collector Reference

Comprehensive guide to OpenTelemetry Collector configuration.

## Official Documentation

- [Collector Overview](https://opentelemetry.io/docs/collector/)
- [Collector Configuration](https://opentelemetry.io/docs/collector/configuration/)
- [Collector Deployment](https://opentelemetry.io/docs/collector/deployment/)
- [Collector Components](https://opentelemetry.io/docs/collector/configuration/)
- [Tail Sampling Processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor)

## Tools

- [OTelBin](https://www.otelbin.io/) - Free editing, visualization, and validation tool for OpenTelemetry Collector configurations. Use this to validate and debug your collector configs before deploying.

## Why Use a Collector

### Benefits

1. **Vendor Agnostic** - Change backends without changing code
2. **Centralized Policy** - Sampling, filtering, transformation in one place
3. **Reduced SDK Complexity** - SDKs just export to Collector
4. **Cross-Service Consistency** - Same policies across all services
5. **Tail Sampling** - Make decisions after seeing complete traces
6. **Buffer and Retry** - Handle backend outages gracefully

### Deployment Patterns

```
Pattern 1: Agent (per-node)
┌─────────────────────────┐
│  Node                   │
│  ┌─────┐ ┌───────────┐ │
│  │ App │→│ Collector │──┼──→ Backend
│  └─────┘ │  (agent)  │ │
│  ┌─────┐ │           │ │
│  │ App │→│           │ │
│  └─────┘ └───────────┘ │
└─────────────────────────┘

Pattern 2: Gateway (centralized)
┌─────┐    ┌───────────┐
│ App │───→│           │
└─────┘    │ Collector │──→ Backend
┌─────┐    │ (gateway) │
│ App │───→│           │
└─────┘    └───────────┘

Pattern 3: Agent + Gateway (recommended)
┌─────────────────────────┐
│  Node                   │
│  ┌─────┐ ┌───────────┐ │    ┌───────────┐
│  │ App │→│ Collector │──┼───→│ Collector │──→ Backend
│  └─────┘ │  (agent)  │ │    │ (gateway) │
└─────────────────────────┘    └───────────┘
```

## Pipeline Architecture

### Basic Structure

```yaml
receivers:     # How data enters the Collector
processors:    # How data is transformed
exporters:     # Where data is sent
extensions:    # Health checks, service discovery
service:       # Pipeline wiring
```

### Example Configuration

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 1s
    send_batch_size: 1024

  memory_limiter:
    check_interval: 1s
    limit_mib: 1000
    spike_limit_mib: 200

exporters:
  otlp:
    endpoint: backend.example.com:4317
    headers:
      authorization: "Bearer ${AUTH_TOKEN}"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp]

    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp]

    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp]
```

## Filter Processor

### Drop Unwanted Data

```yaml
processors:
  filter/traces:
    error_mode: ignore
    traces:
      span:
        # Drop health checks
        - 'attributes["http.route"] == "/health"'
        - 'attributes["http.route"] == "/ready"'
        - 'attributes["http.route"] == "/live"'
        - 'attributes["http.route"] == "/ping"'

        # Drop synthetic traffic
        - 'attributes["http.user_agent"] contains "synthetic"'
        - 'attributes["http.user_agent"] contains "pingdom"'

        # Drop internal probes
        - 'attributes["http.user_agent"] contains "kube-probe"'

  filter/metrics:
    error_mode: ignore
    metrics:
      metric:
        # Drop internal metrics
        - 'name == "internal.debug.metric"'

      datapoint:
        # Drop development environment
        - 'attributes["environment"] == "development"'

  filter/logs:
    error_mode: ignore
    logs:
      log_record:
        # Drop debug logs in production
        - 'severity_number < 9 and resource.attributes["deployment.environment"] == "production"'

        # Drop noisy logs
        - 'body contains "health check"'
```

### OTTL Expressions

```yaml
# Common OTTL patterns
expressions:
  # Attribute matching
  - 'attributes["key"] == "value"'
  - 'attributes["key"] != "value"'

  # Contains check
  - 'attributes["path"] contains "/api"'

  # Regex match
  - 'attributes["path"] matches "^/api/v[0-9]+"'

  # Numeric comparison
  - 'attributes["status_code"] >= 400'

  # Resource attributes
  - 'resource.attributes["service.name"] == "api"'

  # Span-specific
  - 'status.code == STATUS_CODE_ERROR'
  - 'kind == SPAN_KIND_SERVER'

  # Log-specific
  - 'severity_number >= 17'  # ERROR and above
```

## Batch Processor

### Configuration

```yaml
processors:
  batch:
    # Time to wait before sending
    timeout: 1s

    # Max batch size
    send_batch_size: 1024
    send_batch_max_size: 2048
```

### Tuning Guidelines

```yaml
# Low latency (real-time monitoring)
batch/low_latency:
  timeout: 200ms
  send_batch_size: 256

# High throughput (batch processing)
batch/high_throughput:
  timeout: 5s
  send_batch_size: 8192

# Balanced (general use)
batch/balanced:
  timeout: 1s
  send_batch_size: 1024
```

## Tail Sampling Processor

### Why Tail Sampling

```
Head sampling: Decision at trace start
- Pro: No memory overhead
- Con: Can't see if trace will have errors

Tail sampling: Decision after trace complete
- Pro: Can keep all errors, slow traces
- Con: Must buffer traces (memory cost)
```

### Configuration

```yaml
processors:
  tail_sampling:
    # How long to wait for trace to complete
    decision_wait: 10s

    # Number of traces to keep in memory
    num_traces: 100000

    # Expected spans per trace
    expected_new_traces_per_sec: 1000

    policies:
      # Policy 1: Always keep errors
      - name: errors-policy
        type: status_code
        status_code:
          status_codes: [ERROR]

      # Policy 2: Always keep slow traces
      - name: latency-policy
        type: latency
        latency:
          threshold_ms: 1000

      # Policy 3: Keep traces with specific attributes
      - name: important-policy
        type: string_attribute
        string_attribute:
          key: important
          values: [true]

      # Policy 4: Sample remaining traces
      - name: probabilistic-policy
        type: probabilistic
        probabilistic:
          sampling_percentage: 5
```

### Policy Types

```yaml
# Status code sampling
- name: error-traces
  type: status_code
  status_code:
    status_codes: [ERROR, UNSET]

# Latency-based
- name: slow-traces
  type: latency
  latency:
    threshold_ms: 500

# Rate limiting
- name: rate-limit
  type: rate_limiting
  rate_limiting:
    spans_per_second: 100

# String attribute match
- name: by-service
  type: string_attribute
  string_attribute:
    key: service.name
    values: [critical-service]

# Numeric attribute
- name: by-status
  type: numeric_attribute
  numeric_attribute:
    key: http.status_code
    min_value: 500
    max_value: 599

# Composite (AND logic)
- name: composite
  type: composite
  composite:
    max_total_spans_per_second: 1000
    policy_order: [error-policy, latency-policy]
    composite_sub_policy:
      - name: error-policy
        type: status_code
        status_code:
          status_codes: [ERROR]
      - name: latency-policy
        type: latency
        latency:
          threshold_ms: 1000
```

### Memory Considerations

```yaml
# Calculate memory needs
# traces × avg_spans_per_trace × bytes_per_span
# 100,000 × 10 × 2KB = ~2GB

processors:
  tail_sampling:
    num_traces: 100000  # Adjust based on memory
    decision_wait: 10s   # Reduce if memory constrained
```

## Rate Limiting

### Per-Pipeline Limits

```yaml
processors:
  # Limit spans per second
  probabilistic_sampler:
    sampling_percentage: 10

  # Hard rate limit
  rate_limiter:
    rate_limit: 1000  # items per second
```

### Memory Limiter

```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 1000       # Hard limit
    spike_limit_mib: 200  # Additional buffer for spikes

    # What to do when limit reached
    # - refuse: reject new data
    # - drop: drop oldest data
```

## Multi-Pipeline Isolation

### Separate by Priority

```yaml
service:
  pipelines:
    # High priority - errors, critical traces
    traces/high_priority:
      receivers: [otlp]
      processors:
        - filter/high_priority
        - batch
      exporters: [otlp/fast]

    # Low priority - bulk traces
    traces/low_priority:
      receivers: [otlp]
      processors:
        - filter/low_priority
        - tail_sampling
        - batch
      exporters: [otlp/standard]

processors:
  filter/high_priority:
    traces:
      span:
        # Keep only errors and critical
        - 'status.code != STATUS_CODE_ERROR and attributes["priority"] != "critical"'

  filter/low_priority:
    traces:
      span:
        # Everything else
        - 'status.code == STATUS_CODE_ERROR or attributes["priority"] == "critical"'
```

### Separate by Destination

```yaml
exporters:
  otlp/primary:
    endpoint: primary-backend:4317

  otlp/secondary:
    endpoint: secondary-backend:4317

  file/archive:
    path: /var/log/otel/archive

service:
  pipelines:
    traces/primary:
      receivers: [otlp]
      processors: [filter/sampled, batch]
      exporters: [otlp/primary]

    traces/archive:
      receivers: [otlp]
      processors: [batch]
      exporters: [file/archive]
```

## Common Configuration Scenarios

### Scenario 1: Cost Reduction

```yaml
# Goal: Reduce trace volume by 90%
processors:
  # Drop health checks
  filter/drop_noise:
    traces:
      span:
        - 'attributes["http.route"] in ["/health", "/ready", "/metrics"]'
        - 'attributes["http.user_agent"] contains "kube-probe"'

  # Keep errors, sample rest
  tail_sampling:
    policies:
      - name: errors
        type: status_code
        status_code: {status_codes: [ERROR]}
      - name: sample
        type: probabilistic
        probabilistic: {sampling_percentage: 10}
```

### Scenario 2: Multi-Tenant

```yaml
# Goal: Route traces by tenant
processors:
  routing:
    from_attribute: tenant.id
    attribute_source: resource
    table:
      - value: tenant-a
        exporters: [otlp/tenant-a]
      - value: tenant-b
        exporters: [otlp/tenant-b]
    default_exporters: [otlp/default]

exporters:
  otlp/tenant-a:
    endpoint: tenant-a.backend:4317
  otlp/tenant-b:
    endpoint: tenant-b.backend:4317
  otlp/default:
    endpoint: default.backend:4317
```

### Scenario 3: Development vs Production

```yaml
# Use environment variable to switch configs
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  # Production: aggressive filtering
  filter/production:
    traces:
      span:
        - 'severity_number < 13'  # Drop below WARN

  # Development: keep everything
  filter/development:
    # No filtering

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors:
        - ${FILTER_PROCESSOR}  # Set via env var
        - batch
      exporters: [otlp]
```

### Scenario 4: Security/Compliance

```yaml
# Goal: Remove sensitive data
processors:
  attributes/redact:
    actions:
      # Remove sensitive attributes
      - key: http.request.header.authorization
        action: delete
      - key: db.statement
        action: delete
      - key: user.email
        action: hash

      # Truncate potentially large values
      - key: http.request.body
        action: delete
      - key: exception.stacktrace
        action: truncate
        max_length: 1000
```

## Health and Monitoring

### Extensions

```yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133

  pprof:
    endpoint: 0.0.0.0:1777

  zpages:
    endpoint: 0.0.0.0:55679

service:
  extensions: [health_check, pprof, zpages]
```

### Collector Metrics

```yaml
# Export collector's own metrics
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: otel-collector
          scrape_interval: 10s
          static_configs:
            - targets: [localhost:8888]

service:
  telemetry:
    metrics:
      address: 0.0.0.0:8888
```

### Key Metrics to Monitor

```yaml
monitor:
  # Throughput
  - otelcol_receiver_accepted_spans
  - otelcol_receiver_refused_spans
  - otelcol_exporter_sent_spans
  - otelcol_exporter_send_failed_spans

  # Queue health
  - otelcol_processor_batch_batch_size_trigger_send
  - otelcol_processor_batch_timeout_trigger_send

  # Memory
  - otelcol_process_memory_rss
  - otelcol_processor_memory_limiter_decision
```

## Production Checklist

```yaml
checklist:
  - [ ] Memory limiter configured
  - [ ] Batch processor tuned
  - [ ] Health check enabled
  - [ ] Authentication configured
  - [ ] TLS enabled for exporters
  - [ ] Resource limits set (K8s)
  - [ ] Horizontal scaling configured
  - [ ] Monitoring/alerting on collector metrics
  - [ ] Graceful shutdown handling
  - [ ] Retry policies configured
```

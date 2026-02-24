---
name: '@otel-telemetry'
description: Expert guidance for emitting high-quality, cost-efficient OpenTelemetry telemetry. Use when instrumenting applications with traces, metrics, or logs. Triggers on requests for observability, telemetry, tracing, metrics collection, logging integration, or OTel setup. Critical skill that asks clarifying questions to understand exact tracking requirements.
license: MIT
metadata:
  author: mthines
  version: '1.0.0'
  workflow_type: 'advisory'
  signals:
    - traces
    - metrics
    - logs
---

# OpenTelemetry Telemetry Guide

This skill provides expert guidance for implementing high-quality, cost-efficient OpenTelemetry telemetry.

## Official Documentation

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Signals Overview](https://opentelemetry.io/docs/concepts/signals/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
- [Collector Documentation](https://opentelemetry.io/docs/collector/)
- [Sampling Strategies](https://opentelemetry.io/docs/concepts/sampling/)

## Phase 0: Requirements Gathering

**CRITICAL: Always ask clarifying questions before providing guidance.**

Before recommending any telemetry approach, gather requirements:

### Questions to Ask

```markdown
Before I help with your telemetry setup, I need to understand your requirements:

**What are you trying to observe?**
- [ ] Request latency and errors (API performance)
- [ ] Business events (orders, signups, conversions)
- [ ] Resource utilization (CPU, memory, connections)
- [ ] User journeys (click paths, feature usage)
- [ ] Dependencies (database, cache, external APIs)
- [ ] Other: ___

**What's your current stack?**
- Runtime: (Node.js, Python, Go, Java, .NET)?
- Framework: (Express, FastAPI, Spring Boot, etc.)?
- Existing logging: (pino, winston, log4j, etc.)?
- Current observability: (none, basic logging, existing APM)?

**What's your observability backend?**
- [ ] Dash0
- [ ] Grafana Cloud (Tempo, Loki, Mimir)
- [ ] Datadog
- [ ] New Relic
- [ ] Honeycomb
- [ ] Self-hosted (Prometheus, Elasticsearch)
- [ ] Not decided yet

**What are your constraints?**
- Cost budget per month?
- Cardinality limits from your backend?
- Retention requirements (7 days, 30 days, 1 year)?
- Compliance requirements (PII handling)?

**Do you have SLOs defined?**
- What are your availability targets?
- What latency percentiles matter (p50, p95, p99)?
- Which metrics feed your SLOs? (These should never be sampled)
```

### Why This Matters

- **Wrong signal choice** wastes money and misses insights
- **Over-instrumentation** creates noise and cost overruns
- **Under-instrumentation** leaves blind spots
- **Sampling without SLO awareness** breaks alerting

## Cross-Cutting Principles

### 1. Signal Density Over Volume

Every telemetry item should serve one of three purposes:
- **Detect** - Help identify that something is wrong
- **Localize** - Help pinpoint where the problem is
- **Explain** - Help understand why it happened

If it doesn't serve one of these purposes, don't emit it.

### 2. Push Reduction Early

Apply reduction at each layer, progressively:

```
SDK (sampling)  →  Collector (filtering)  →  Backend (retention)
     ↓                    ↓                       ↓
  Cheapest           Centralized              Most flexible
  Most efficient     Policy enforcement       Most expensive
```

**Rule**: Always reduce as early as possible in the pipeline.

### 3. SLO-Aware Policies

Telemetry feeding SLOs has special requirements:
- **Never sample** metrics used in SLO calculations
- **Always capture** error traces for SLO violations
- **Prioritize** data needed for incident response

### 4. Environment-Specific Treatment

| Environment | Traces | Metrics | Logs |
|-------------|--------|---------|------|
| Production | 1-10% sample, 100% errors | Full resolution | WARN+ only |
| Staging | 50% sample | Full resolution | INFO+ |
| Development | 100% (local) | Full resolution | DEBUG+ |

### 5. Cost as a Constraint

Treat your telemetry budget like any other resource constraint:
1. Define your monthly budget
2. Calculate current/projected costs
3. Apply sampling until costs fit budget
4. Monitor and adjust

## Signal Selection Guide

### Decision Tree

```
What do you need to know?
│
├─► "Why was this specific request slow?"
│   └─► Use TRACES (distributed tracing)
│
├─► "What's our error rate over the last hour?"
│   └─► Use METRICS (counters, histograms)
│
├─► "What happened at 3:47 AM?"
│   └─► Use LOGS (structured events)
│
└─► "All of the above"
    └─► Use ALL THREE with correlation
```

### Signal Characteristics

| Signal | Best For | Cardinality | Cost Driver |
|--------|----------|-------------|-------------|
| Traces | Request flow, latency breakdown | High (per-request) | Span count, attributes |
| Metrics | Aggregates, trends, alerts | Low (pre-aggregated) | Label cardinality |
| Logs | Events, debugging, audit | Medium | Volume, retention |

### Correlation Strategy

Always correlate signals using trace context:

```
Trace ID: abc123
    │
    ├─► Spans: Show request flow
    ├─► Metrics: Tagged with trace.id for exemplars
    └─► Logs: Include trace_id and span_id fields
```

## Sampling Strategy Design

### Head Sampling (SDK-side)

Apply at the SDK before data leaves your application:

```yaml
# Recommended head sampling rates
production:
  default: 0.01 to 0.10  # 1-10% of traces
  errors: 1.0            # 100% of errors (never sample)
  slow_requests: 1.0     # 100% over latency threshold

staging:
  default: 0.50          # 50% of traces

development:
  default: 1.0           # 100% (but use local collector)
```

### Tail Sampling (Collector-side)

Apply at the Collector after seeing complete traces:

```yaml
# Tail sampling policies (applied in order)
policies:
  - name: errors-policy
    type: status_code
    status_code: {status_codes: [ERROR]}
    # Decision: ALWAYS KEEP errors

  - name: slow-traces-policy
    type: latency
    latency: {threshold_ms: 1000}
    # Decision: KEEP traces > 1 second

  - name: probabilistic-policy
    type: probabilistic
    probabilistic: {sampling_percentage: 5}
    # Decision: KEEP 5% of remaining
```

### What to Never Sample

- Error traces (status = ERROR)
- Traces feeding SLO calculations
- Security-relevant events (auth failures)
- Business-critical transactions (payments, orders)

### What to Always Drop

- Health check endpoints (`/health`, `/ready`, `/live`)
- Kubernetes probes
- Synthetic monitoring (unless specifically needed)
- Internal load balancer checks

## Cardinality Management

### The Cardinality Problem

Cardinality = number of unique time series for metrics.

```
metric{method="GET", path="/users", status="200"}  # 1 series
metric{method="GET", path="/users/123", status="200"}  # +1 series
metric{method="GET", path="/users/456", status="200"}  # +1 series
# User IDs in paths = unbounded cardinality = cost explosion
```

### Cardinality Controls

**1. Attribute Whitelisting**

Only allow known, bounded attributes:

```yaml
# Good - bounded cardinality
allowed_attributes:
  - http.method      # ~10 values
  - http.status_code # ~50 values
  - service.name     # Known services
  - environment      # prod, staging, dev

# Bad - unbounded cardinality
forbidden_attributes:
  - user.id          # Millions of values
  - request.id       # Unique per request
  - http.url         # Contains query params
```

**2. Path Normalization**

```
Before: /users/123/orders/456
After:  /users/{userId}/orders/{orderId}
```

**3. Label Value Limits**

Set maximum unique values per label:
- If exceeded, bucket into "other"
- Alert on approaching limits

### Cardinality Budget

Calculate your cardinality budget:

```
Total series =
  (# metrics) ×
  (# label combinations) ×
  (# services) ×
  (# instances)

Example:
  20 metrics ×
  100 label combos ×
  10 services ×
  5 instances =
  100,000 series
```

## Verification & Testing

### Verify Telemetry Is Working

1. **Check SDK initialization**
   ```bash
   # Look for OTel startup logs
   grep -i "opentelemetry\|otel" application.log
   ```

2. **Verify exporter connection**
   ```bash
   # Check OTLP endpoint is reachable
   curl -v https://your-collector:4318/v1/traces
   ```

3. **Generate test traffic**
   - Make requests to your service
   - Check traces appear in backend within 30 seconds

4. **Verify sampling**
   - Send 100 requests
   - Confirm expected number appear (based on sample rate)

### Common Pitfalls

| Problem | Symptom | Solution |
|---------|---------|----------|
| Missing traces | No data in backend | Check exporter config, network |
| Missing spans | Incomplete traces | Check context propagation |
| High costs | Unexpected bills | Review sampling, cardinality |
| Missing errors | Errors not captured | Check error recording code |
| Duplicate spans | Same span twice | Check instrumentation overlap |

### Debugging Missing Data

```
Checklist:
[ ] SDK initialized before application code runs
[ ] Correct OTLP endpoint configured
[ ] Authentication token/header set
[ ] Network allows outbound to collector
[ ] Batch processor not holding data (flush on shutdown)
[ ] Sampling not dropping everything
[ ] Resource attributes set (service.name required)
```

## Reference Documents

For detailed guidance on each signal type:

- [Telemetry Quality](./references/telemetry-quality.md) - Cardinality, attribute hygiene, signal density
- [Spans Reference](./references/signals/spans.md) - Traces and spans best practices
- [Metrics Reference](./references/signals/metrics.md) - Metrics instrumentation guide
- [Logs Reference](./references/signals/logs.md) - Structured logging with OTel
- [Collector Reference](./references/collector.md) - Collector configuration
- [Setup Reference](./references/setup.md) - SDK setup and backend integration

## Examples

- [Cost Optimization Scenarios](./examples/cost-optimization.md) - Real-world cost reduction strategies

---
title: "Telemetry: Spans, Metrics, Logs"
impact: CRITICAL
tags:
  - telemetry
  - spans
  - metrics
  - logs
  - traces
  - cardinality
---

# Telemetry Reference

## Spans

### When to Create Custom Spans

- **Use auto-instrumentation** for HTTP, databases, frameworks
- **Create custom spans** for business logic, custom protocols

### Naming

```
Good: order.process, payment.authorize, cache.get
Bad:  handleRequest, doStuff, /api/users/123
```

### Basic Pattern

```javascript
tracer.startActiveSpan("order.process", async (span) => {
  try {
    span.setAttribute("order.id", order.id);
    const result = await processOrder(order);
    return result;
  } catch (error) {
    span.recordException(error);
    span.setStatus({ code: SpanStatusCode.ERROR });
    throw error;
  } finally {
    span.end();
  }
});
```

### Status Codes

| Code | When |
|------|------|
| UNSET | Default, operation completed |
| ERROR | Operation failed |

### Events vs Logs

| Use Events | Use Logs |
|------------|----------|
| Tied to span operation | Independent events |
| Cache hit/miss, retries | Audit logs, debug output |

---

## Metrics

### Instrument Types

| Type | Use For | Example |
|------|---------|---------|
| Counter | Monotonic increase | Requests, errors |
| UpDownCounter | Can decrease | Active connections |
| Histogram | Distributions | Latency, sizes |
| Gauge | Point-in-time | Memory, CPU |

### Golden Signals

```javascript
// Latency
const duration = meter.createHistogram("http.server.duration", { unit: "ms" });

// Traffic
const requests = meter.createCounter("http.server.requests");

// Errors
const errors = meter.createCounter("http.server.errors");

// Saturation
meter.createObservableGauge("system.cpu.utilization", (result) => {
  result.observe(getCpuUtilization());
});
```

### Naming

```
http.server.requests      # Counter
http.server.duration      # Histogram
db.connections.active     # UpDownCounter
```

### Cardinality

**The #1 cost driver.** Before adding attributes, calculate:

```
method:    5 values
route:     50 values (normalized)
status:    5 values (bucketed)
instances: 10

Total: 5 × 50 × 5 × 10 = 12,500 series ✓
```

| Series | Zone | Action |
|--------|------|--------|
| < 10K | Ideal | Proceed |
| 10K-100K | Caution | Review attributes |
| > 100K | Danger | Remove unbounded attributes |

**Never use on metrics:**
- `user.id`, `request.id`, `order.id`
- `url.full` (has query params)
- `timestamp`, `ip.address`

### Normalization

```javascript
// URLs: /users/123 → /users/{id}
path.replace(/\/\d+/g, "/{id}")

// Status codes: 200 → "2xx"
if (code >= 200 && code < 300) return "2xx";
if (code >= 400 && code < 500) return "4xx";
if (code >= 500) return "5xx";
```

---

## Logs

### Structure

```javascript
// BAD: unstructured
logger.info(`User ${userId} placed order ${orderId}`);

// GOOD: structured
logger.info("order.placed", {
  user_id: userId,
  order_id: orderId,
  amount: amount
});
```

### Levels by Environment

| Level | Production | Development |
|-------|------------|-------------|
| ERROR | Always | Always |
| WARN | Always | Always |
| INFO | Sample 10% | Always |
| DEBUG | Never | Always |

### Trace Correlation

```javascript
import { trace, context } from "@opentelemetry/api";

function getTraceContext() {
  const span = trace.getSpan(context.active());
  if (!span) return {};
  const ctx = span.spanContext();
  return { trace_id: ctx.traceId, span_id: ctx.spanId };
}

logger.info("order.placed", { ...getTraceContext(), order_id: orderId });
```

---

## Attribute Placement

| Signal | Cardinality Tolerance | OK | Avoid |
|--------|----------------------|----|----|
| Metrics | Very low | method, route, status_bucket | user.id |
| Spans | Medium | + user.id, order.id | request.body |
| Logs | Higher | + request.id | secrets |

---

## Anti-Patterns

### Spans in Loops

```javascript
// BAD: 10,000 items = 10,000 spans
items.forEach(item => {
  tracer.startActiveSpan("process.item", span => { process(item); span.end(); });
});

// GOOD: single span
tracer.startActiveSpan("process.batch", span => {
  span.setAttribute("batch.size", items.length);
  items.forEach(process);
  span.end();
});
```

### Unbounded Metric Labels

```javascript
// BAD: millions of series
counter.add(1, { user_id: userId });

// GOOD: bounded
counter.add(1, { user_tier: "premium" });
```

### Unstructured Logs

```javascript
// BAD
logger.error(`Failed: ${error.message}`);

// GOOD
logger.error("order.failed", {
  error_type: error.name,
  error_message: error.message,
  order_id: orderId
});
```

---

## Resources

- [Traces](https://opentelemetry.io/docs/concepts/signals/traces/)
- [Metrics](https://opentelemetry.io/docs/concepts/signals/metrics/)
- [Logs](https://opentelemetry.io/docs/concepts/signals/logs/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

---
title: "Signals: Spans, Metrics, Logs"
impact: CRITICAL
tags:
  - spans
  - metrics
  - logs
  - traces
---

# Signals Reference

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

### Cardinality Rules

```javascript
// GOOD: bounded labels
counter.add(1, { method: "GET", route: "/users/{id}", status: "2xx" });

// BAD: unbounded labels
counter.add(1, { user_id: userId });  // Millions of values!
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

### Severity Mapping

| Logger | OTel |
|--------|------|
| debug | DEBUG (5) |
| info | INFO (9) |
| warn | WARN (13) |
| error | ERROR (17) |

---

## Common Anti-Patterns

### Spans in Loops

```javascript
// BAD
items.forEach(item => {
  tracer.startActiveSpan("process.item", span => { process(item); span.end(); });
});

// GOOD
tracer.startActiveSpan("process.batch", span => {
  span.setAttribute("batch.size", items.length);
  items.forEach(process);
  span.end();
});
```

### Unbounded Metric Labels

```javascript
// BAD
counter.add(1, { user_id: userId, timestamp: Date.now() });

// GOOD
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

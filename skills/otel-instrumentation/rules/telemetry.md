---
title: 'Telemetry: Spans, Metrics, Logs'
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

## Overview

Telemetry consists of three core signal types: **Metrics**, **Traces**, and **Logs**. Each serves a distinct purpose in understanding system behavior.

| Signal  | When to Use                             |
| ------- |-----------------------------------------|
| Metrics | Alerting, dashboards, trends            |
| Traces  | Latency analysis, distributed debugging |
| Logs    | Audit trails                            |

**Symptom-to-cause workflow:** Metrics surface problems → Traces pinpoint location → Logs explain causation.

**Correlation is essential.** Link signals through shared context (trace IDs, span IDs) so you can navigate from an alert to the exact log line that explains the failure.

---

## Signals

### Metrics

Metrics are time-stamped numerical measurements, aggregated over time.

#### Instrument Types

| Type          | Use For            | Example            |
| ------------- | ------------------ | ------------------ |
| Counter       | Monotonic increase | Requests, errors   |
| UpDownCounter | Can decrease       | Active connections |
| Histogram     | Distributions      | Latency, sizes     |
| Gauge         | Point-in-time      | Memory, CPU        |

#### Golden Signals

```javascript
// Latency
const duration = meter.createHistogram('http.server.duration', { unit: 'ms' });

// Traffic
const requests = meter.createCounter('http.server.requests');

// Errors
const errors = meter.createCounter('http.server.errors');

// Saturation
meter.createObservableGauge('system.cpu.utilization', (result) => {
  result.observe(getCpuUtilization());
});
```

#### Naming

```
http.server.requests      # Counter
http.server.duration      # Histogram
db.connections.active     # UpDownCounter
```

---

### Spans/Traces

Traces are hierarchical records of a request's journey across services. Each span represents a unit of work.

#### When to Create Custom Spans

- **Use auto-instrumentation** for HTTP, databases, frameworks
- **Create custom spans** for business logic, custom protocols

#### Naming

```
Good: order.process, payment.authorize, cache.get
Bad:  handleRequest, doStuff, /api/users/123
```

#### Basic Pattern

```javascript
tracer.startActiveSpan('order.process', async (span) => {
  try {
    span.setAttribute('order.id', order.id);
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

#### Status Codes

| Code  | When                         |
| ----- | ---------------------------- |
| UNSET | Default, operation completed |
| ERROR | Operation failed             |

#### Events vs Logs

| Use Events              | Use Logs                 |
| ----------------------- | ------------------------ |
| Tied to span operation  | Independent events       |
| Cache hit/miss, retries | Audit logs, debug output |

---

### Logs

Logs are textual records of discrete events with local context.

#### Structured Logging

```javascript
// BAD: unstructured
logger.info(`User ${userId} placed order ${orderId}`);

// GOOD: structured
logger.info('order.placed', {
  user_id: userId,
  order_id: orderId,
  amount: amount,
});
```

#### Levels by Environment

| Level | Production | Development |
| ----- | ---------- | ----------- |
| ERROR | Always     | Always      |
| WARN  | Always     | Always      |
| INFO  | Sample 10% | Always      |
| DEBUG | Never      | Always      |

#### Trace Correlation

Inject trace context into logs to navigate from traces to related log entries:

```javascript
import { trace, context } from '@opentelemetry/api';

function getTraceContext() {
  const span = trace.getSpan(context.active());
  if (!span) return {};
  const ctx = span.spanContext();
  return { trace_id: ctx.traceId, span_id: ctx.spanId };
}

logger.info('order.placed', { ...getTraceContext(), order_id: orderId });
```

---

## Telemetry Quality

### Cardinality Management

**The #1 cost driver.** Cardinality is the number of unique time series created by your metrics. Each new attribute multiplies your current total series count by its number of unique values. E.g. an additional attribute with 10 values turns 12,500 series into 125,000 instantly. Before adding attributes, calculate:

An example of a metric with 4 attributes:

```
method:    5 values
route:     50 values (normalized)
status:    5 values (bucketed)
instances: 10
```

Total: 5 × 50 × 5 × 10 = <ins>**12,500 series**</ins> ✓

| Series Count     | Zone       | Action                                  |
| ---------------- | ---------- | --------------------------------------- |
| < 1,000          | Minimal    | Room to add more dimensions             |
| 1,000 - 10,000   | ✓ Ideal    | Good balance of detail vs cost          |
| 10,000 - 50,000  | Acceptable | Monitor growth, review monthly          |
| 50,000 - 100,000 | Caution    | Review attributes, consider sampling    |
| > 100,000        | Danger     | Remove unbounded attributes immediately |
| > 1,000,000      | Critical   | Backend instability, massive costs      |

**Never use on metrics:**

- `user.id`, `request.id`, `order.id`
- `url.full` (has query params)
- `timestamp`, `ip.address`

Why? Each unique value creates a new time series, which can quickly lead to millions of series and skyrocketing costs.

### Normalization

Normalize high-cardinality values before using them as attributes:

```javascript
// URLs: /users/123 → /users/{id}
path.replace(/\/\d+/g, '/{id}');

// Status codes: 200 → "2xx"
if (code >= 200 && code < 300) return '2xx';
if (code >= 400 && code < 500) return '4xx';
if (code >= 500) return '5xx';
```

### Attribute Placement

Different signals tolerate different cardinality levels:

| Signal  | Cardinality Tolerance | OK                           | Avoid        |
| ------- | --------------------- | ---------------------------- | ------------ |
| Metrics | Very low              | method, route, status_bucket | user.id      |
| Spans   | Medium                | + user.id, order.id          | request.body |
| Logs    | Higher                | + request.id                 | secrets      |

### Semantic Conventions

Semantic conventions are standardized attribute names that ensure consistency across services, teams, and tools. Using them enables:

- **Interoperability** - Dashboards and queries work across services
- **Correlation** - Backends can link related signals automatically
- **Tooling** - Observability platforms recognize standard attributes

#### Common Namespaces

| Namespace     | Use For                        | Example Attributes                                                           |
| ------------- | ------------------------------ | ---------------------------------------------------------------------------- |
| `http.*`      | HTTP requests/responses        | `http.request.method`, `http.response.status_code`, `http.route`             |
| `db.*`        | Database operations            | `db.system`, `db.operation.name`, `db.collection.name`                       |
| `messaging.*` | Message queues                 | `messaging.system`, `messaging.operation.type`, `messaging.destination.name` |
| `rpc.*`       | Remote procedure calls         | `rpc.system`, `rpc.method`, `rpc.service`                                    |
| `error.*`     | Error details                  | `error.type`, `error.message`                                                |
| `user.*`      | User context (spans/logs only) | `user.id`, `user.email`                                                      |

#### Usage

```javascript
// HTTP span attributes (auto-instrumented, but useful to know)
span.setAttributes({
  'http.request.method': 'POST',
  'http.route': '/api/orders/{id}',
  'http.response.status_code': 201,
});

// Database span attributes
span.setAttributes({
  'db.system': 'postgresql',
  'db.operation.name': 'SELECT',
  'db.collection.name': 'orders',
});

// Custom business attributes (use your own namespace)
span.setAttributes({
  'order.id': orderId,
  'order.total': total,
  'order.item_count': items.length,
});
```

#### Rules

1. **Prefer standard conventions** over custom names when they exist
2. **Use namespaces** for custom attributes (`app.feature.*`, `order.*`)
3. **Check the spec** before inventing new attributes—it's likely already defined
4. **Don't mix conventions** (e.g., `httpMethod` vs `http.request.method`)

Read more at [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/) for a comprehensive list of standard attributes and best practices.

---

## Anti-Patterns

### Spans in Loops

```javascript
// BAD: 10,000 items = 10,000 spans
items.forEach((item) => {
  tracer.startActiveSpan('process.item', (span) => {
    process(item);
    span.end();
  });
});

// GOOD: single span
tracer.startActiveSpan('process.batch', (span) => {
  span.setAttribute('batch.size', items.length);
  items.forEach(process);
  span.end();
});
```

### Unbounded Metric Labels

```javascript
// BAD: millions of series
counter.add(1, { user_id: userId });

// GOOD: bounded
counter.add(1, { user_tier: 'premium' });
```

### Unstructured Logs

```javascript
// BAD
logger.error(`Failed: ${error.message}`);

// GOOD
logger.error('order.failed', {
  error_type: error.name,
  error_message: error.message,
  order_id: orderId,
});
```

### Missing Trace Correlation

```javascript
// BAD: logs without context
logger.info('Payment processed');

// GOOD: logs with trace context
logger.info('payment.processed', {
  ...getTraceContext(),
  payment_id: paymentId,
});
```

---

## Resources

- [Traces](https://opentelemetry.io/docs/concepts/signals/traces/)
- [Metrics](https://opentelemetry.io/docs/concepts/signals/metrics/)
- [Logs](https://opentelemetry.io/docs/concepts/signals/logs/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

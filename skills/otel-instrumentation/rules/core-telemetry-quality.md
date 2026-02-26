---
title: "Telemetry Quality and Cardinality"
impact: CRITICAL
tags:
  - cardinality
  - attributes
  - quality
  - cost
  - optimization
---

# Telemetry Quality Reference

## The Quality Hierarchy

Follow this order when instrumenting:

| Level | Focus | Key Question |
|-------|-------|--------------|
| 1. **Correctness** | Data accuracy, timestamps, context propagation | "Is the data right?" |
| 2. **Actionability** | Every signal helps detect, localize, or explain | "Will this help diagnose issues?" |
| 3. **Efficiency** | Bounded cardinality, normalized attributes | "Is this sustainable?" |
| 4. **Cost** | Sampling, retention, query performance | "Can we afford this at scale?" |

**The golden rule**: For every piece of telemetry, ask: "When did we last look at this?" If never, remove it.

---

## 5-Minute Cardinality Check

Run this before deploying new instrumentation:

### Step 1: List Your Metric Attributes

```
metric: http.server.duration
attributes: [method, route, status_code]
```

### Step 2: Count Maximum Combinations

```
method:      5 values (GET, POST, PUT, DELETE, PATCH)
route:       50 values (your API endpoints, normalized)
status_code: 5 values (bucket to 2xx, 3xx, 4xx, 5xx, other)
instances:   10 (pods/containers)

Total: 5 × 50 × 5 × 10 = 12,500 series ✓ Ideal Zone
```

### Cardinality Budget Zones

```
Series Count    │ Zone        │ Action
────────────────┼─────────────┼─────────────────────────────────
< 1,000         │ Minimal     │ Room to add more dimensions
1,000 - 10,000  │ ✓ Ideal     │ Good balance of detail vs cost
10,000 - 50,000 │ Acceptable  │ Monitor growth, review monthly
50,000 - 100,000│ Caution     │ Review attributes, consider sampling
> 100,000       │ Danger      │ Remove unbounded attributes immediately
> 1,000,000     │ Critical    │ Backend instability, massive costs
```

**Target the Ideal Zone (1K-10K series per metric)** - enough dimensions for debugging without exploding costs.

### Step 3: Check for Unbounded Attributes

**Red flags** - these will explode:
- `user.id` → millions of users
- `request.id` → unique per request
- `url.full` → includes query params
- `timestamp` → infinite values
- `ip.address` → millions of IPs

**Safe** - bounded by design:
- `http.method` → ~10 values
- `http.route` → known endpoints
- `error.type` → categorized errors
- `customer.tier` → few tiers

### Step 4: Apply the Budget

| Cardinality | Impact | Action |
|-------------|--------|--------|
| < 10K series | Safe | Proceed |
| 10K-100K | Caution | Review attributes |
| > 100K | Danger | Remove unbounded attributes |

---

## Official Documentation

- [OpenTelemetry Metrics Data Model](https://opentelemetry.io/docs/specs/otel/metrics/data-model/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

---

## Attribute Placement Guide

| Signal | Cardinality Tolerance | Safe Attributes | Avoid |
|--------|----------------------|-----------------|-------|
| **Metrics** | Very low | method, route, status_bucket, error_type | user.id, request.id |
| **Spans** | Medium | All metric attributes + user.id, order.id | request.body, full URLs |
| **Logs** | Higher | All span attributes + request.id | Secrets, credentials |

### Where to Put What

```
RESOURCE (set once at startup)
  service.name, service.version, deployment.environment.name

SPAN (per-operation)
  http.method, http.route, user.id, error.type

METRIC (bounded only!)
  http.method, http.route, status_bucket
  NO: user.id, request.id, timestamps
```

---

## Cardinality Management

### Strategy 1: Normalize URLs

```javascript
function normalizePath(path) {
  return path
    .replace(/\/\d+/g, "/{id}")           // /users/123 → /users/{id}
    .replace(/\/[a-f0-9-]{36}/g, "/{uuid}") // UUIDs
    .split("?")[0];                         // Strip query params
}
```

### Strategy 2: Bucket Status Codes

```javascript
function bucketStatus(code) {
  if (code >= 200 && code < 300) return "2xx";
  if (code >= 400 && code < 500) return "4xx";
  if (code >= 500) return "5xx";
  return "other";
}
```

### Strategy 3: Categorize Errors

```javascript
const errorCategories = {
  ECONNREFUSED: "connection_error",
  ETIMEDOUT: "timeout_error",
  ValidationError: "validation_error",
  AuthenticationError: "auth_error"
};

function categorizeError(error) {
  return errorCategories[error.name] || errorCategories[error.code] || "unknown";
}
```

---

## Quality Anti-Patterns

### Don't: Metric per Entity

```javascript
// BAD - infinite metrics
meter.createCounter(`orders.${orderId}.processed`);

// GOOD - attributes for dimensions
ordersProcessed.add(1, { order_type: order.type });
```

### Don't: Span per Iteration

```javascript
// BAD - 10,000 items = 10,000 spans
for (const item of items) {
  tracer.startActiveSpan("process.item", span => {
    processItem(item);
    span.end();
  });
}

// GOOD - single span with batch size
tracer.startActiveSpan("process.batch", span => {
  span.setAttribute("batch.size", items.length);
  items.forEach(processItem);
  span.end();
});
```

### Don't: Timestamps as Attributes

```javascript
// BAD - infinite cardinality
span.setAttribute("started_at", Date.now());

// GOOD - use built-in timing
// Spans have automatic start/end times
```

### Don't: Full Request/Response Bodies

```javascript
// BAD - size issues, PII risk
span.setAttribute("request.body", JSON.stringify(req.body));

// GOOD - structure, not content
span.setAttribute("request.content_type", req.contentType);
span.setAttribute("request.body_size", JSON.stringify(req.body).length);
```

---

## Semantic Conventions

Use standard attribute names:

```yaml
# HTTP
http.request.method: GET          # Not: method, http_method
http.response.status_code: 200    # Not: status, statusCode
url.path: /api/users              # Not: path, http.path

# Database
db.system: postgresql             # Not: database, db_type
db.operation.name: SELECT         # Not: query_type

# Errors
error.type: ValidationError       # Not: error_class
exception.message: "Invalid..."   # Not: error_message
```

---

## Quality Checklist

Before deploying new telemetry:

- [ ] Cardinality calculated and < 100K series
- [ ] All attributes bounded or normalized
- [ ] Semantic conventions used
- [ ] No PII in attributes
- [ ] Sampling strategy defined
- [ ] Dashboard or alert will use this data

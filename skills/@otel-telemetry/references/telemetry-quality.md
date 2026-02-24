# Telemetry Quality Reference

Comprehensive guide to emitting high-quality, actionable telemetry that serves your observability goals without breaking the bank.

## Official Documentation

- [OpenTelemetry Metrics Data Model](https://opentelemetry.io/docs/specs/otel/metrics/data-model/)
- [Attribute Specification](https://opentelemetry.io/docs/specs/otel/common/#attribute)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

## What is Telemetry Quality?

High-quality telemetry has three characteristics:

```
┌─────────────────────────────────────────────────────────────────┐
│                    HIGH-QUALITY TELEMETRY                       │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │  ACTIONABLE │  │  EFFICIENT  │  │ SUSTAINABLE │             │
│  │             │  │             │  │             │             │
│  │ Helps you   │  │ Minimal     │  │ Costs stay  │             │
│  │ detect,     │  │ overhead,   │  │ predictable │             │
│  │ localize,   │  │ bounded     │  │ as you      │             │
│  │ explain     │  │ cardinality │  │ scale       │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
└─────────────────────────────────────────────────────────────────┘
```

**The quality test:** For every piece of telemetry, ask:
1. Will this help me detect a problem?
2. Will this help me localize a problem?
3. Will this help me explain why a problem occurred?

If the answer to all three is "no," don't emit it.

---

## Understanding Cardinality

### What is Cardinality?

Cardinality = the number of unique combinations of label/attribute values.

```
Simple example:
  metric{method="GET"}                    → 1 series
  metric{method="GET", status="200"}      → 1 series
  metric{method="POST", status="200"}     → 1 series (total: 2)
  metric{method="GET", status="404"}      → 1 series (total: 3)

With 5 methods × 10 status codes = 50 series (manageable)
```

### The Cardinality Problem

```
Dangerous example:
  metric{method="GET", user_id="user_1"}   → 1 series
  metric{method="GET", user_id="user_2"}   → 1 series
  ...
  metric{method="GET", user_id="user_1000000"}  → 1,000,000 series!

With 5 methods × 1,000,000 users = 5,000,000 series = $$$$$
```

### Why Cardinality Matters

| Cardinality | Impact |
|-------------|--------|
| Low (< 10K series) | Fast queries, low cost |
| Medium (10K-100K) | Acceptable for most backends |
| High (100K-1M) | Slow queries, high cost |
| Explosive (> 1M) | Backend failures, massive bills |

### Cardinality Sources

```
┌─────────────────────────────────────────────────────────────────┐
│                  CARDINALITY SOURCES                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  BOUNDED (Safe)              UNBOUNDED (Dangerous)              │
│  ─────────────               ──────────────────                 │
│  http.method (~10)           user.id (millions)                 │
│  http.status_code (~50)      request.id (unique)                │
│  service.name (known)        session.id (millions)              │
│  environment (3-5)           order.id (unbounded)               │
│  region (< 20)               timestamp (infinite)               │
│  error.type (< 100)          ip.address (millions)              │
│                              url.full (infinite with params)    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Cardinality Management Strategies

### Strategy 1: Attribute Whitelisting

Only allow known, bounded attributes:

```yaml
# Define allowed attributes per signal
traces:
  allowed_attributes:
    - http.method
    - http.status_code
    - http.route          # Normalized, not http.url
    - db.system
    - db.operation
    - error.type
    - service.name
    - deployment.environment

metrics:
  # More restrictive for metrics (cardinality hits hardest here)
  allowed_attributes:
    - http.method
    - http.route
    - status_bucket       # "2xx", "4xx", "5xx" not exact codes
    - service.name

logs:
  # Can be more permissive (logs are sampled differently)
  allowed_attributes:
    - All trace attributes
    - user.id             # OK in logs, dangerous in metrics
    - request.id          # OK in logs for correlation
```

### Strategy 2: Value Normalization

Transform unbounded values into bounded ones:

```javascript
// URL Path Normalization
function normalizePath(path) {
  return path
    // IDs become placeholders
    .replace(/\/\d+/g, '/{id}')
    // UUIDs become placeholders
    .replace(/\/[a-f0-9-]{36}/g, '/{uuid}')
    // Tokens become placeholders
    .replace(/\/[a-zA-Z0-9]{20,}/g, '/{token}')
    // Query params stripped
    .split('?')[0];
}

// Before: /users/12345/orders/abc-def-ghi
// After:  /users/{id}/orders/{uuid}
```

```javascript
// Status Code Bucketing
function bucketStatus(code) {
  if (code >= 200 && code < 300) return '2xx';
  if (code >= 300 && code < 400) return '3xx';
  if (code >= 400 && code < 500) return '4xx';
  if (code >= 500) return '5xx';
  return 'other';
}

// For important codes, keep them explicit
function bucketStatusSelective(code) {
  const keepExact = [200, 201, 400, 401, 403, 404, 500, 502, 503];
  return keepExact.includes(code) ? String(code) : bucketStatus(code);
}
```

```javascript
// Error Type Normalization
function normalizeErrorType(error) {
  // Map specific errors to categories
  const categories = {
    'ECONNREFUSED': 'connection_error',
    'ETIMEDOUT': 'timeout_error',
    'ENOTFOUND': 'dns_error',
    'ValidationError': 'validation_error',
    'AuthenticationError': 'auth_error',
    'AuthorizationError': 'auth_error',
  };

  return categories[error.name] || categories[error.code] || 'unknown_error';
}
```

### Strategy 3: Cardinality Limits

Set hard limits to prevent explosions:

```yaml
# Collector configuration
processors:
  # Limit unique attribute values
  attributes:
    actions:
      - key: http.route
        action: limit
        limit: 100  # Max 100 unique routes
        overflow_value: "other"

  # Limit total metric series
  filter/cardinality:
    metrics:
      datapoint:
        - 'metric.name == "http.server.duration" and count(attributes) > 1000'
```

```javascript
// SDK-side cardinality tracking
const seenValues = new Map();
const MAX_VALUES = 100;

function safeAttribute(key, value) {
  if (!seenValues.has(key)) {
    seenValues.set(key, new Set());
  }

  const values = seenValues.get(key);

  if (values.size >= MAX_VALUES && !values.has(value)) {
    return 'other';  // Bucket overflow
  }

  values.add(value);
  return value;
}
```

### Strategy 4: Cardinality Budgeting

Calculate your budget before deploying:

```javascript
function calculateCardinality(config) {
  let total = 0;

  for (const metric of config.metrics) {
    let combinations = 1;

    for (const label of metric.labels) {
      combinations *= label.maxValues;
    }

    // Multiply by number of instances
    combinations *= config.instances;

    total += combinations;

    console.log(`${metric.name}: ${combinations} series`);
  }

  return total;
}

// Example
const config = {
  instances: 10,
  metrics: [
    {
      name: 'http.server.duration',
      labels: [
        { name: 'method', maxValues: 5 },
        { name: 'route', maxValues: 50 },
        { name: 'status', maxValues: 5 }  // Bucketed
      ]
    },
    {
      name: 'db.query.duration',
      labels: [
        { name: 'operation', maxValues: 10 },
        { name: 'table', maxValues: 20 }
      ]
    }
  ]
};

console.log(`Total: ${calculateCardinality(config)} series`);
// http.server.duration: 12,500 series
// db.query.duration: 2,000 series
// Total: 14,500 series
```

---

## Attribute Hygiene

### The Attribute Quality Checklist

For every attribute you add, verify:

```
[ ] Is it bounded? (finite number of possible values)
[ ] Is it useful? (helps detect, localize, or explain)
[ ] Is it stable? (won't change meaning over time)
[ ] Is it standardized? (uses semantic conventions)
[ ] Is it safe? (no PII, secrets, or sensitive data)
```

### Good vs Bad Attributes

```yaml
# GOOD ATTRIBUTES
http.method: "GET"           # Bounded, standard, useful
http.route: "/users/{id}"    # Normalized, bounded
http.status_code: 200        # Bounded, standard
error.type: "validation"     # Categorized, bounded
customer.tier: "premium"     # Bounded business context
feature.flag: "new_checkout" # Bounded, useful for debugging
cache.hit: true              # Boolean, bounded

# BAD ATTRIBUTES - Don't use on metrics
user.id: "usr_abc123"        # Unbounded (millions of users)
request.id: "req_xyz789"     # Unique per request (infinite)
timestamp: "2024-01-15..."   # Infinite values
url.full: "/users?page=1"    # Includes query params (unbounded)
email: "user@example.com"    # PII risk
request.body: "{...}"        # Large, potentially sensitive
password: "..."              # NEVER log credentials
api_key: "..."               # NEVER log secrets
```

### Attribute Placement Guide

```
┌─────────────────────────────────────────────────────────────────┐
│                WHERE TO PUT ATTRIBUTES                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  RESOURCE ATTRIBUTES (set once at startup)                      │
│  ─────────────────────────────────────────                      │
│  service.name, service.version, deployment.environment          │
│  host.name, cloud.region, k8s.pod.name                          │
│                                                                 │
│  SPAN ATTRIBUTES (per-operation context)                        │
│  ─────────────────────────────────────────                      │
│  http.method, http.route, db.operation                          │
│  user.id (OK on spans), order.id (OK on spans)                  │
│  error.type, error.message                                      │
│                                                                 │
│  METRIC ATTRIBUTES (bounded only!)                              │
│  ─────────────────────────────────────────                      │
│  http.method, http.route, status_bucket                         │
│  service.name, error.type (categorized)                         │
│  NO user.id, request.id, or unbounded values                    │
│                                                                 │
│  LOG ATTRIBUTES (most permissive)                               │
│  ─────────────────────────────────────────                      │
│  All of the above + request.id, user.id                         │
│  Correlation IDs, detailed error info                           │
│  Still no PII without proper handling                           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Semantic Conventions

Always prefer standard attribute names:

```yaml
# HTTP (https://opentelemetry.io/docs/specs/semconv/http/)
http.request.method: GET          # Not: method, http_method
http.response.status_code: 200    # Not: status, statusCode
url.path: /api/users              # Not: path, http.path
url.scheme: https                 # Not: protocol, scheme

# Database (https://opentelemetry.io/docs/specs/semconv/database/)
db.system: postgresql             # Not: database, db_type
db.name: users                    # Not: database_name
db.operation: SELECT              # Not: query_type, operation

# Errors
error.type: ValidationError       # Not: error_class, exception_type
exception.message: "Invalid..."   # Not: error_message
exception.stacktrace: "..."       # Not: stack, stacktrace
```

---

## Signal Density

### The Density Principle

```
HIGH DENSITY (Good)                 LOW DENSITY (Bad)
─────────────────                   ─────────────────
Few signals, high value             Many signals, low value
Each helps diagnose issues          Most are noise
Focused on what matters             "Just in case" mentality
Scales sustainably                  Costs explode with growth
```

### Measuring Signal Density

```javascript
// Calculate your signal density score
function calculateDensity(telemetry) {
  const metrics = {
    traces: {
      total: telemetry.traces.count,
      usedInIncidents: telemetry.traces.usedInDiagnosis,
      // What % of traces helped diagnose an issue?
      density: telemetry.traces.usedInDiagnosis / telemetry.traces.count
    },
    metrics: {
      total: telemetry.metrics.series,
      inDashboards: telemetry.metrics.dashboarded,
      inAlerts: telemetry.metrics.alerted,
      // What % of metrics are actually used?
      density: (telemetry.metrics.dashboarded + telemetry.metrics.alerted)
               / telemetry.metrics.series
    }
  };

  return metrics;
}

// Target densities:
// Traces: > 10% should be referenced in incident diagnosis
// Metrics: > 50% should be in dashboards or alerts
// If lower, you're over-instrumenting
```

### Improving Density

```yaml
# Questions to ask for each metric/span:

1. "When did we last look at this?"
   - Never → Consider removing
   - Monthly → Keep but sample heavily
   - Weekly → Keep
   - Daily → Essential, never sample

2. "What decision would this inform?"
   - None → Remove
   - Capacity planning → Keep at low resolution
   - Incident response → Keep at high resolution

3. "Could we get this from another signal?"
   - Derive from existing data → Remove duplicate
   - Unique insight → Keep
```

---

## Quality Anti-Patterns

### Anti-Pattern 1: "Log Everything"

```javascript
// BAD: Logging every variable "just in case"
logger.debug('Processing started', {
  user, request, config, state, timestamp,
  sessionId, correlationId, parentId, ...everything
});

// GOOD: Log what you need for diagnosis
logger.info('order.processing.started', {
  order_id: order.id,
  customer_tier: order.customer.tier,
  items_count: order.items.length
});
```

### Anti-Pattern 2: "Metric Per Entity"

```javascript
// BAD: Creating metrics with entity IDs
meter.createCounter(`orders.${orderId}.processed`);  // Infinite metrics!
meter.createHistogram(`user.${userId}.latency`);     // Millions of histograms!

// GOOD: Use attributes for dimensions
ordersProcessed.add(1, {
  customer_tier: order.customer.tier,  // Bounded
  order_type: order.type               // Bounded
});
```

### Anti-Pattern 3: "Span Per Iteration"

```javascript
// BAD: Span explosion in loops
for (const item of items) {  // 10,000 items = 10,000 spans
  tracer.startActiveSpan('process.item', (span) => {
    processItem(item);
    span.end();
  });
}

// GOOD: Single span with events or metrics
tracer.startActiveSpan('process.batch', (span) => {
  span.setAttribute('batch.size', items.length);

  for (const item of items) {
    processItem(item);
    // Use event for sampling, not span
    if (shouldSample()) {
      span.addEvent('item.processed', { item_id: item.id });
    }
  }

  span.setAttribute('batch.processed', items.length);
  span.end();
});
```

### Anti-Pattern 4: "Timestamp Attributes"

```javascript
// BAD: Timestamps as attributes
span.setAttribute('started_at', Date.now());
span.setAttribute('finished_at', Date.now());
metric.record(value, { timestamp: Date.now() });  // Infinite cardinality!

// GOOD: Use built-in timing
// Spans have automatic start/end times
// Metrics have automatic timestamps
// Events have automatic timestamps
span.addEvent('checkpoint');  // Timestamp is automatic
```

### Anti-Pattern 5: "Full Request/Response Logging"

```javascript
// BAD: Logging entire payloads
logger.info('Request received', { body: req.body });  // Could be huge, PII risk
span.setAttribute('response.body', JSON.stringify(response));  // Size limit issues

// GOOD: Log structure, not content
logger.info('Request received', {
  content_type: req.headers['content-type'],
  body_size: JSON.stringify(req.body).length,
  has_file: !!req.file
});
```

---

## Quality Metrics

### Track Your Telemetry Health

```javascript
// Metrics about your metrics
const telemetryHealth = {
  // Cardinality tracking
  'telemetry.metrics.cardinality': new Gauge(),
  'telemetry.metrics.cardinality.growth_rate': new Gauge(),

  // Cost tracking
  'telemetry.spans.exported': new Counter(),
  'telemetry.spans.dropped': new Counter(),
  'telemetry.metrics.exported': new Counter(),

  // Quality tracking
  'telemetry.spans.with_errors': new Counter(),
  'telemetry.spans.duration_exceeded': new Counter(),
};

// Alert on cardinality growth
alert:
  name: cardinality_explosion
  expr: rate(telemetry.metrics.cardinality[1h]) > 1000
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Metric cardinality growing rapidly"
```

### Quality Checklist

Before deploying new telemetry:

```
[ ] Cardinality calculated and within budget
[ ] All attributes are bounded or normalized
[ ] Semantic conventions used where applicable
[ ] No PII in attributes or logs
[ ] Sampling strategy defined
[ ] Cost impact estimated
[ ] Dashboard/alert exists that uses this data
[ ] Retention requirements considered
```

---

## Summary: The Quality Hierarchy

```
┌─────────────────────────────────────────────────────────────────┐
│                  TELEMETRY QUALITY HIERARCHY                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Level 1: CORRECTNESS                                           │
│  └─ Data is accurate and complete                               │
│  └─ Timestamps are correct                                      │
│  └─ Context propagation works                                   │
│                                                                 │
│  Level 2: ACTIONABILITY                                         │
│  └─ Every signal helps detect, localize, or explain             │
│  └─ Dashboards and alerts are built on this data                │
│  └─ Incidents are diagnosed faster with this data               │
│                                                                 │
│  Level 3: EFFICIENCY                                            │
│  └─ Cardinality is bounded and budgeted                         │
│  └─ Attributes are normalized                                   │
│  └─ Sampling is appropriate for signal type                     │
│                                                                 │
│  Level 4: SUSTAINABILITY                                        │
│  └─ Costs scale linearly (not exponentially) with growth        │
│  └─ Team understands what telemetry exists and why              │
│  └─ Regular reviews prune unused telemetry                      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

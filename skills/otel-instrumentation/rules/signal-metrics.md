---
title: "Metrics Instrumentation"
impact: CRITICAL
tags:
  - metrics
  - counters
  - histograms
  - gauges
  - cardinality
---

# Metrics Reference

Comprehensive guide to metrics instrumentation with OpenTelemetry.

## Official Documentation

- [Metrics Overview](https://opentelemetry.io/docs/concepts/signals/metrics/)
- [Metrics API Specification](https://opentelemetry.io/docs/specs/otel/metrics/api/)
- [Metrics Data Model](https://opentelemetry.io/docs/specs/otel/metrics/data-model/)
- [Metrics Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/general/metrics/)

## Instrument Types

### Counter

**Use for**: Monotonically increasing values

```javascript
const requestCounter = meter.createCounter('http.server.requests', {
  description: 'Total number of HTTP requests',
  unit: '1'
});

// Usage
requestCounter.add(1, {
  'http.method': 'GET',
  'http.status_code': 200
});
```

**Examples**:
- Request counts
- Error counts
- Bytes sent/received
- Items processed

### UpDownCounter

**Use for**: Values that can increase or decrease

```javascript
const activeConnections = meter.createUpDownCounter('db.connections.active', {
  description: 'Number of active database connections',
  unit: '1'
});

// Usage
activeConnections.add(1);   // Connection opened
activeConnections.add(-1);  // Connection closed
```

**Examples**:
- Active connections
- Queue depth
- In-flight requests
- Active sessions

### Gauge (Observable)

**Use for**: Point-in-time measurements

```javascript
meter.createObservableGauge('process.memory.usage', {
  description: 'Current memory usage',
  unit: 'By'
}, (observableResult) => {
  observableResult.observe(process.memoryUsage().heapUsed);
});
```

**Examples**:
- Memory usage
- CPU utilization
- Temperature
- Current queue size

### Histogram

**Use for**: Distribution of values (latency, sizes)

```javascript
const requestDuration = meter.createHistogram('http.server.duration', {
  description: 'HTTP request duration',
  unit: 'ms'
});

// Usage
const startTime = Date.now();
// ... handle request ...
requestDuration.record(Date.now() - startTime, {
  'http.method': 'GET',
  'http.route': '/api/users'
});
```

**Examples**:
- Request latency
- Response sizes
- Batch sizes
- Processing times

### Decision Matrix

| Question | Instrument |
|----------|------------|
| Does it only go up? | Counter |
| Can it go up and down? | UpDownCounter |
| Is it a current state/measurement? | Gauge |
| Do you need percentiles? | Histogram |
| Is it a rate (per second)? | Counter (derive rate) |

## Golden Signals Implementation

### 1. Latency

```javascript
// Histogram for latency distribution
const requestDuration = meter.createHistogram('http.server.duration', {
  description: 'Request duration in milliseconds',
  unit: 'ms'
});

// Record with route context
requestDuration.record(duration, {
  'http.method': method,
  'http.route': route,
  'http.status_code': statusCode
});
```

**Bucket boundaries recommendation**:
```javascript
[5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000]
```

### 2. Traffic

```javascript
// Counter for request volume
const requestCounter = meter.createCounter('http.server.requests', {
  description: 'Total HTTP requests',
  unit: '1'
});

requestCounter.add(1, {
  'http.method': method,
  'http.route': route
});
```

### 3. Errors

```javascript
// Counter for errors (separate from requests for clarity)
const errorCounter = meter.createCounter('http.server.errors', {
  description: 'Total HTTP errors',
  unit: '1'
});

if (statusCode >= 500) {
  errorCounter.add(1, {
    'http.method': method,
    'http.route': route,
    'error.type': errorType
  });
}
```

### 4. Saturation

```javascript
// Gauge for resource utilization
meter.createObservableGauge('system.cpu.utilization', {
  description: 'CPU utilization',
  unit: '1'
}, (result) => {
  result.observe(getCpuUtilization());
});

// UpDownCounter for queue depth
const queueDepth = meter.createUpDownCounter('queue.depth', {
  description: 'Current queue depth',
  unit: '1'
});
```

## Naming Conventions

### Pattern

```
<namespace>.<entity>.<metric>
```

### Examples

```yaml
# HTTP metrics
http.server.requests           # Counter: total requests
http.server.duration           # Histogram: request latency
http.server.request.size       # Histogram: request body size
http.server.response.size      # Histogram: response body size

# Database metrics
db.client.connections.active   # UpDownCounter
db.client.connections.max      # Gauge
db.client.operation.duration   # Histogram

# Custom business metrics
orders.created                 # Counter
orders.value                   # Counter (sum of order values)
checkout.duration              # Histogram
cart.items                     # Histogram (items per cart)
```

### Rules

1. **Use lowercase with dots** - `namespace.entity.metric`
2. **Use standard units** - `ms`, `By`, `1` (dimensionless)
3. **Follow semantic conventions** - Check OTel spec first
4. **Be descriptive** - Name should explain what's measured

## Cardinality Control

### The #1 Cost Driver

```
Cardinality = unique combinations of label values

10 routes × 5 methods × 50 status codes × 10 instances = 25,000 series
Add user_id (100,000 users) = 2.5 BILLION series = $$$$$
```

### Label Hygiene Rules

```yaml
# Rule 1: Only bounded labels
allowed_labels:
  http.method: [GET, POST, PUT, DELETE, PATCH]  # Max 10
  http.status_code: [200, 201, 400, 404, 500]   # Bucket others
  service.name: [api, web, worker]               # Known services
  environment: [prod, staging, dev]              # 3 values

# Rule 2: Never unbounded labels
forbidden_labels:
  user.id         # Millions of values
  request.id      # Unique per request
  order.id        # Unbounded
  timestamp       # Infinite
  url.full        # Contains query params
```

### Status Code Bucketing

```javascript
function bucketStatusCode(code) {
  if (code >= 200 && code < 300) return '2xx';
  if (code >= 300 && code < 400) return '3xx';
  if (code >= 400 && code < 500) return '4xx';
  if (code >= 500) return '5xx';
  return 'other';
}

// Or keep common codes, bucket rare ones
function bucketStatusCodeSelective(code) {
  const common = [200, 201, 400, 401, 403, 404, 500, 502, 503];
  return common.includes(code) ? code.toString() : `${Math.floor(code/100)}xx`;
}
```

### Path Normalization

```javascript
// Before: /users/123/orders/456 (infinite cardinality)
// After:  /users/{id}/orders/{id} (bounded)

function normalizePath(path) {
  return path
    .replace(/\/\d+/g, '/{id}')
    .replace(/\/[a-f0-9-]{36}/g, '/{uuid}')
    .replace(/\/[a-zA-Z0-9]{20,}/g, '/{token}');
}
```

### Cardinality Budget Calculator

```javascript
function calculateCardinality(metrics) {
  let total = 0;
  for (const metric of metrics) {
    let combinations = 1;
    for (const label of metric.labels) {
      combinations *= label.uniqueValues;
    }
    combinations *= metric.instances;
    total += combinations;
  }
  return total;
}

// Example
const metrics = [
  { name: 'http.requests', labels: [
    { name: 'method', uniqueValues: 5 },
    { name: 'route', uniqueValues: 20 },
    { name: 'status', uniqueValues: 10 }
  ], instances: 10 }
];

console.log(calculateCardinality(metrics)); // 10,000 series
```

## Aggregation Temporality

### Cumulative (Default)

```
Value represents total since process start
Good for: Counters where you derive rate on query
Resilient to: Missed scrapes (value doesn't reset)
```

### Delta

```
Value represents change since last export
Good for: High-throughput systems, stateless exporters
Challenge: Must handle process restarts
```

### When to Use Each

| Scenario | Temporality |
|----------|-------------|
| Prometheus backend | Cumulative |
| OTLP push to backend | Delta (usually) |
| Stateless functions | Delta |
| Long-running services | Cumulative |

## Views for Customization

### Dropping Metrics

```javascript
const meterProvider = new MeterProvider({
  views: [
    // Drop noisy metric entirely
    new View({
      instrumentName: 'noisy.metric',
      aggregation: new DropAggregation()
    })
  ]
});
```

### Changing Histogram Buckets

```javascript
new View({
  instrumentName: 'http.server.duration',
  aggregation: new ExplicitBucketHistogramAggregation([
    10, 50, 100, 200, 500, 1000, 2000, 5000
  ])
})
```

### Dropping Labels

```javascript
new View({
  instrumentName: 'http.server.requests',
  attributeKeys: ['http.method', 'http.route'], // Only keep these
})
```

### Renaming Metrics

```javascript
new View({
  instrumentName: 'legacy.metric.name',
  name: 'new.metric.name'
})
```

## Environment-Specific Configuration

### Production

```yaml
# High-value, low-cardinality
metrics:
  collection_interval: 60s
  cardinality_limit: 10000
  labels:
    allowed: [method, route, status_bucket, service]
  histograms:
    bucket_count: 10  # Fewer buckets = less data
```

### Staging

```yaml
# More detail for debugging
metrics:
  collection_interval: 30s
  cardinality_limit: 50000
  labels:
    allowed: [method, route, status, service, version]
  histograms:
    bucket_count: 15
```

### Development

```yaml
# Maximum detail, local only
metrics:
  collection_interval: 10s
  cardinality_limit: unlimited
  labels:
    allowed: all
  histograms:
    bucket_count: 20
```

## Common Anti-Patterns

### 1. User ID as Label

```javascript
// WRONG: Creates millions of series
counter.add(1, { user_id: userId });

// RIGHT: Use traces for per-user data, metrics for aggregates
counter.add(1, { user_tier: userTier });
```

### 2. Timestamp as Label

```javascript
// WRONG: Infinite cardinality
counter.add(1, { timestamp: Date.now() });

// RIGHT: Let the metric system handle timestamps
counter.add(1);
```

### 3. Full URL as Label

```javascript
// WRONG: Query params = unbounded
counter.add(1, { url: req.url });

// RIGHT: Normalized route
counter.add(1, { route: '/users/{id}' });
```

### 4. Too Many Histogram Buckets

```javascript
// WRONG: 100 buckets = 100x data
new ExplicitBucketHistogramAggregation(
  Array.from({length: 100}, (_, i) => i * 10)
);

// RIGHT: 10-15 meaningful buckets
new ExplicitBucketHistogramAggregation([
  10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000
]);
```

---
title: "Spans and Traces"
impact: CRITICAL
tags:
  - traces
  - spans
  - distributed-tracing
  - context-propagation
---

# Spans Reference

Comprehensive guide to traces and spans in OpenTelemetry.

## Official Documentation

- [Traces Overview](https://opentelemetry.io/docs/concepts/signals/traces/)
- [Span Specification](https://opentelemetry.io/docs/specs/otel/trace/api/)
- [Trace Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/general/trace/)
- [HTTP Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/http/)

## When to Create Custom Spans

### Use Auto-Instrumentation When

- Instrumenting HTTP clients/servers
- Database queries
- Message queue operations
- gRPC calls
- Standard framework operations

### Create Custom Spans When

- Tracking business logic operations
- Measuring specific code sections
- Adding context not captured by auto-instrumentation
- Instrumenting custom protocols

### Decision Guide

```
Is there auto-instrumentation available?
├─► Yes: Use it, add attributes if needed
└─► No: Create custom span
    │
    ├─► Is the operation > 1ms typically?
    │   └─► Yes: Create span
    │   └─► No: Use span events instead
    │
    └─► Does it cross a service boundary?
        └─► Yes: Definitely create span
```

## Span Naming Conventions

### Pattern: `verb.object` or `object.verb`

```
Good:
  http.request
  db.query
  cache.get
  order.process
  payment.authorize
  user.authenticate

Bad:
  handleRequest        # Not descriptive
  doStuff              # Meaningless
  /api/users/123       # URL as name (use attributes)
  processOrderAndSendEmail  # Too specific, multiple operations
```

### Rules

1. **Use lowercase with dots** - `service.operation`
2. **Be consistent** - Same pattern across services
3. **Be specific but not too specific** - Operation type, not instance
4. **Avoid dynamic values** - No IDs, timestamps, or user data in names

## Status Codes

### UNSET (Default)

```
When: Operation completed, success/failure unclear or irrelevant
Example: Span for logging operation, informational tracking
```

### OK

```
When: Operation explicitly succeeded
Example: HTTP 2xx response, successful database write
Note: Most spans should NOT set OK - UNSET is fine for success
```

### ERROR

```
When: Operation failed or encountered an error
Example: HTTP 5xx, exception thrown, business rule violation
CRITICAL: Always record the error details
```

### Code Example

```javascript
// Don't overuse OK - UNSET is the default for success
span.setStatus({ code: SpanStatusCode.UNSET });

// Use ERROR for failures - always include description
span.setStatus({
  code: SpanStatusCode.ERROR,
  message: 'Database connection timeout'
});

// Record exception details
span.recordException(error);
span.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
```

## Attribute Best Practices

### Standard Semantic Conventions

Always use semantic conventions when available:

```yaml
# HTTP
http.request.method: GET
http.response.status_code: 200
url.path: /api/users
url.scheme: https

# Database
db.system: postgresql
db.name: users_db
db.operation: SELECT
db.statement: "SELECT * FROM users WHERE id = ?"

# Messaging
messaging.system: kafka
messaging.destination.name: orders
messaging.operation: publish
```

### Custom Attributes

```yaml
# Business context
order.id: "ord_123"
order.total: 99.99
customer.tier: "premium"

# Operation context
retry.count: 2
cache.hit: true
feature.flag: "new_checkout"
```

### Cardinality Awareness

```yaml
# Good - bounded cardinality
http.request.method: GET      # ~10 possible values
http.response.status_code: 200  # ~50 possible values
customer.tier: premium        # 3-5 tiers

# Bad - unbounded cardinality (avoid on metrics, ok on traces)
user.id: "usr_abc123"        # Millions of values
request.id: "req_xyz789"     # Unique per request
timestamp: "2024-01-15T..."  # Infinite values
```

### Attribute Size Limits

```yaml
# Recommended limits
attribute_value_length_limit: 1024  # Characters
attribute_count_limit: 128          # Per span
```

## Span Events vs Logs

### Use Span Events When

- Event is tightly coupled to the span's operation
- Event represents a point-in-time occurrence within the span
- You want the event visible in trace visualization

```javascript
span.addEvent('cache.miss', {
  'cache.key': 'user:123',
  'cache.backend': 'redis'
});

span.addEvent('retry.attempt', {
  'retry.count': 2,
  'retry.reason': 'timeout'
});
```

### Use Logs When

- Event is independent of any specific operation
- Event needs different retention than traces
- Event volume is high (logs can be sampled separately)
- Event needs to be searchable outside trace context

### Decision Matrix

| Scenario | Use |
|----------|-----|
| Exception in span | Span event (recordException) |
| Cache hit/miss | Span event |
| User action during request | Span event |
| Audit log entry | Log with trace context |
| Debug output | Log |
| High-volume events | Log (can sample separately) |

## Span Links

### When to Use Links

Links connect spans that are related but not parent-child:

```
Use links for:
- Batch processing (link to triggering spans)
- Fan-out operations (link to all downstream)
- Async workflows (link to initiating request)
- Message queue consumers (link to producer span)
```

### Example: Batch Processing

```javascript
// Consumer processes batch of messages
const links = messages.map(msg => ({
  context: extract(msg.headers), // Extract trace context from each message
  attributes: { 'messaging.message.id': msg.id }
}));

tracer.startSpan('batch.process', { links });
```

## Sampling Rules

### Always Keep

```yaml
error_traces:
  condition: span.status.code == ERROR
  decision: KEEP
  reason: "Errors always provide value for debugging"

slow_traces:
  condition: span.duration > 1000ms
  decision: KEEP
  reason: "Slow requests indicate problems"

critical_operations:
  condition: span.name in ['payment.process', 'order.create']
  decision: KEEP
  reason: "Business-critical, need full visibility"
```

### Sample Probabilistically

```yaml
default_traffic:
  condition: not (error OR slow OR critical)
  decision: SAMPLE at 1-10%
  reason: "Bulk traffic, statistical sample sufficient"
```

### Always Drop

```yaml
health_checks:
  condition: url.path in ['/health', '/ready', '/live', '/ping']
  decision: DROP
  reason: "No diagnostic value, high volume"

synthetic_monitoring:
  condition: user_agent contains 'synthetic' OR 'monitoring'
  decision: DROP
  reason: "Not real user traffic"

internal_probes:
  condition: source.ip in internal_lb_ranges
  decision: DROP
  reason: "Infrastructure noise"
```

### Attribute Whitelisting

```yaml
# Only propagate these attributes to reduce costs
allowed_span_attributes:
  # Semantic conventions
  - http.request.method
  - http.response.status_code
  - url.path
  - db.system
  - db.operation

  # Custom business attributes
  - order.id
  - customer.tier
  - feature.flag

# Block high-cardinality attributes
blocked_span_attributes:
  - user.id           # Use for logs, not trace attributes
  - request.body      # Too large
  - db.statement      # May contain PII
```

## Context Propagation

### Extracting Context (Incoming)

```javascript
// HTTP server - extract from headers
const context = propagation.extract(ROOT_CONTEXT, req.headers);
const span = tracer.startSpan('handle.request', undefined, context);
```

### Injecting Context (Outgoing)

```javascript
// HTTP client - inject into headers
const headers = {};
propagation.inject(context.active(), headers);
fetch(url, { headers });
```

### Propagation Formats

```yaml
# Recommended: W3C Trace Context (default)
traceparent: 00-{trace-id}-{span-id}-{flags}
tracestate: vendor=value

# Legacy: B3 (for Zipkin compatibility)
X-B3-TraceId: {trace-id}
X-B3-SpanId: {span-id}
X-B3-Sampled: 1
```

## Performance Considerations

### Span Creation Overhead

```
Typical overhead per span:
- Memory: ~500 bytes
- CPU: ~1-5 microseconds
- Network: ~200 bytes exported (compressed)
```

### Optimization Tips

1. **Avoid creating spans in tight loops**
   ```javascript
   // Bad: span per iteration
   for (const item of items) {
     const span = tracer.startSpan('process.item');
     process(item);
     span.end();
   }

   // Good: single span with event per item
   const span = tracer.startSpan('process.batch');
   for (const item of items) {
     span.addEvent('item.processed', { 'item.id': item.id });
     process(item);
   }
   span.end();
   ```

2. **Use span events for fine-grained timing**

3. **Set appropriate sampling rates**

4. **Batch exports** (default in most SDKs)

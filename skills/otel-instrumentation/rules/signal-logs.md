---
title: "Structured Logging"
impact: CRITICAL
tags:
  - logs
  - structured-logging
  - trace-correlation
  - severity
---

# Logs Reference

Comprehensive guide to structured logging with OpenTelemetry.

## Official Documentation

- [Logs Overview](https://opentelemetry.io/docs/concepts/signals/logs/)
- [Logs API Specification](https://opentelemetry.io/docs/specs/otel/logs/)
- [Logs Data Model](https://opentelemetry.io/docs/specs/otel/logs/data-model/)
- [Logs Bridge API](https://opentelemetry.io/docs/specs/otel/logs/bridge-api/)

## Structured Logging Only

### Why Structured Logs

```javascript
// Bad: Unstructured log
logger.info(`User ${userId} placed order ${orderId} for $${amount}`);
// Output: "User 123 placed order 456 for $99.99"
// Problem: Can't query, filter, or aggregate

// Good: Structured log
logger.info('order.placed', {
  user_id: userId,
  order_id: orderId,
  amount: amount,
  currency: 'USD'
});
// Output: {"message":"order.placed","user_id":"123","order_id":"456","amount":99.99}
// Benefit: Queryable, filterable, aggregatable
```

### Structure Requirements

```yaml
# Every log entry should have
required_fields:
  timestamp: ISO8601 format
  level: severity level
  message: human-readable description
  service.name: originating service

# Trace correlation (when in request context)
correlation_fields:
  trace_id: W3C trace ID
  span_id: current span ID

# Context fields (as appropriate)
context_fields:
  environment: prod/staging/dev
  version: service version
  host: hostname/pod name
```

## Log Levels by Environment

### Production

```yaml
production:
  default_level: WARN

  # Only emit for
  ERROR: Always (exceptions, failures)
  WARN: Always (degraded state, retries)
  INFO: Sampled 10% (business events)
  DEBUG: Never
  TRACE: Never
```

### Staging

```yaml
staging:
  default_level: INFO

  ERROR: Always
  WARN: Always
  INFO: Always
  DEBUG: On-demand (feature flag)
  TRACE: Never
```

### Development

```yaml
development:
  default_level: DEBUG

  # All levels enabled
  ERROR: Always
  WARN: Always
  INFO: Always
  DEBUG: Always
  TRACE: On-demand
```

### Dynamic Level Control

```javascript
// Allow runtime level changes via environment variable
const level = process.env.LOG_LEVEL ||
  (process.env.NODE_ENV === 'production' ? 'warn' : 'info');

// Or via feature flag
const level = featureFlags.get('log_level') || 'warn';
```

## Trace Context Injection

### Automatic Correlation

```javascript
// OTel automatically injects trace context when configured
const logger = new Logger({
  // Logger configuration
});

// In a traced context, logs automatically include:
// - trace_id
// - span_id
// - trace_flags
logger.info('Processing request');
// Output includes: {"trace_id":"abc123","span_id":"def456",...}
```

### Manual Injection (if needed)

```javascript
import { trace, context } from '@opentelemetry/api';

function getTraceContext() {
  const span = trace.getSpan(context.active());
  if (!span) return {};

  const spanContext = span.spanContext();
  return {
    trace_id: spanContext.traceId,
    span_id: spanContext.spanId,
    trace_flags: spanContext.traceFlags.toString(16)
  };
}

logger.info('message', {
  ...getTraceContext(),
  custom_field: value
});
```

### Correlation Benefits

```
1. Click trace_id in log viewer → See full trace
2. See error in trace → Jump to related logs
3. Aggregate logs by trace_id → Full request story
```

## Severity Mapping

### OTel Severity Numbers

| SeverityNumber | SeverityText | Use Case |
|----------------|--------------|----------|
| 1-4 | TRACE | Fine-grained debugging |
| 5-8 | DEBUG | Diagnostic information |
| 9-12 | INFO | Normal operations |
| 13-16 | WARN | Potentially harmful |
| 17-20 | ERROR | Error events |
| 21-24 | FATAL | Application crash |

### Mapping from Common Loggers

```yaml
# pino → OTel
pino:
  trace: TRACE (1)
  debug: DEBUG (5)
  info: INFO (9)
  warn: WARN (13)
  error: ERROR (17)
  fatal: FATAL (21)

# winston → OTel
winston:
  silly: TRACE (1)
  debug: DEBUG (5)
  verbose: DEBUG (7)
  info: INFO (9)
  warn: WARN (13)
  error: ERROR (17)
```

### Choosing Severity

```
TRACE: Loop iterations, variable values
DEBUG: Function entry/exit, intermediate states
INFO: Startup, shutdown, configuration, business events
WARN: Deprecated APIs, poor performance, retries, recovery
ERROR: Exceptions, failures, data inconsistencies
FATAL: Unrecoverable errors, crash imminent
```

## Log Body vs Attributes

### Body

```yaml
# Use body for
- Human-readable message
- The "what happened" summary
- Should be readable without attributes

# Example
body: "Order processing completed successfully"
```

### Attributes

```yaml
# Use attributes for
- Structured data for querying
- Context that adds detail
- Values to filter/aggregate on

# Example
attributes:
  order.id: "ord_123"
  order.total: 99.99
  order.items_count: 3
  processing.duration_ms: 150
  customer.tier: "premium"
```

### Combined Example

```javascript
logger.info('Order processed', {
  // These become attributes
  'order.id': order.id,
  'order.total': order.total,
  'customer.id': customer.id,
  'processing.duration_ms': duration
});

// OTel Log Record:
// {
//   body: "Order processed",
//   attributes: {
//     "order.id": "ord_123",
//     "order.total": 99.99,
//     ...
//   }
// }
```

## Sampling Strategy

### By Severity

```yaml
sampling_rules:
  FATAL: 1.0    # 100% - always keep
  ERROR: 1.0    # 100% - always keep
  WARN: 1.0     # 100% - keep all warnings
  INFO: 0.1     # 10% - sample routine info
  DEBUG: 0.0    # 0% - drop in production
  TRACE: 0.0    # 0% - drop in production
```

### By Content

```yaml
always_keep:
  - severity >= ERROR
  - attributes.security_event == true
  - attributes.audit_log == true
  - attributes.slo_relevant == true

always_drop:
  - body contains "health check"
  - attributes.synthetic == true
  - severity == DEBUG AND environment == production

sample_probabilistically:
  - severity == INFO: 10%
  - severity == WARN: 50%
```

### Implementation

```javascript
// SDK-side sampling
function shouldLog(level, attributes) {
  // Always keep errors
  if (level >= LogLevel.ERROR) return true;

  // Always keep security/audit
  if (attributes.security_event || attributes.audit_log) return true;

  // Sample INFO in production
  if (level === LogLevel.INFO && process.env.NODE_ENV === 'production') {
    return Math.random() < 0.1; // 10%
  }

  // Drop DEBUG in production
  if (level <= LogLevel.DEBUG && process.env.NODE_ENV === 'production') {
    return false;
  }

  return true;
}
```

## Tiered Destinations

### Hot Storage (Fast, Expensive)

```yaml
hot_tier:
  destination: primary_backend
  retention: 7_days

  include:
    - severity >= WARN
    - attributes.priority == "high"
    - attributes.alert_relevant == true

  features:
    - Full-text search
    - Real-time alerting
    - Fast queries
```

### Warm Storage (Medium)

```yaml
warm_tier:
  destination: secondary_backend
  retention: 30_days

  include:
    - severity == INFO
    - not in hot_tier

  features:
    - Indexed search
    - Slower queries OK
```

### Cold Storage (Slow, Cheap)

```yaml
cold_tier:
  destination: object_storage
  retention: 1_year

  include:
    - all logs (compressed)

  features:
    - Compliance/audit
    - Forensic analysis
    - No real-time queries
```

### Collector Configuration

```yaml
exporters:
  otlp/hot:
    endpoint: hot-backend:4317
  otlp/warm:
    endpoint: warm-backend:4317
  file/cold:
    path: /logs/archive

processors:
  routing:
    from_attribute: severity
    table:
      - value: ERROR
        exporters: [otlp/hot, file/cold]
      - value: WARN
        exporters: [otlp/hot, file/cold]
      - value: INFO
        exporters: [otlp/warm, file/cold]
      - value: DEBUG
        exporters: [file/cold]

pipelines:
  logs:
    receivers: [otlp]
    processors: [routing]
    exporters: [otlp/hot, otlp/warm, file/cold]
```

## Common Patterns

### Request Logging

```javascript
// Log at request start and end
app.use((req, res, next) => {
  const startTime = Date.now();

  // Request start (DEBUG level - often sampled away)
  logger.debug('request.start', {
    'http.method': req.method,
    'http.url': req.url,
    'http.user_agent': req.get('user-agent')
  });

  res.on('finish', () => {
    const duration = Date.now() - startTime;

    // Request end (INFO level with key metrics)
    logger.info('request.complete', {
      'http.method': req.method,
      'http.route': req.route?.path,
      'http.status_code': res.statusCode,
      'http.duration_ms': duration
    });
  });

  next();
});
```

### Error Logging

```javascript
// Comprehensive error logging
function logError(error, context = {}) {
  logger.error('error.occurred', {
    // Error details
    'error.type': error.name,
    'error.message': error.message,
    'error.stack': error.stack,

    // Context
    ...context,

    // Classification
    'error.recoverable': isRecoverable(error),
    'error.user_facing': isUserFacing(error)
  });
}
```

### Audit Logging

```javascript
// Audit logs - never sample
function auditLog(action, actor, resource, details = {}) {
  logger.info('audit.event', {
    'audit.action': action,
    'audit.actor.id': actor.id,
    'audit.actor.type': actor.type,
    'audit.resource.type': resource.type,
    'audit.resource.id': resource.id,
    'audit.timestamp': new Date().toISOString(),
    'audit.ip': actor.ip,
    'audit_log': true,  // Flag for routing - never sample
    ...details
  });
}

// Usage
auditLog('user.login',
  { id: userId, type: 'user', ip: clientIp },
  { type: 'session', id: sessionId },
  { method: '2fa', success: true }
);
```

### Business Event Logging

```javascript
// Business events - sample-able in high volume
function logBusinessEvent(event, data) {
  logger.info(`business.${event}`, {
    'event.name': event,
    'event.timestamp': new Date().toISOString(),
    ...data,
    'business_event': true
  });
}

// Usage
logBusinessEvent('order.placed', {
  'order.id': order.id,
  'order.total': order.total,
  'order.items_count': order.items.length,
  'customer.tier': customer.tier
});
```

## Anti-Patterns

### 1. Logging Sensitive Data

```javascript
// WRONG: PII in logs
logger.info('User logged in', { email: user.email, password: password });

// RIGHT: Redact or hash sensitive data
logger.info('User logged in', {
  user_id: user.id,
  email_domain: user.email.split('@')[1]  // Only domain
});
```

### 2. Unstructured Interpolation

```javascript
// WRONG: Important data buried in string
logger.info(`Order ${orderId} for ${userId} totaling ${amount}`);

// RIGHT: Structured attributes
logger.info('order.created', {
  order_id: orderId,
  user_id: userId,
  amount: amount
});
```

### 3. Too Verbose in Production

```javascript
// WRONG: Debug logging always on
logger.debug(`Processing item ${i} of ${total}`); // In a loop

// RIGHT: Conditional logging
if (process.env.NODE_ENV !== 'production') {
  logger.debug(`Processing item ${i} of ${total}`);
}
```

### 4. Missing Context

```javascript
// WRONG: No context
logger.error('Failed to process');

// RIGHT: Rich context
logger.error('order.processing.failed', {
  order_id: orderId,
  error_type: error.name,
  error_message: error.message,
  retry_count: retryCount,
  last_step: lastCompletedStep
});
```

### 5. Logging in Tight Loops

```javascript
// WRONG: Log per iteration
items.forEach(item => {
  logger.debug('Processing item', { id: item.id });
  process(item);
});

// RIGHT: Log batch summary
logger.info('batch.processing.start', { count: items.length });
items.forEach(item => process(item));
logger.info('batch.processing.complete', {
  count: items.length,
  duration_ms: duration
});
```

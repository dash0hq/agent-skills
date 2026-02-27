# OpenTelemetry Node.js Example

A comprehensive example demonstrating proper OpenTelemetry instrumentation in Node.js applications.

## Features Demonstrated

### 1. Auto-Instrumentation
- HTTP requests via `@opentelemetry/auto-instrumentations-node`
- Automatic trace context propagation
- Zero-code instrumentation for Express

### 2. Custom Spans
- Business logic spans (`order.process`, `payment.process`)
- Proper error handling with `recordException()`
- Meaningful span names (noun.verb format)
- Nested span hierarchies

### 3. Golden Signal Metrics
| Metric | Type | Purpose |
|--------|------|---------|
| `http.server.duration` | Histogram | Latency tracking |
| `http.server.requests` | Counter | Traffic monitoring |
| `http.server.errors` | Counter | Error rate SLOs |
| `http.server.active_connections` | UpDownCounter | Saturation |

### 4. Structured Logging
- Trace correlation (`trace_id`, `span_id`)
- Structured attributes (not string interpolation)
- Environment-appropriate log levels

### 5. Cardinality Management
- Normalized paths on metrics (`/users/123` → `/users/{id}`)
- Bucketed status codes on metrics (`200` → `2xx`)
- High-cardinality attributes only on spans, not metrics

## Quick Start

### Install Dependencies

```bash
npm install
```

### Run Without OpenTelemetry (Development)

```bash
npm run dev
```

### Run With OpenTelemetry (Console Output)

```bash
# Quick test - exports telemetry to console
npm run start:otel:console
```

### Run With OpenTelemetry (OTLP Export)

```bash
# Required: Service name
export OTEL_SERVICE_NAME="otel-nodejs-example"

# Required: Enable OTLP exporters (default is "none"!)
export OTEL_TRACES_EXPORTER="otlp"
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"

# Required: Collector endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4317"

# Optional: Auth headers for Dash0 or other backends
# export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
# export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"

# Run with auto-instrumentation
npm run start:otel
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/orders` | POST | Create order (demonstrates custom spans + metrics) |
| `/orders/:id` | GET | Get order by ID |
| `/batch` | POST | Batch processing (demonstrates batch span pattern) |
| `/slow` | GET | Simulate slow operation |
| `/error` | GET | Simulate error (demonstrates error recording) |
| `/nested` | GET | Nested spans example |

## Testing with VS Code REST Client

Install the [REST Client extension](https://marketplace.visualstudio.com/items?itemName=humao.rest-client), then open `api.http` and click "Send Request" above any request.

The file includes tests for all endpoints with dynamic variables and chained requests.

## Test Commands (curl)

```bash
# Health check
curl http://localhost:3001/health

# Create an order
curl -X POST http://localhost:3001/orders \
  -H "Content-Type: application/json" \
  -d '{"total": 49.99, "items": [{"name": "Widget", "price": 49.99}]}'

# Get order (path normalization demo)
curl http://localhost:3001/orders/ord_123

# Batch processing
curl -X POST http://localhost:3001/batch \
  -H "Content-Type: application/json" \
  -d '{"items": [{"id": 1}, {"id": 2}, {"id": 3}]}'

# Slow operation (latency demo)
curl http://localhost:3001/slow

# Error simulation
curl http://localhost:3001/error

# Nested spans
curl http://localhost:3001/nested
```

## Project Structure

```
src/
├── app.js                    # Express server and routes
├── telemetry.js              # OpenTelemetry setup (tracer, meter, metrics)
├── logger.js                 # Structured logging with trace correlation
├── middleware/
│   └── metrics.js            # HTTP metrics middleware
└── services/
    └── order-service.js      # Business logic with instrumentation
```

## Key Patterns

### Custom Spans

```javascript
import { withSpan, SpanStatusCode } from "./telemetry.js";

async function processOrder(order) {
  return withSpan("order.process", {
    "order.id": order.id,
    "order.total": order.total,
  }, async (span) => {
    // Business logic here
    span.addEvent("order.validated");
    return result;
  });
}
```

### Trace-Correlated Logging

```javascript
import logger from "./logger.js";

// Automatically includes trace_id and span_id
logger.info("order.completed", {
  order_id: orderId,
  total: order.total,
});
```

### Cardinality-Safe Metrics

```javascript
// GOOD: Bounded attributes only
httpDuration.record(duration, {
  method: "POST",
  route: "/users/{id}",  // Normalized
  status: "2xx",         // Bucketed
});

// BAD: Unbounded attributes (DON'T DO THIS)
httpDuration.record(duration, {
  user_id: userId,       // Millions of unique values!
  request_id: requestId, // Unbounded
});
```

### Batch Processing

```javascript
// GOOD: Single span for batch
tracer.startActiveSpan("batch.process", (span) => {
  span.setAttribute("batch.size", items.length);
  items.forEach(process);
  span.end();
});

// BAD: Span per item (don't do this!)
items.forEach(item => {
  tracer.startActiveSpan("process.item", span => {
    process(item);
    span.end();
  });
});
```

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `OTEL_SERVICE_NAME` | Service name for telemetry | `otel-nodejs-example` |
| `OTEL_TRACES_EXPORTER` | Trace exporter type (`otlp`, `console`, `none`) | `otlp` |
| `OTEL_METRICS_EXPORTER` | Metrics exporter type | `otlp` |
| `OTEL_LOGS_EXPORTER` | Logs exporter type | `otlp` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP collector endpoint | `http://localhost:4317` |
| `OTEL_EXPORTER_OTLP_HEADERS` | Auth headers for backend | `Authorization=Bearer xxx` |
| `NODE_ENV` | Environment (affects log levels) | `production` |
| `PORT` | Server port | `3001` |

## Kubernetes Setup

When running in Kubernetes without the Dash0 Operator:

```bash
export OTEL_RESOURCE_ATTRIBUTES="k8s.pod.name=$(hostname),k8s.pod.uid=$(POD_UID)"
```

For automatic instrumentation, consider using the [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator).

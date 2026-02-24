---
name: '@otel-nodejs'
description: OpenTelemetry implementation for Node.js and TypeScript applications. Use when setting up OTel SDK, configuring auto-instrumentation, or adding manual spans/metrics/logs in Node.js projects. Triggers on Node.js observability, Express/Fastify/NestJS instrumentation, TypeScript telemetry, or pino/winston integration.
license: MIT
metadata:
  author: mthines
  version: '1.0.0'
  requires:
    - '@otel-telemetry'
---

# OpenTelemetry for Node.js

This skill provides implementation guidance for OpenTelemetry in Node.js and TypeScript applications.

> **Prerequisites**: Review `@otel-telemetry` for core concepts before implementing.

## Official Documentation

- [OpenTelemetry JavaScript](https://opentelemetry.io/docs/languages/js/)
- [Getting Started Guide](https://opentelemetry.io/docs/languages/js/getting-started/nodejs/)
- [API Reference](https://open-telemetry.github.io/opentelemetry-js/)
- [Instrumentation Libraries](https://opentelemetry.io/docs/languages/js/libraries/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

## Prerequisites

### Node.js Version

```yaml
minimum: Node.js 18.x
recommended: Node.js 20.x or 22.x
```

### Core Packages

```bash
# Minimal setup
npm install @opentelemetry/sdk-node \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-proto \
  @opentelemetry/exporter-metrics-otlp-proto

# With logs
npm install @opentelemetry/exporter-logs-otlp-proto \
  @opentelemetry/instrumentation-pino  # or winston
```

## SDK Initialization

### The `--import` Flag Approach (Recommended)

Create `instrumentation.ts`:

```typescript
import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-proto';
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-proto';
import { PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';
import { Resource } from '@opentelemetry/resources';
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT
} from '@opentelemetry/semantic-conventions';

const sdk = new NodeSDK({
  resource: new Resource({
    [ATTR_SERVICE_NAME]: process.env.OTEL_SERVICE_NAME || 'my-service',
    [ATTR_SERVICE_VERSION]: process.env.npm_package_version || '0.0.0',
    [ATTR_DEPLOYMENT_ENVIRONMENT]: process.env.NODE_ENV || 'development'
  }),

  traceExporter: new OTLPTraceExporter({
    url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT
  }),

  metricReader: new PeriodicExportingMetricReader({
    exporter: new OTLPMetricExporter({
      url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT
    }),
    exportIntervalMillis: 60000
  }),

  instrumentations: [
    getNodeAutoInstrumentations({
      // Disable noisy instrumentations
      '@opentelemetry/instrumentation-fs': { enabled: false }
    })
  ]
});

sdk.start();

// Graceful shutdown
process.on('SIGTERM', () => {
  sdk.shutdown()
    .then(() => console.log('OTel SDK shut down'))
    .catch((err) => console.error('Error shutting down OTel SDK', err))
    .finally(() => process.exit(0));
});
```

Run your application:

```bash
# TypeScript (with tsx)
node --import tsx ./instrumentation.ts --import ./instrumentation.ts app.ts

# JavaScript (compile first)
npx tsc instrumentation.ts
node --import ./instrumentation.js app.js
```

### Why `--import`?

The `--import` flag ensures instrumentation loads before any application code:

```
Without --import:
1. App code loads
2. HTTP module loads (not instrumented)
3. OTel SDK starts
4. Too late to instrument HTTP

With --import:
1. OTel SDK starts
2. Instrumentations register
3. App code loads
4. HTTP module loads (instrumented!)
```

## Auto-Instrumentation

### What Gets Instrumented

The `@opentelemetry/auto-instrumentations-node` package instruments:

| Library | Creates |
|---------|---------|
| http/https | HTTP server and client spans |
| express | Route spans with parameters |
| fastify | Route spans |
| koa | Route spans |
| nestjs | Controller/handler spans |
| pg | PostgreSQL query spans |
| mysql/mysql2 | MySQL query spans |
| mongodb | MongoDB operation spans |
| redis | Redis command spans |
| ioredis | Redis command spans |
| grpc | gRPC client/server spans |
| aws-sdk | AWS service call spans |
| fetch | Fetch client spans |
| pino | Log correlation |
| winston | Log correlation |

### Configuration

```typescript
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';

const instrumentations = getNodeAutoInstrumentations({
  // Disable specific instrumentations
  '@opentelemetry/instrumentation-fs': {
    enabled: false
  },

  // Configure HTTP instrumentation
  '@opentelemetry/instrumentation-http': {
    ignoreIncomingPaths: ['/health', '/ready', '/metrics'],
    ignoreOutgoingUrls: [/.*elasticsearch.*/]
  },

  // Configure Express
  '@opentelemetry/instrumentation-express': {
    ignoreLayers: [/middleware/]
  },

  // Configure database
  '@opentelemetry/instrumentation-pg': {
    enhancedDatabaseReporting: true
  }
});
```

### Selective Auto-Instrumentation

```typescript
// Import only what you need (reduces bundle size)
import { HttpInstrumentation } from '@opentelemetry/instrumentation-http';
import { ExpressInstrumentation } from '@opentelemetry/instrumentation-express';
import { PgInstrumentation } from '@opentelemetry/instrumentation-pg';

const sdk = new NodeSDK({
  instrumentations: [
    new HttpInstrumentation(),
    new ExpressInstrumentation(),
    new PgInstrumentation()
  ]
});
```

## Manual Instrumentation

### Getting Tracer/Meter/Logger

```typescript
import { trace, metrics, context } from '@opentelemetry/api';
import { logs } from '@opentelemetry/api-logs';

// Get instances
const tracer = trace.getTracer('my-service', '1.0.0');
const meter = metrics.getMeter('my-service', '1.0.0');
const logger = logs.getLogger('my-service', '1.0.0');
```

### Creating Custom Spans

```typescript
import { trace, SpanStatusCode, SpanKind } from '@opentelemetry/api';

const tracer = trace.getTracer('my-service');

async function processOrder(order: Order) {
  // Start span
  return tracer.startActiveSpan('order.process', async (span) => {
    try {
      // Add attributes
      span.setAttribute('order.id', order.id);
      span.setAttribute('order.total', order.total);
      span.setAttribute('customer.tier', order.customer.tier);

      // Add event
      span.addEvent('validation.start');
      await validateOrder(order);
      span.addEvent('validation.complete');

      // Process the order
      const result = await executeOrder(order);

      // Set success status (optional - UNSET is fine)
      span.setStatus({ code: SpanStatusCode.OK });

      return result;
    } catch (error) {
      // Record error
      span.recordException(error);
      span.setStatus({
        code: SpanStatusCode.ERROR,
        message: error.message
      });
      throw error;
    } finally {
      // Always end span
      span.end();
    }
  });
}
```

### Recording Metrics

```typescript
import { metrics } from '@opentelemetry/api';

const meter = metrics.getMeter('my-service');

// Counter
const requestCounter = meter.createCounter('http.server.requests', {
  description: 'Total HTTP requests',
  unit: '1'
});

// Histogram
const requestDuration = meter.createHistogram('http.server.duration', {
  description: 'HTTP request duration',
  unit: 'ms'
});

// UpDownCounter
const activeConnections = meter.createUpDownCounter('db.connections.active', {
  description: 'Active database connections',
  unit: '1'
});

// Observable Gauge
meter.createObservableGauge('process.memory.heap', {
  description: 'Heap memory usage',
  unit: 'By'
}, (result) => {
  result.observe(process.memoryUsage().heapUsed);
});

// Usage in middleware
app.use((req, res, next) => {
  const start = Date.now();

  res.on('finish', () => {
    const duration = Date.now() - start;
    const attributes = {
      'http.method': req.method,
      'http.route': req.route?.path || 'unknown',
      'http.status_code': res.statusCode
    };

    requestCounter.add(1, attributes);
    requestDuration.record(duration, attributes);
  });

  next();
});
```

### Injecting Trace Context into Logs

```typescript
import { trace, context } from '@opentelemetry/api';
import pino from 'pino';

// Create logger with trace context mixin
const logger = pino({
  mixin() {
    const span = trace.getSpan(context.active());
    if (!span) return {};

    const spanContext = span.spanContext();
    return {
      trace_id: spanContext.traceId,
      span_id: spanContext.spanId,
      trace_flags: spanContext.traceFlags.toString(16)
    };
  }
});

// Usage - trace context automatically included
logger.info({ order_id: '123' }, 'Processing order');
// Output: {"trace_id":"abc123","span_id":"def456","order_id":"123","msg":"Processing order"}
```

## Framework-Specific Guides

### Express

```typescript
import express from 'express';
import { trace, SpanStatusCode } from '@opentelemetry/api';

const app = express();
const tracer = trace.getTracer('express-api');

// Middleware for custom business spans
app.post('/orders', async (req, res) => {
  return tracer.startActiveSpan('order.create', async (span) => {
    try {
      span.setAttribute('order.items_count', req.body.items.length);

      const order = await createOrder(req.body);

      span.setAttribute('order.id', order.id);
      span.setAttribute('order.total', order.total);

      res.json(order);
    } catch (error) {
      span.recordException(error);
      span.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
      res.status(500).json({ error: error.message });
    } finally {
      span.end();
    }
  });
});
```

### Fastify

```typescript
import Fastify from 'fastify';
import { trace } from '@opentelemetry/api';

const fastify = Fastify();
const tracer = trace.getTracer('fastify-api');

fastify.post('/orders', async (request, reply) => {
  return tracer.startActiveSpan('order.create', async (span) => {
    try {
      const order = await createOrder(request.body);
      span.setAttribute('order.id', order.id);
      return order;
    } finally {
      span.end();
    }
  });
});
```

### NestJS

```typescript
import { Injectable } from '@nestjs/common';
import { trace, SpanStatusCode } from '@opentelemetry/api';

@Injectable()
export class OrderService {
  private readonly tracer = trace.getTracer('order-service');

  async createOrder(dto: CreateOrderDto) {
    return this.tracer.startActiveSpan('order.create', async (span) => {
      try {
        span.setAttribute('order.customer_id', dto.customerId);

        const order = await this.orderRepository.create(dto);

        span.setAttribute('order.id', order.id);
        return order;
      } catch (error) {
        span.recordException(error);
        span.setStatus({ code: SpanStatusCode.ERROR });
        throw error;
      } finally {
        span.end();
      }
    });
  }
}
```

### Next.js (App Router)

See [examples/nextjs-app.md](./examples/nextjs-app.md) for comprehensive Next.js setup.

## Logger Integration

### pino

```bash
npm install @opentelemetry/instrumentation-pino
```

```typescript
// instrumentation.ts
import { PinoInstrumentation } from '@opentelemetry/instrumentation-pino';

const sdk = new NodeSDK({
  instrumentations: [
    new PinoInstrumentation({
      // Log correlation
      logHook: (span, record) => {
        record['resource.service.name'] = 'my-service';
      }
    })
  ]
});
```

### winston

```bash
npm install @opentelemetry/instrumentation-winston
```

```typescript
// instrumentation.ts
import { WinstonInstrumentation } from '@opentelemetry/instrumentation-winston';

const sdk = new NodeSDK({
  instrumentations: [
    new WinstonInstrumentation({
      // Inject trace context
      logHook: (span, record) => {
        record['trace_id'] = span.spanContext().traceId;
        record['span_id'] = span.spanContext().spanId;
      }
    })
  ]
});
```

## Testing Your Instrumentation

### Local Development Setup

```bash
# Run local collector + Jaeger
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest

# Set environment
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_SERVICE_NAME=my-service

# Run your app
node --import ./instrumentation.js app.js

# Open Jaeger UI
open http://localhost:16686
```

### Verification Checklist

```yaml
checklist:
  - [ ] Traces appear in Jaeger/backend
  - [ ] Spans have correct names (verb.object pattern)
  - [ ] Attributes are present and correct
  - [ ] Errors are recorded with stack traces
  - [ ] Context propagates across async boundaries
  - [ ] HTTP headers contain traceparent
  - [ ] Logs include trace_id and span_id
  - [ ] Metrics are being exported
```

### Debug Logging

```bash
# Enable OTel debug logging
export OTEL_LOG_LEVEL=debug

# See what's being exported
export OTEL_TRACES_EXPORTER=console  # Print to console instead
```

## Common Issues

### Traces Not Appearing

```yaml
checklist:
  - [ ] SDK initialized before app code (--import flag)
  - [ ] OTEL_EXPORTER_OTLP_ENDPOINT is correct
  - [ ] Network allows outbound to collector
  - [ ] forceFlush() called on shutdown
  - [ ] Sample rate > 0
```

### Missing Spans in Trace

```typescript
// Problem: Context lost in async code
setTimeout(() => {
  // This span won't be connected to parent
  tracer.startSpan('child');
}, 100);

// Solution: Preserve context
import { context } from '@opentelemetry/api';

const ctx = context.active();
setTimeout(() => {
  context.with(ctx, () => {
    tracer.startSpan('child'); // Now connected
  });
}, 100);
```

### Memory Issues

```typescript
// Problem: Creating spans in tight loop
for (const item of largeArray) {
  const span = tracer.startSpan('process.item'); // Memory explosion
  process(item);
  span.end();
}

// Solution: Single span with events
const span = tracer.startSpan('process.batch');
span.setAttribute('batch.size', largeArray.length);
for (const item of largeArray) {
  span.addEvent('item.processed', { 'item.id': item.id });
  process(item);
}
span.end();
```

## Reference Documents

- [SDK Setup Reference](./references/sdk-setup.md)
- [Auto-Instrumentation Reference](./references/auto-instrumentation.md)
- [Manual Instrumentation Reference](./references/manual-instrumentation.md)

## Examples

- [Express API Example](./examples/express-api.md)
- [Next.js App Example](./examples/nextjs-app.md)

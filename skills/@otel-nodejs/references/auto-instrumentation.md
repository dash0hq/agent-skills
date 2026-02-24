# Auto-Instrumentation Reference

Guide to automatic instrumentation with OpenTelemetry for Node.js.

## Official Documentation

- [Auto-Instrumentation Package](https://www.npmjs.com/package/@opentelemetry/auto-instrumentations-node)
- [Instrumentation Registry](https://opentelemetry.io/ecosystem/registry/?language=js)
- [Instrumentation Configuration](https://opentelemetry.io/docs/languages/js/instrumentation/)

## @opentelemetry/auto-instrumentations-node

### Installation

```bash
npm install @opentelemetry/auto-instrumentations-node
```

### Basic Usage

```typescript
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';

const sdk = new NodeSDK({
  instrumentations: [getNodeAutoInstrumentations()]
});
```

### What Gets Instrumented

| Category | Libraries | Creates |
|----------|-----------|---------|
| HTTP | http, https | Server/client spans |
| Express | express | Route spans, middleware |
| Fastify | fastify | Route spans |
| Koa | koa | Route spans |
| Hapi | @hapi/hapi | Route spans |
| NestJS | @nestjs/* | Controller spans |
| GraphQL | graphql | Operation spans |
| gRPC | @grpc/grpc-js | RPC spans |
| Database | pg, mysql, mysql2, mongodb, redis, ioredis | Query spans |
| ORM | knex, sequelize, typeorm, prisma | Query spans |
| AWS | aws-sdk, @aws-sdk/* | Service call spans |
| Messaging | amqplib, kafkajs | Producer/consumer spans |
| Cache | memcached, lru-memoizer | Operation spans |
| Fetch | fetch, undici | Client spans |
| DNS | dns | Lookup spans |
| Net | net | Connection spans |
| FS | fs | File operation spans |
| Logging | pino, winston, bunyan | Log correlation |

## Configuration Options

### Disabling Specific Instrumentations

```typescript
const instrumentations = getNodeAutoInstrumentations({
  // Disable filesystem instrumentation (very noisy)
  '@opentelemetry/instrumentation-fs': {
    enabled: false
  },

  // Disable DNS (usually not useful)
  '@opentelemetry/instrumentation-dns': {
    enabled: false
  },

  // Disable net (low-level, usually covered by http)
  '@opentelemetry/instrumentation-net': {
    enabled: false
  }
});
```

### HTTP Instrumentation Options

```typescript
getNodeAutoInstrumentations({
  '@opentelemetry/instrumentation-http': {
    // Ignore health check endpoints
    ignoreIncomingPaths: [
      '/health',
      '/ready',
      '/live',
      '/metrics',
      /^\/internal\//  // Regex pattern
    ],

    // Ignore outgoing requests
    ignoreOutgoingUrls: [
      /.*elasticsearch.*/,
      /.*redis.*/,
      'http://localhost:9200'
    ],

    // Custom span name
    requestHook: (span, request) => {
      span.updateName(`${request.method} ${request.path}`);
    },

    // Add custom attributes
    applyCustomAttributesOnSpan: (span, request, response) => {
      span.setAttribute('http.request_id', request.headers['x-request-id']);
    },

    // Filter requests (return false to ignore)
    ignoreIncomingRequestHook: (request) => {
      return request.headers['user-agent']?.includes('kube-probe');
    }
  }
});
```

### Express Instrumentation Options

```typescript
getNodeAutoInstrumentations({
  '@opentelemetry/instrumentation-express': {
    // Include middleware in traces
    ignoreLayersType: [], // Don't ignore any layers

    // Or ignore specific middleware
    ignoreLayers: [
      'middleware - bodyParser',
      'middleware - cors'
    ],

    // Custom span naming
    spanNameHook: (info, defaultName) => {
      return `${info.request.method} ${info.route}`;
    }
  }
});
```

### Database Instrumentation Options

```typescript
getNodeAutoInstrumentations({
  // PostgreSQL
  '@opentelemetry/instrumentation-pg': {
    // Include query parameters in span
    enhancedDatabaseReporting: true,

    // Limit query size to prevent large attributes
    maxQueryLength: 1000,

    // Custom hook
    responseHook: (span, response) => {
      span.setAttribute('db.rows_affected', response.rowCount);
    }
  },

  // MongoDB
  '@opentelemetry/instrumentation-mongodb': {
    enhancedDatabaseReporting: true,

    // Hook for command start
    responseHook: (span, response) => {
      span.setAttribute('db.documents_affected', response.modifiedCount);
    }
  },

  // Redis
  '@opentelemetry/instrumentation-redis-4': {
    // Include command arguments
    dbStatementSerializer: (cmd, args) => {
      return `${cmd} ${args.join(' ')}`;
    }
  }
});
```

### Logging Instrumentation Options

```typescript
getNodeAutoInstrumentations({
  '@opentelemetry/instrumentation-pino': {
    // Customize log records
    logHook: (span, record, level) => {
      record['resource.service.name'] = 'my-service';
      record['severity'] = level;
    }
  },

  '@opentelemetry/instrumentation-winston': {
    // Customize log records
    logHook: (span, record) => {
      record.traceId = span.spanContext().traceId;
      record.spanId = span.spanContext().spanId;
    }
  }
});
```

## Selective Auto-Instrumentation

### Installing Only What You Need

```bash
# Instead of auto-instrumentations-node (large bundle)
npm install @opentelemetry/instrumentation-http
npm install @opentelemetry/instrumentation-express
npm install @opentelemetry/instrumentation-pg
npm install @opentelemetry/instrumentation-pino
```

```typescript
import { HttpInstrumentation } from '@opentelemetry/instrumentation-http';
import { ExpressInstrumentation } from '@opentelemetry/instrumentation-express';
import { PgInstrumentation } from '@opentelemetry/instrumentation-pg';
import { PinoInstrumentation } from '@opentelemetry/instrumentation-pino';

const sdk = new NodeSDK({
  instrumentations: [
    new HttpInstrumentation({
      ignoreIncomingPaths: ['/health']
    }),
    new ExpressInstrumentation(),
    new PgInstrumentation({
      enhancedDatabaseReporting: true
    }),
    new PinoInstrumentation()
  ]
});
```

### Benefits of Selective Installation

```yaml
auto-instrumentations-node:
  pros:
    - Easy setup
    - Comprehensive coverage
  cons:
    - Large bundle size
    - Unused instrumentations still loaded
    - May instrument unwanted libraries

selective:
  pros:
    - Smaller bundle
    - Only instrument what you use
    - Fine-grained control
  cons:
    - More setup work
    - Must add new instrumentations manually
```

## Enabling/Disabling at Runtime

### Environment Variable Control

```bash
# Disable specific instrumentations via env
OTEL_NODE_DISABLED_INSTRUMENTATIONS=fs,dns,net
```

```typescript
// Read from environment
const disabled = (process.env.OTEL_NODE_DISABLED_INSTRUMENTATIONS || '')
  .split(',')
  .filter(Boolean);

const instrumentations = getNodeAutoInstrumentations({
  ...Object.fromEntries(
    disabled.map(name => [`@opentelemetry/instrumentation-${name}`, { enabled: false }])
  )
});
```

### Feature Flag Control

```typescript
import { FeatureFlags } from './feature-flags';

const instrumentations = getNodeAutoInstrumentations({
  '@opentelemetry/instrumentation-fs': {
    enabled: FeatureFlags.get('otel.instrument.fs', false)
  },
  '@opentelemetry/instrumentation-http': {
    enabled: FeatureFlags.get('otel.instrument.http', true)
  }
});
```

## Auto-Instrumentation Limitations

### What It Can't Do

```yaml
limitations:
  - Custom business logic spans (need manual instrumentation)
  - Custom attributes beyond library-specific hooks
  - Custom span names (only via hooks)
  - Metrics (auto-instrumentation focuses on traces)
  - Logs (only correlation, not structured logging)
  - Custom sampling per operation type
```

### When to Add Manual Instrumentation

```typescript
// Auto-instrumentation creates HTTP span
// But you need business context

async function processOrder(req, res) {
  // HTTP span: "POST /orders" (auto-created)

  // Add manual span for business logic
  return tracer.startActiveSpan('order.process', async (span) => {
    span.setAttribute('order.customer_id', req.body.customerId);
    span.setAttribute('order.items_count', req.body.items.length);

    // Auto-instrumented DB calls appear as children
    const order = await db.createOrder(req.body);

    span.setAttribute('order.id', order.id);
    span.setAttribute('order.total', order.total);

    return order;
  });
}
```

## Common Auto-Instrumentation Patterns

### Express + PostgreSQL

```typescript
const instrumentations = getNodeAutoInstrumentations({
  // Disable noisy instrumentations
  '@opentelemetry/instrumentation-fs': { enabled: false },
  '@opentelemetry/instrumentation-dns': { enabled: false },

  // Configure HTTP
  '@opentelemetry/instrumentation-http': {
    ignoreIncomingPaths: ['/health', '/metrics']
  },

  // Configure Express
  '@opentelemetry/instrumentation-express': {
    ignoreLayers: ['middleware - static']
  },

  // Configure PostgreSQL
  '@opentelemetry/instrumentation-pg': {
    enhancedDatabaseReporting: true
  }
});
```

### Fastify + MongoDB + Redis

```typescript
const instrumentations = getNodeAutoInstrumentations({
  '@opentelemetry/instrumentation-fs': { enabled: false },

  '@opentelemetry/instrumentation-fastify': {
    // Fastify-specific options
  },

  '@opentelemetry/instrumentation-mongodb': {
    enhancedDatabaseReporting: true
  },

  '@opentelemetry/instrumentation-ioredis': {
    // Redis options
  }
});
```

### NestJS + TypeORM

```typescript
const instrumentations = getNodeAutoInstrumentations({
  '@opentelemetry/instrumentation-fs': { enabled: false },

  '@opentelemetry/instrumentation-nestjs-core': {
    // NestJS options
  },

  // TypeORM uses underlying database driver
  '@opentelemetry/instrumentation-pg': {
    enhancedDatabaseReporting: true
  }
});
```

## Debugging Auto-Instrumentation

### Check What's Instrumented

```typescript
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';

const instrumentations = getNodeAutoInstrumentations();

// Log all instrumentation names
instrumentations.forEach(inst => {
  console.log(`Instrumentation: ${inst.instrumentationName}`);
});
```

### Enable Debug Logging

```bash
OTEL_LOG_LEVEL=debug node --import ./instrumentation.js app.js
```

### Verify Instrumentation is Working

```typescript
// Add a test route
app.get('/test-otel', (req, res) => {
  // This should create HTTP + Express spans
  res.json({ status: 'ok' });
});

// Check your backend for:
// - Span: GET /test-otel (http)
// - Span: request handler - /test-otel (express)
```

### Common Issues

```yaml
issue: "No spans appearing"
solutions:
  - Verify --import flag is used
  - Check OTEL_EXPORTER_OTLP_ENDPOINT
  - Ensure SDK started before app code
  - Check sampler isn't dropping everything

issue: "Missing child spans"
solutions:
  - Verify library is supported
  - Check instrumentation isn't disabled
  - Verify library version compatibility
  - Check enhancedDatabaseReporting for DB

issue: "Too many spans"
solutions:
  - Add ignoreIncomingPaths for health checks
  - Disable fs, dns, net instrumentations
  - Configure ignoreLayers for Express
  - Reduce logging instrumentation
```

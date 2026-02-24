# SDK Setup Reference

Detailed guide to OpenTelemetry SDK initialization for Node.js.

## Official Documentation

- [Node.js SDK Documentation](https://opentelemetry.io/docs/languages/js/getting-started/nodejs/)
- [SDK Configuration](https://opentelemetry.io/docs/languages/js/instrumentation/)
- [Environment Variables](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)

## Package Installation

### Core Packages

```bash
# Essential SDK
npm install @opentelemetry/sdk-node

# API (peer dependency, may already be installed)
npm install @opentelemetry/api

# Exporters (choose based on your backend)
npm install @opentelemetry/exporter-trace-otlp-proto    # Traces
npm install @opentelemetry/exporter-metrics-otlp-proto  # Metrics
npm install @opentelemetry/exporter-logs-otlp-proto     # Logs

# Auto-instrumentation
npm install @opentelemetry/auto-instrumentations-node

# Resources
npm install @opentelemetry/resources
npm install @opentelemetry/semantic-conventions
```

### Optional Packages

```bash
# Specific instrumentations (if not using auto-instrumentations-node)
npm install @opentelemetry/instrumentation-http
npm install @opentelemetry/instrumentation-express
npm install @opentelemetry/instrumentation-pg
npm install @opentelemetry/instrumentation-pino

# Console exporters (for debugging)
npm install @opentelemetry/sdk-trace-node
npm install @opentelemetry/sdk-metrics

# Propagators
npm install @opentelemetry/propagator-b3  # If using Zipkin
```

## instrumentation.ts File Pattern

### Complete Example

```typescript
// instrumentation.ts
import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-proto';
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-proto';
import { OTLPLogExporter } from '@opentelemetry/exporter-logs-otlp-proto';
import { PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';
import { BatchLogRecordProcessor } from '@opentelemetry/sdk-logs';
import { Resource } from '@opentelemetry/resources';
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT
} from '@opentelemetry/semantic-conventions';
import {
  ParentBasedSampler,
  TraceIdRatioBasedSampler,
  AlwaysOnSampler
} from '@opentelemetry/sdk-trace-node';

// Determine environment
const isProduction = process.env.NODE_ENV === 'production';

// Configure sampler based on environment
const sampler = isProduction
  ? new ParentBasedSampler({
      root: new TraceIdRatioBasedSampler(0.1) // 10% in prod
    })
  : new AlwaysOnSampler(); // 100% in dev

// Build resource
const resource = new Resource({
  [ATTR_SERVICE_NAME]: process.env.OTEL_SERVICE_NAME || 'my-service',
  [ATTR_SERVICE_VERSION]: process.env.npm_package_version || '0.0.0',
  [ATTR_DEPLOYMENT_ENVIRONMENT]: process.env.NODE_ENV || 'development'
});

// Configure exporters
const traceExporter = new OTLPTraceExporter({
  url: process.env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT ||
       process.env.OTEL_EXPORTER_OTLP_ENDPOINT,
  headers: parseHeaders(process.env.OTEL_EXPORTER_OTLP_HEADERS)
});

const metricExporter = new OTLPMetricExporter({
  url: process.env.OTEL_EXPORTER_OTLP_METRICS_ENDPOINT ||
       process.env.OTEL_EXPORTER_OTLP_ENDPOINT,
  headers: parseHeaders(process.env.OTEL_EXPORTER_OTLP_HEADERS)
});

const logExporter = new OTLPLogExporter({
  url: process.env.OTEL_EXPORTER_OTLP_LOGS_ENDPOINT ||
       process.env.OTEL_EXPORTER_OTLP_ENDPOINT,
  headers: parseHeaders(process.env.OTEL_EXPORTER_OTLP_HEADERS)
});

// Initialize SDK
const sdk = new NodeSDK({
  resource,
  sampler,
  traceExporter,
  metricReader: new PeriodicExportingMetricReader({
    exporter: metricExporter,
    exportIntervalMillis: isProduction ? 60000 : 10000
  }),
  logRecordProcessor: new BatchLogRecordProcessor(logExporter),
  instrumentations: [
    getNodeAutoInstrumentations({
      '@opentelemetry/instrumentation-fs': { enabled: false },
      '@opentelemetry/instrumentation-http': {
        ignoreIncomingPaths: ['/health', '/ready', '/live']
      }
    })
  ]
});

// Start SDK
sdk.start();
console.log('OpenTelemetry SDK initialized');

// Graceful shutdown
const shutdown = async () => {
  console.log('Shutting down OpenTelemetry SDK...');
  try {
    await sdk.shutdown();
    console.log('OpenTelemetry SDK shut down successfully');
  } catch (err) {
    console.error('Error shutting down OpenTelemetry SDK', err);
  }
};

process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);

// Helper to parse header string
function parseHeaders(headerString?: string): Record<string, string> {
  if (!headerString) return {};

  return headerString.split(',').reduce((acc, pair) => {
    const [key, value] = pair.split('=');
    if (key && value) acc[key.trim()] = value.trim();
    return acc;
  }, {} as Record<string, string>);
}
```

## --import Flag Usage

### Why --import?

Node.js loads modules before your code runs. Instrumentation must be in place before those modules load.

```bash
# Correct: Instrumentation loads first
node --import ./instrumentation.js app.js

# Wrong: App loads before instrumentation
node app.js  # With require('./instrumentation') inside app.js
```

### TypeScript with tsx

```bash
# Using tsx loader
node --import tsx --import ./instrumentation.ts app.ts

# Or with tsx directly
tsx --import ./instrumentation.ts app.ts
```

### TypeScript with ts-node

```bash
# Using ts-node loader
node --import ts-node/register --import ./instrumentation.ts app.ts
```

### Package.json Scripts

```json
{
  "scripts": {
    "start": "node --import ./dist/instrumentation.js dist/app.js",
    "dev": "tsx --import ./instrumentation.ts src/app.ts",
    "start:debug": "OTEL_LOG_LEVEL=debug node --import ./dist/instrumentation.js dist/app.js"
  }
}
```

### Docker

```dockerfile
# Dockerfile
FROM node:20-alpine

WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY dist ./dist

# Use --import in CMD
CMD ["node", "--import", "./dist/instrumentation.js", "dist/app.js"]
```

## Environment Variable Configuration

### Standard OTel Environment Variables

```bash
# Service identification
OTEL_SERVICE_NAME=my-service
OTEL_RESOURCE_ATTRIBUTES="service.version=1.0.0,deployment.environment=production"

# Exporter configuration
OTEL_EXPORTER_OTLP_ENDPOINT=https://collector:4317
OTEL_EXPORTER_OTLP_HEADERS="authorization=Bearer token"
OTEL_EXPORTER_OTLP_PROTOCOL=grpc  # or http/protobuf

# Signal-specific endpoints (override general endpoint)
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://traces:4317
OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=https://metrics:4317
OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=https://logs:4317

# Sampling
OTEL_TRACES_SAMPLER=parentbased_traceidratio
OTEL_TRACES_SAMPLER_ARG=0.1

# Propagation
OTEL_PROPAGATORS=tracecontext,baggage

# Debug
OTEL_LOG_LEVEL=info  # debug, info, warn, error
```

### Node.js-Specific Variables

```bash
# Batch span processor
OTEL_BSP_SCHEDULE_DELAY=5000           # Export delay (ms)
OTEL_BSP_MAX_QUEUE_SIZE=2048           # Max queued spans
OTEL_BSP_MAX_EXPORT_BATCH_SIZE=512     # Spans per batch
OTEL_BSP_EXPORT_TIMEOUT=30000          # Export timeout (ms)

# Metric export
OTEL_METRIC_EXPORT_INTERVAL=60000      # Collection interval (ms)
OTEL_METRIC_EXPORT_TIMEOUT=30000       # Export timeout (ms)

# Attribute limits
OTEL_ATTRIBUTE_VALUE_LENGTH_LIMIT=1024 # Max attribute value length
OTEL_ATTRIBUTE_COUNT_LIMIT=128         # Max attributes per span
```

## Resource Attributes Setup

### Automatic Detection

```typescript
import { Resource, detectResources } from '@opentelemetry/resources';
import {
  hostDetector,
  processDetector,
  envDetector
} from '@opentelemetry/resources';

const resource = await detectResources({
  detectors: [
    hostDetector,
    processDetector,
    envDetector
  ]
});
```

### Manual Configuration

```typescript
import { Resource } from '@opentelemetry/resources';
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT,
  ATTR_SERVICE_NAMESPACE,
  ATTR_SERVICE_INSTANCE_ID
} from '@opentelemetry/semantic-conventions';

const resource = new Resource({
  // Required
  [ATTR_SERVICE_NAME]: 'payment-service',

  // Highly recommended
  [ATTR_SERVICE_VERSION]: '2.1.0',
  [ATTR_DEPLOYMENT_ENVIRONMENT]: 'production',

  // Optional but useful
  [ATTR_SERVICE_NAMESPACE]: 'checkout',
  [ATTR_SERVICE_INSTANCE_ID]: process.env.HOSTNAME || 'unknown',

  // Custom attributes
  'team.name': 'platform',
  'cloud.region': 'us-west-2'
});
```

### Merging Resources

```typescript
const detectedResource = await detectResources({ ... });
const manualResource = new Resource({ ... });

const mergedResource = detectedResource.merge(manualResource);
```

## Sampler Configuration

### Available Samplers

```typescript
import {
  AlwaysOnSampler,
  AlwaysOffSampler,
  TraceIdRatioBasedSampler,
  ParentBasedSampler
} from '@opentelemetry/sdk-trace-node';

// Always sample (development)
const alwaysOn = new AlwaysOnSampler();

// Never sample
const alwaysOff = new AlwaysOffSampler();

// Sample 10% of traces
const ratioSampler = new TraceIdRatioBasedSampler(0.1);

// Respect parent decision, default to 10%
const parentBased = new ParentBasedSampler({
  root: new TraceIdRatioBasedSampler(0.1),
  remoteParentSampled: new AlwaysOnSampler(),
  remoteParentNotSampled: new AlwaysOffSampler(),
  localParentSampled: new AlwaysOnSampler(),
  localParentNotSampled: new AlwaysOffSampler()
});
```

### Custom Sampler (Error-Aware)

```typescript
import { Sampler, SamplingDecision, Context, Link, Attributes } from '@opentelemetry/api';

class ErrorAwareSampler implements Sampler {
  private readonly fallback: Sampler;

  constructor(fallbackSampleRate: number) {
    this.fallback = new TraceIdRatioBasedSampler(fallbackSampleRate);
  }

  shouldSample(
    context: Context,
    traceId: string,
    spanName: string,
    spanKind: SpanKind,
    attributes: Attributes,
    links: Link[]
  ): SamplingResult {
    // Always sample if error attribute present
    if (attributes['error'] === true) {
      return { decision: SamplingDecision.RECORD_AND_SAMPLED };
    }

    // Always sample critical operations
    if (spanName.startsWith('payment.') || spanName.startsWith('order.')) {
      return { decision: SamplingDecision.RECORD_AND_SAMPLED };
    }

    // Fallback to ratio sampling
    return this.fallback.shouldSample(
      context, traceId, spanName, spanKind, attributes, links
    );
  }

  toString(): string {
    return 'ErrorAwareSampler';
  }
}
```

## Exporter Protocols

### gRPC (Default, Recommended)

```typescript
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-grpc';

const exporter = new OTLPTraceExporter({
  url: 'https://collector:4317',
  credentials: credentials.createSsl(),
  metadata: new Metadata()
});
```

### HTTP/Protobuf

```typescript
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-proto';

const exporter = new OTLPTraceExporter({
  url: 'https://collector:4318/v1/traces',
  headers: {
    'Authorization': 'Bearer token'
  }
});
```

### HTTP/JSON (Debugging)

```typescript
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';

const exporter = new OTLPTraceExporter({
  url: 'https://collector:4318/v1/traces',
  headers: {}
});
```

### Console (Local Development)

```typescript
import { ConsoleSpanExporter } from '@opentelemetry/sdk-trace-node';

const sdk = new NodeSDK({
  traceExporter: new ConsoleSpanExporter(),
  // ... rest of config
});
```

## Multiple Exporters

### Sending to Multiple Backends

```typescript
import { SimpleSpanProcessor, BatchSpanProcessor } from '@opentelemetry/sdk-trace-node';
import { NodeTracerProvider } from '@opentelemetry/sdk-trace-node';

const provider = new NodeTracerProvider();

// Primary backend (batched for efficiency)
provider.addSpanProcessor(
  new BatchSpanProcessor(
    new OTLPTraceExporter({ url: 'https://primary:4317' })
  )
);

// Secondary backend (also batched)
provider.addSpanProcessor(
  new BatchSpanProcessor(
    new OTLPTraceExporter({ url: 'https://secondary:4317' })
  )
);

// Console for debugging (simple processor for immediate output)
if (process.env.DEBUG_OTEL) {
  provider.addSpanProcessor(
    new SimpleSpanProcessor(new ConsoleSpanExporter())
  );
}

provider.register();
```

## Shutdown Handling

### Graceful Shutdown

```typescript
async function gracefulShutdown(signal: string) {
  console.log(`${signal} received, shutting down gracefully...`);

  // Stop accepting new requests
  server.close();

  try {
    // Flush and shut down OTel (this exports pending telemetry)
    await sdk.shutdown();
    console.log('OTel SDK shut down successfully');
  } catch (err) {
    console.error('Error during OTel shutdown', err);
  }

  process.exit(0);
}

process.on('SIGTERM', () => gracefulShutdown('SIGTERM'));
process.on('SIGINT', () => gracefulShutdown('SIGINT'));
```

### Force Flush (for Testing)

```typescript
import { trace } from '@opentelemetry/api';

// Force export all pending spans
await trace.getTracerProvider().forceFlush();
```

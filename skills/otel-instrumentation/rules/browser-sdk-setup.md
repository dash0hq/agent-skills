---
title: "Browser SDK Setup"
impact: HIGH
tags:
  - browser
  - sdk
  - web
  - frontend
  - react
---

# Browser SDK Setup Reference

Detailed guide to configuring the OpenTelemetry Web SDK.

## Official Documentation

- [Browser SDK](https://opentelemetry.io/docs/languages/js/getting-started/browser/)
- [WebTracerProvider](https://open-telemetry.github.io/opentelemetry-js/classes/_opentelemetry_sdk_trace_web.WebTracerProvider.html)
- [Context Managers](https://opentelemetry.io/docs/languages/js/context/)

## Package Installation

```bash
# Core packages
npm install @opentelemetry/api \
  @opentelemetry/sdk-trace-web \
  @opentelemetry/sdk-trace-base \
  @opentelemetry/resources \
  @opentelemetry/semantic-conventions

# Exporter (use HTTP for browsers, not gRPC)
npm install @opentelemetry/exporter-trace-otlp-http

# Context manager (required for async context)
npm install @opentelemetry/context-zone
# OR for lighter weight (no Zone.js):
npm install @opentelemetry/context-async-hooks  # Node.js only
```

## Basic Provider Setup

```typescript
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { BatchSpanProcessor, SimpleSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { ZoneContextManager } from '@opentelemetry/context-zone';
import { Resource } from '@opentelemetry/resources';
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT
} from '@opentelemetry/semantic-conventions';

// 1. Create resource with service info
const resource = new Resource({
  [ATTR_SERVICE_NAME]: 'my-frontend',
  [ATTR_SERVICE_VERSION]: '1.0.0',
  [ATTR_DEPLOYMENT_ENVIRONMENT]: process.env.NODE_ENV || 'development',

  // Browser-specific attributes
  'browser.language': navigator.language,
  'browser.user_agent': navigator.userAgent,
  'session.id': getOrCreateSessionId()
});

// 2. Create provider
const provider = new WebTracerProvider({
  resource
});

// 3. Configure exporter
const exporter = new OTLPTraceExporter({
  url: 'https://ingress.eu-west-1.dash0.com:4318/v1/traces',
  headers: {
    'Authorization': `Bearer ${DASH0_AUTH_TOKEN}`
  }
});

// 4. Add span processor
provider.addSpanProcessor(new BatchSpanProcessor(exporter, {
  maxQueueSize: 100,
  maxExportBatchSize: 30,
  scheduledDelayMillis: 1000,
  exportTimeoutMillis: 30000
}));

// 5. Register with context manager
provider.register({
  contextManager: new ZoneContextManager()
});
```

## Context Managers

### ZoneContextManager (Recommended)

Uses Zone.js for context propagation. Works with all async patterns.

```typescript
import { ZoneContextManager } from '@opentelemetry/context-zone';

provider.register({
  contextManager: new ZoneContextManager()
});
```

**Note**: Zone.js adds ~15KB gzipped. Angular apps already include it.

### StackContextManager (Lighter)

Lighter alternative but limited async support.

```typescript
import { StackContextManager } from '@opentelemetry/sdk-trace-web';

provider.register({
  contextManager: new StackContextManager()
});
```

**Limitation**: Context may be lost across certain async boundaries.

## Exporters

### OTLP HTTP Exporter (Recommended)

```typescript
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';

const exporter = new OTLPTraceExporter({
  url: 'https://your-collector:4318/v1/traces',
  headers: {
    'Authorization': 'Bearer token'
  },
  // Optional: compression
  compression: 'gzip'
});
```

### Console Exporter (Development)

```typescript
import { ConsoleSpanExporter } from '@opentelemetry/sdk-trace-base';

// For development debugging
if (process.env.NODE_ENV === 'development') {
  provider.addSpanProcessor(new SimpleSpanProcessor(new ConsoleSpanExporter()));
}
```

### Multiple Exporters

```typescript
// Console for debugging + OTLP for backend
provider.addSpanProcessor(new SimpleSpanProcessor(new ConsoleSpanExporter()));
provider.addSpanProcessor(new BatchSpanProcessor(new OTLPTraceExporter({
  url: 'https://ingress.eu-west-1.dash0.com:4318/v1/traces',
  headers: { 'Authorization': `Bearer ${DASH0_AUTH_TOKEN}` }
})));
```

## Span Processors

### BatchSpanProcessor (Production)

Batches spans before export to reduce network overhead.

```typescript
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';

provider.addSpanProcessor(new BatchSpanProcessor(exporter, {
  // Max spans to queue before dropping
  maxQueueSize: 100,          // Default: 2048 (reduce for browsers)

  // Max spans per export batch
  maxExportBatchSize: 30,     // Default: 512 (reduce for browsers)

  // How often to export (ms)
  scheduledDelayMillis: 1000, // Default: 5000

  // Export timeout (ms)
  exportTimeoutMillis: 30000  // Default: 30000
}));
```

### SimpleSpanProcessor (Development)

Exports each span immediately. Higher overhead but useful for debugging.

```typescript
import { SimpleSpanProcessor } from '@opentelemetry/sdk-trace-base';

provider.addSpanProcessor(new SimpleSpanProcessor(exporter));
```

## Sampling

### Always Sample (Development)

```typescript
import { AlwaysOnSampler } from '@opentelemetry/sdk-trace-base';

const provider = new WebTracerProvider({
  resource,
  sampler: new AlwaysOnSampler()
});
```

### Ratio-Based Sampling (Production)

```typescript
import { TraceIdRatioBasedSampler } from '@opentelemetry/sdk-trace-base';

const provider = new WebTracerProvider({
  resource,
  sampler: new TraceIdRatioBasedSampler(0.1) // 10% of traces
});
```

### Parent-Based Sampling (Recommended)

Respects sampling decision from server (if trace started there).

```typescript
import { ParentBasedSampler, TraceIdRatioBasedSampler, AlwaysOnSampler } from '@opentelemetry/sdk-trace-base';

const provider = new WebTracerProvider({
  resource,
  sampler: new ParentBasedSampler({
    // If parent is sampled, sample this
    // If no parent, use this sampler for new traces
    root: new TraceIdRatioBasedSampler(0.1),

    // Optional: different behavior for remote parents
    remoteParentSampled: new AlwaysOnSampler(),
    remoteParentNotSampled: new TraceIdRatioBasedSampler(0.01)
  })
});
```

## Resource Attributes

### Standard Attributes

```typescript
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT
} from '@opentelemetry/semantic-conventions';

const resource = new Resource({
  // Required
  [ATTR_SERVICE_NAME]: 'my-frontend',

  // Recommended
  [ATTR_SERVICE_VERSION]: '1.0.0',
  [ATTR_DEPLOYMENT_ENVIRONMENT]: 'production'
});
```

### Browser-Specific Attributes

```typescript
const resource = new Resource({
  [ATTR_SERVICE_NAME]: 'my-frontend',

  // Session tracking
  'session.id': getOrCreateSessionId(),

  // Browser info (be careful with fingerprinting)
  'browser.language': navigator.language,
  'browser.platform': navigator.platform,
  'browser.viewport.width': window.innerWidth,
  'browser.viewport.height': window.innerHeight,

  // App info
  'app.version': APP_VERSION,
  'app.build': BUILD_NUMBER
});

function getOrCreateSessionId(): string {
  let sessionId = sessionStorage.getItem('otel_session_id');
  if (!sessionId) {
    sessionId = crypto.randomUUID();
    sessionStorage.setItem('otel_session_id', sessionId);
  }
  return sessionId;
}
```

## Initialization Timing

### Before App Load (Critical)

```typescript
// index.ts - FIRST import
import './instrumentation';

// Then app code
import { createRoot } from 'react-dom/client';
import App from './App';

createRoot(document.getElementById('root')!).render(<App />);
```

### With Vite

```typescript
// vite.config.ts
export default defineConfig({
  // Ensure instrumentation is bundled first
  optimizeDeps: {
    include: ['./src/instrumentation']
  }
});
```

### With Webpack

```typescript
// webpack.config.js
module.exports = {
  entry: {
    instrumentation: './src/instrumentation.ts',
    main: './src/index.tsx'
  }
};
```

## Environment-Based Configuration

```typescript
interface TelemetryConfig {
  endpoint: string;
  sampleRate: number;
  debug: boolean;
}

function getConfig(): TelemetryConfig {
  const env = process.env.NODE_ENV;

  if (env === 'production') {
    return {
      endpoint: 'https://ingress.eu-west-1.dash0.com:4318/v1/traces',
      sampleRate: 0.1,  // 10%
      debug: false
    };
  }

  if (env === 'staging') {
    return {
      endpoint: 'https://ingress.eu-west-1.dash0.com:4318/v1/traces',
      sampleRate: 0.5,  // 50%
      debug: false
    };
  }

  // Development
  return {
    endpoint: 'http://localhost:4318/v1/traces',
    sampleRate: 1.0,  // 100%
    debug: true
  };
}

const config = getConfig();

const provider = new WebTracerProvider({
  resource,
  sampler: new TraceIdRatioBasedSampler(config.sampleRate)
});

if (config.debug) {
  provider.addSpanProcessor(new SimpleSpanProcessor(new ConsoleSpanExporter()));
}

provider.addSpanProcessor(new BatchSpanProcessor(new OTLPTraceExporter({
  url: config.endpoint,
  headers: { 'Authorization': `Bearer ${DASH0_AUTH_TOKEN}` }
})));
```

## Shutdown

Handle page unload to flush pending spans:

```typescript
// Flush on page unload
window.addEventListener('beforeunload', () => {
  // Use sendBeacon for reliability during unload
  provider.forceFlush();
});

// For SPAs, also handle visibility change
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'hidden') {
    provider.forceFlush();
  }
});
```

## Complete Setup Example

```typescript
// instrumentation.ts
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { BatchSpanProcessor, SimpleSpanProcessor, ConsoleSpanExporter } from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { ZoneContextManager } from '@opentelemetry/context-zone';
import { Resource } from '@opentelemetry/resources';
import { ParentBasedSampler, TraceIdRatioBasedSampler } from '@opentelemetry/sdk-trace-base';
import { ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION, ATTR_DEPLOYMENT_ENVIRONMENT } from '@opentelemetry/semantic-conventions';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { UserInteractionInstrumentation } from '@opentelemetry/instrumentation-user-interaction';

const IS_PROD = process.env.NODE_ENV === 'production';
const DASH0_AUTH_TOKEN = process.env.DASH0_AUTH_TOKEN;

function getSessionId(): string {
  let id = sessionStorage.getItem('session_id');
  if (!id) {
    id = crypto.randomUUID();
    sessionStorage.setItem('session_id', id);
  }
  return id;
}

export function initTelemetry() {
  const resource = new Resource({
    [ATTR_SERVICE_NAME]: 'my-frontend',
    [ATTR_SERVICE_VERSION]: process.env.APP_VERSION || '0.0.0',
    [ATTR_DEPLOYMENT_ENVIRONMENT]: process.env.NODE_ENV || 'development',
    'session.id': getSessionId()
  });

  const provider = new WebTracerProvider({
    resource,
    sampler: new ParentBasedSampler({
      root: new TraceIdRatioBasedSampler(IS_PROD ? 0.1 : 1.0)
    })
  });

  // Console exporter for development
  if (!IS_PROD) {
    provider.addSpanProcessor(new SimpleSpanProcessor(new ConsoleSpanExporter()));
  }

  // OTLP exporter for all environments
  if (DASH0_AUTH_TOKEN) {
    provider.addSpanProcessor(new BatchSpanProcessor(
      new OTLPTraceExporter({
        url: 'https://ingress.eu-west-1.dash0.com:4318/v1/traces',
        headers: { 'Authorization': `Bearer ${DASH0_AUTH_TOKEN}` }
      }),
      {
        maxQueueSize: IS_PROD ? 100 : 50,
        maxExportBatchSize: IS_PROD ? 30 : 10,
        scheduledDelayMillis: IS_PROD ? 1000 : 500
      }
    ));
  }

  provider.register({
    contextManager: new ZoneContextManager()
  });

  registerInstrumentations({
    instrumentations: [
      new DocumentLoadInstrumentation(),
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [/api\.yoursite\.com/],
        ignoreUrls: [/analytics/, /sentry/]
      }),
      new UserInteractionInstrumentation({
        eventNames: ['click', 'submit']
      })
    ]
  });

  // Flush on page hide
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') {
      provider.forceFlush();
    }
  });

  console.log('Telemetry initialized');
}
```

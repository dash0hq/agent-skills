---
name: '@otel-browser'
description: OpenTelemetry implementation for browser/client-side applications. Use when setting up Real User Monitoring (RUM), tracking user interactions, measuring Web Vitals, or correlating frontend traces with backend services. Triggers on browser observability, client-side tracing, React/Vue/Angular instrumentation, or frontend performance monitoring.
license: MIT
metadata:
  author: dash0
  version: '1.0.0'
  requires:
    - '@otel-telemetry'
---

# OpenTelemetry for Browsers

This skill provides implementation guidance for OpenTelemetry in browser/client-side applications using TypeScript.

> **Prerequisites**: Review `@otel-telemetry` for core concepts before implementing.

## Official Documentation

- [OpenTelemetry JavaScript (Browser)](https://opentelemetry.io/docs/languages/js/getting-started/browser/)
- [Web SDK API Reference](https://open-telemetry.github.io/opentelemetry-js/modules/_opentelemetry_sdk_trace_web.html)
- [Browser Instrumentations](https://github.com/open-telemetry/opentelemetry-js-contrib/tree/main/plugins/web)
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)

## Why Browser Telemetry?

```
User clicks button → Frontend span → API call → Backend span → Database span
       ↑                                              ↓
       └──────────── Same trace ID ──────────────────┘
```

Browser telemetry lets you:
- **See the full picture**: Trace from user click to database and back
- **Measure real user experience**: Not synthetic tests, actual users
- **Identify frontend bottlenecks**: Slow renders, long tasks, layout shifts
- **Correlate errors**: Connect frontend errors to backend failures

## Core Packages

```bash
# Minimal setup
npm install @opentelemetry/api \
  @opentelemetry/sdk-trace-web \
  @opentelemetry/exporter-trace-otlp-http \
  @opentelemetry/context-zone \
  @opentelemetry/instrumentation

# Auto-instrumentation
npm install @opentelemetry/instrumentation-document-load \
  @opentelemetry/instrumentation-fetch \
  @opentelemetry/instrumentation-xml-http-request \
  @opentelemetry/instrumentation-user-interaction
```

## Quick Start

### Basic Setup

Create `instrumentation.ts`:

```typescript
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { ZoneContextManager } from '@opentelemetry/context-zone';
import { Resource } from '@opentelemetry/resources';
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT
} from '@opentelemetry/semantic-conventions';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { UserInteractionInstrumentation } from '@opentelemetry/instrumentation-user-interaction';

export function initTelemetry() {
  const resource = new Resource({
    [ATTR_SERVICE_NAME]: 'my-frontend',
    [ATTR_SERVICE_VERSION]: '1.0.0',
    [ATTR_DEPLOYMENT_ENVIRONMENT]: process.env.NODE_ENV || 'development'
  });

  const provider = new WebTracerProvider({ resource });

  // Export to Dash0
  const exporter = new OTLPTraceExporter({
    url: 'https://ingress.eu-west-1.dash0.com:4318/v1/traces',
    headers: {
      'Authorization': `Bearer ${process.env.DASH0_AUTH_TOKEN}`
    }
  });

  provider.addSpanProcessor(new BatchSpanProcessor(exporter));

  // ZoneContextManager enables context propagation across async operations
  provider.register({
    contextManager: new ZoneContextManager()
  });

  // Register auto-instrumentations
  registerInstrumentations({
    instrumentations: [
      new DocumentLoadInstrumentation(),
      new FetchInstrumentation({
        // Propagate trace context to backend
        propagateTraceHeaderCorsUrls: [
          /https:\/\/api\.yoursite\.com.*/,
          /https:\/\/.*\.yoursite\.com\/api.*/
        ]
      }),
      new UserInteractionInstrumentation()
    ]
  });

  console.log('OpenTelemetry browser instrumentation initialized');
}
```

### Initialize Early

```typescript
// index.tsx or main.ts - BEFORE your app code
import { initTelemetry } from './instrumentation';

initTelemetry();

// Now import and render your app
import { createRoot } from 'react-dom/client';
import App from './App';

createRoot(document.getElementById('root')!).render(<App />);
```

## Server-to-Client Correlation

The key to full-stack tracing is connecting browser traces with server traces.

### Option 1: Fetch Header Propagation (Recommended)

**The simplest approach.** Configure which API URLs should receive trace headers, and the browser automatically sends `traceparent` to your backend on every fetch request.

```typescript
new FetchInstrumentation({
  // CRITICAL: Configure which URLs receive trace headers
  propagateTraceHeaderCorsUrls: [
    /https:\/\/api\.yoursite\.com.*/,
    /http:\/\/localhost:\d+\/api.*/
  ]
})
```

**What happens:** Every matching fetch request includes:
```
traceparent: 00-abc123def456789012345678901234-9876543210abcdef-01
```

**Server requirement:** Allow these headers in CORS:
```typescript
app.use(cors({
  allowedHeaders: ['traceparent', 'tracestate', 'Content-Type', 'Authorization']
}));
```

The server's OTel SDK automatically extracts this context and continues the trace. **No extra code needed on either side.**

### Option 2: Meta Tag Injection (For SSR Page Load)

For server-rendered apps where you want to correlate the **initial HTML page load** with browser hydration. Server injects trace context into HTML:

```html
<!-- Server-rendered HTML -->
<html>
<head>
  <meta name="traceparent" content="00-abc123def456-789xyz-01" />
  <meta name="tracestate" content="dash0=..." />
</head>
```

Browser reads and continues the trace:

```typescript
import { context, trace } from '@opentelemetry/api';
import { W3CTraceContextPropagator } from '@opentelemetry/core';

function getServerTraceContext(): context.Context | undefined {
  const traceparent = document.querySelector('meta[name="traceparent"]')?.getAttribute('content');
  const tracestate = document.querySelector('meta[name="tracestate"]')?.getAttribute('content');

  if (!traceparent) return undefined;

  const carrier = { traceparent, tracestate: tracestate || '' };
  const propagator = new W3CTraceContextPropagator();

  return propagator.extract(context.active(), carrier, {
    get: (carrier, key) => carrier[key as keyof typeof carrier],
    keys: (carrier) => Object.keys(carrier)
  });
}

// Use the server context as parent for browser spans
const serverContext = getServerTraceContext();
if (serverContext) {
  const tracer = trace.getTracer('my-frontend');

  context.with(serverContext, () => {
    tracer.startActiveSpan('page.interactive', (span) => {
      // This span is now connected to the server trace!
      span.setAttribute('page.url', window.location.href);
      span.end();
    });
  });
}
```

### Option 3: Server-Timing Header

Server sends trace context in response headers:

```
Server-Timing: traceparent;desc="00-abc123-def456-01"
```

Browser extracts from navigation timing:

```typescript
function getTraceFromServerTiming(): string | null {
  const entries = performance.getEntriesByType('navigation') as PerformanceNavigationTiming[];
  const navEntry = entries[0];

  if (navEntry?.serverTiming) {
    const traceTiming = navEntry.serverTiming.find(t => t.name === 'traceparent');
    return traceTiming?.description || null;
  }
  return null;
}
```

### Option 4: Initial Data Payload

For SPAs loading data via API:

```typescript
// Server response includes trace context
{
  "data": { ... },
  "_trace": {
    "traceparent": "00-abc123-def456-01",
    "tracestate": "dash0=..."
  }
}

// Browser continues the trace
const response = await fetch('/api/initial-data');
const { data, _trace } = await response.json();

if (_trace?.traceparent) {
  // Continue this trace for subsequent operations
}
```

> **Note:** Most apps only need **Option 1** (Fetch Header Propagation). The other options are for specific scenarios like correlating SSR page loads or initial data fetches.

## Auto-Instrumentation

### What Gets Instrumented

| Instrumentation | What It Captures |
|-----------------|------------------|
| `document-load` | Page load timing, resource loading, DOM events |
| `fetch` | Fetch API calls with request/response details |
| `xml-http-request` | XHR calls (legacy APIs) |
| `user-interaction` | Clicks, form submissions, user actions |

### Document Load Instrumentation

Captures the full page load waterfall:

```typescript
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';

new DocumentLoadInstrumentation({
  // Add custom attributes
  applyCustomAttributesOnSpan: {
    documentLoad: (span) => {
      span.setAttribute('page.title', document.title);
      span.setAttribute('page.url', window.location.href);
    },
    documentFetch: (span) => {
      span.setAttribute('document.referrer', document.referrer);
    },
    resourceFetch: (span, resource) => {
      span.setAttribute('resource.type', resource.initiatorType);
    }
  }
});
```

Creates spans for:
- `documentLoad` - Full page load
- `documentFetch` - HTML document fetch
- `resourceFetch` - Each resource (JS, CSS, images)

### Fetch Instrumentation

```typescript
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';

new FetchInstrumentation({
  // Which URLs to add trace headers to
  propagateTraceHeaderCorsUrls: [
    /https:\/\/api\.yoursite\.com.*/,
    'https://other-service.com/api'
  ],

  // Clear timing resources to avoid memory leaks
  clearTimingResources: true,

  // Add custom attributes
  applyCustomAttributesOnSpan: (span, request, response) => {
    span.setAttribute('http.request_id', response.headers.get('x-request-id'));
  },

  // Ignore certain URLs
  ignoreUrls: [
    /\/health/,
    /\/analytics/,
    /google-analytics\.com/
  ]
});
```

### User Interaction Instrumentation

```typescript
import { UserInteractionInstrumentation } from '@opentelemetry/instrumentation-user-interaction';

new UserInteractionInstrumentation({
  // Which events to track
  eventNames: ['click', 'submit', 'change'],

  // Only track elements with data-track attribute
  shouldPreventSpanCreation: (eventType, element) => {
    return !element.hasAttribute('data-track');
  }
});
```

Usage in HTML:

```html
<button data-track="true" data-track-name="checkout">
  Complete Purchase
</button>
```

## Manual Instrumentation

### Creating Custom Spans

```typescript
import { trace, SpanStatusCode } from '@opentelemetry/api';

const tracer = trace.getTracer('my-frontend', '1.0.0');

// Track a user action
async function handleCheckout(cart: Cart) {
  return tracer.startActiveSpan('checkout.process', async (span) => {
    try {
      span.setAttribute('cart.items_count', cart.items.length);
      span.setAttribute('cart.total', cart.total);

      // Validate cart
      span.addEvent('validation.start');
      await validateCart(cart);
      span.addEvent('validation.complete');

      // Process payment
      const payment = await processPayment(cart);
      span.setAttribute('payment.method', payment.method);

      return payment;
    } catch (error) {
      span.recordException(error as Error);
      span.setStatus({
        code: SpanStatusCode.ERROR,
        message: (error as Error).message
      });
      throw error;
    } finally {
      span.end();
    }
  });
}
```

### Tracking Route Changes (SPA)

```typescript
// React Router example
import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { trace } from '@opentelemetry/api';

const tracer = trace.getTracer('router');

export function useRouteTracking() {
  const location = useLocation();

  useEffect(() => {
    const span = tracer.startSpan('route.change', {
      attributes: {
        'route.path': location.pathname,
        'route.search': location.search,
        'route.hash': location.hash
      }
    });

    // End span when route is fully loaded
    // Could use React.Profiler or custom logic
    requestIdleCallback(() => {
      span.end();
    });

    return () => {
      if (span.isRecording()) {
        span.end();
      }
    };
  }, [location]);
}
```

### Tracking Component Renders

```typescript
import { trace } from '@opentelemetry/api';
import { Profiler, ProfilerOnRenderCallback } from 'react';

const tracer = trace.getTracer('react');

const onRenderCallback: ProfilerOnRenderCallback = (
  id,
  phase,
  actualDuration,
  baseDuration,
  startTime,
  commitTime
) => {
  const span = tracer.startSpan('react.render', {
    startTime: startTime,
    attributes: {
      'react.component': id,
      'react.phase': phase,
      'react.actual_duration_ms': actualDuration,
      'react.base_duration_ms': baseDuration
    }
  });
  span.end(commitTime);
};

// Wrap components
<Profiler id="ProductList" onRender={onRenderCallback}>
  <ProductList products={products} />
</Profiler>
```

## Web Vitals Integration

Track Core Web Vitals alongside traces:

```typescript
import { onCLS, onFID, onLCP, onFCP, onTTFB, onINP } from 'web-vitals';
import { trace, metrics } from '@opentelemetry/api';

const meter = metrics.getMeter('web-vitals');
const tracer = trace.getTracer('web-vitals');

// Create metrics for each vital
const lcpHistogram = meter.createHistogram('web_vital.lcp', {
  description: 'Largest Contentful Paint',
  unit: 'ms'
});

const clsHistogram = meter.createHistogram('web_vital.cls', {
  description: 'Cumulative Layout Shift',
  unit: '1'
});

const inpHistogram = meter.createHistogram('web_vital.inp', {
  description: 'Interaction to Next Paint',
  unit: 'ms'
});

// Record vitals
function recordVital(name: string, value: number, histogram: any) {
  const attributes = {
    'page.url': window.location.href,
    'page.path': window.location.pathname
  };

  histogram.record(value, attributes);

  // Also create a span for correlation
  const span = tracer.startSpan(`web_vital.${name}`);
  span.setAttribute('vital.name', name);
  span.setAttribute('vital.value', value);
  span.end();
}

onLCP((metric) => recordVital('lcp', metric.value, lcpHistogram));
onCLS((metric) => recordVital('cls', metric.value, clsHistogram));
onINP((metric) => recordVital('inp', metric.value, inpHistogram));
```

## Session and User Context

### Adding Session ID

```typescript
function getOrCreateSessionId(): string {
  let sessionId = sessionStorage.getItem('otel_session_id');
  if (!sessionId) {
    sessionId = crypto.randomUUID();
    sessionStorage.setItem('otel_session_id', sessionId);
  }
  return sessionId;
}

const resource = new Resource({
  [ATTR_SERVICE_NAME]: 'my-frontend',
  'session.id': getOrCreateSessionId(),
  'browser.name': navigator.userAgent,
  'browser.language': navigator.language
});
```

### Adding User Context (After Auth)

```typescript
import { trace } from '@opentelemetry/api';

function setUserContext(userId: string, userTier: string) {
  // Add to all future spans
  const tracer = trace.getTracer('my-frontend');

  // Store for use in span creation
  window.__otel_user = { userId, userTier };
}

// Use in custom spans
tracer.startActiveSpan('user.action', (span) => {
  if (window.__otel_user) {
    span.setAttribute('user.id', window.__otel_user.userId);
    span.setAttribute('user.tier', window.__otel_user.tier);
  }
  // ...
});
```

## Error Tracking

### Global Error Handler

```typescript
import { trace, SpanStatusCode } from '@opentelemetry/api';

const tracer = trace.getTracer('error-handler');

window.addEventListener('error', (event) => {
  const span = tracer.startSpan('error.uncaught');

  span.recordException({
    name: event.error?.name || 'Error',
    message: event.message,
    stack: event.error?.stack
  });

  span.setStatus({ code: SpanStatusCode.ERROR, message: event.message });
  span.setAttribute('error.filename', event.filename);
  span.setAttribute('error.lineno', event.lineno);
  span.setAttribute('error.colno', event.colno);
  span.end();
});

window.addEventListener('unhandledrejection', (event) => {
  const span = tracer.startSpan('error.unhandled_rejection');

  span.recordException({
    name: 'UnhandledRejection',
    message: String(event.reason)
  });

  span.setStatus({ code: SpanStatusCode.ERROR });
  span.end();
});
```

### React Error Boundary

```typescript
import { Component, ErrorInfo, ReactNode } from 'react';
import { trace, SpanStatusCode } from '@opentelemetry/api';

const tracer = trace.getTracer('react-error-boundary');

interface Props {
  children: ReactNode;
  fallback: ReactNode;
}

class TracedErrorBoundary extends Component<Props, { hasError: boolean }> {
  state = { hasError: false };

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    const span = tracer.startSpan('react.error_boundary');

    span.recordException(error);
    span.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
    span.setAttribute('react.component_stack', errorInfo.componentStack || '');
    span.end();
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback;
    }
    return this.props.children;
  }
}
```

## Dash0 Integration

### Direct Browser Export

```typescript
const exporter = new OTLPTraceExporter({
  url: 'https://ingress.eu-west-1.dash0.com:4318/v1/traces',
  headers: {
    'Authorization': `Bearer ${DASH0_AUTH_TOKEN}`
  }
});
```

### Through Your Backend (Recommended for Production)

To avoid exposing your auth token in the browser:

```typescript
// Browser sends to your backend
const exporter = new OTLPTraceExporter({
  url: '/api/telemetry/traces'  // Your backend endpoint
});

// Your backend proxies to Dash0
// backend/api/telemetry/traces.ts
export async function POST(request: Request) {
  const body = await request.arrayBuffer();

  const response = await fetch('https://ingress.eu-west-1.dash0.com:4318/v1/traces', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/x-protobuf',
      'Authorization': `Bearer ${process.env.DASH0_AUTH_TOKEN}`
    },
    body
  });

  return new Response(null, { status: response.status });
}
```

### Viewing in Dash0

In Dash0, you'll see:
- Frontend service (`my-frontend`) and backend services in the same trace
- User interaction → API call → Backend processing → Database
- Full request timeline from click to response

Filter by:
- `service.name = "my-frontend"` for browser traces
- `session.id` to follow a user's journey
- `web_vital.*` metrics for performance

## Browser-Specific Considerations

### Bundle Size

OTel adds ~30-50KB gzipped. Optimize by:

```typescript
// Import only what you need
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
// NOT: import * as opentelemetry from '@opentelemetry/api';

// Use dynamic imports for non-critical instrumentations
if (process.env.NODE_ENV === 'development') {
  import('@opentelemetry/instrumentation-long-task').then(({ LongTaskInstrumentation }) => {
    // Register for dev only
  });
}
```

### Sampling for High-Traffic Sites

```typescript
import { ParentBasedSampler, TraceIdRatioBasedSampler } from '@opentelemetry/sdk-trace-base';

const provider = new WebTracerProvider({
  resource,
  sampler: new ParentBasedSampler({
    // If parent is sampled, sample this too
    // Otherwise, sample 10% of new traces
    root: new TraceIdRatioBasedSampler(0.1)
  })
});
```

### Privacy Considerations

```typescript
// Don't capture PII in URLs
new FetchInstrumentation({
  applyCustomAttributesOnSpan: (span, request) => {
    // Sanitize URL - remove query params that might contain PII
    const url = new URL(request.url);
    url.search = ''; // Remove all query params
    span.setAttribute('http.url', url.toString());
  }
});

// Don't capture user input values
new UserInteractionInstrumentation({
  shouldPreventSpanCreation: (eventType, element) => {
    // Don't track password fields
    if (element.type === 'password') return true;
    return false;
  }
});
```

## Common Issues

### Context Not Propagating to Backend

```typescript
// Make sure CORS URLs are configured
new FetchInstrumentation({
  propagateTraceHeaderCorsUrls: [
    /your-api-domain\.com/  // Must match your API
  ]
});

// Backend must accept the traceparent header
// CORS config on backend:
// Access-Control-Allow-Headers: traceparent, tracestate
```

### Spans Not Appearing

```typescript
// 1. Check exporter URL is correct (HTTP, not gRPC for browsers)
url: 'https://....:4318/v1/traces'  // HTTP port, not 4317

// 2. Check CORS allows your origin on Dash0/collector

// 3. Verify ZoneContextManager is registered
provider.register({
  contextManager: new ZoneContextManager()
});
```

### High Memory Usage

```typescript
// Limit batch size
provider.addSpanProcessor(new BatchSpanProcessor(exporter, {
  maxQueueSize: 100,        // Default is 2048
  maxExportBatchSize: 30,   // Default is 512
  scheduledDelayMillis: 1000
}));
```

## Reference Documents

- [SDK Setup Reference](./references/sdk-setup.md)
- [Auto-Instrumentation Reference](./references/auto-instrumentation.md)
- [Manual Instrumentation Reference](./references/manual-instrumentation.md)
- [Server Correlation Reference](./references/server-correlation.md)

## Examples

- [React SPA Example](./examples/react-spa.md)
- [Next.js Client Example](./examples/nextjs-client.md)

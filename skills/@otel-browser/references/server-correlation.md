# Server-to-Browser Correlation Reference

Guide to connecting frontend traces with backend traces for full-stack observability.

## Why Correlation Matters

Without correlation:
```
Browser: page.load ─────── 500ms     ← Can't connect these
Server:  GET /api/data ──── 450ms    ← Separate traces
```

With correlation:
```
Browser: page.load ─────────────────────────── 500ms
  └─ Browser: fetch /api/data ──────────────── 480ms
      └─ Server: GET /api/data ─────────────── 450ms
          └─ Server: db.query ──────────────── 400ms
```

Same trace ID links everything together.

## Correlation Methods

### Method 1: Fetch Header Propagation (Recommended)

**The simplest and most common approach.** The browser automatically adds `traceparent` header to API calls, and the server continues the trace.

This works automatically with the Fetch Instrumentation - no extra code needed on the browser side.

#### Browser Setup

```typescript
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';

new FetchInstrumentation({
  // CRITICAL: List all domains that should receive trace headers
  propagateTraceHeaderCorsUrls: [
    // Your primary API
    /https:\/\/api\.yoursite\.com.*/,

    // Subdomains
    /https:\/\/.*\.yoursite\.com\/api.*/,

    // Specific partner APIs
    'https://partner-api.example.com/v1',

    // Local development
    /http:\/\/localhost:\d+\/api.*/
  ]
});
```

#### What Happens

When a fetch URL matches, these headers are automatically added:

```
traceparent: 00-abc123def456789012345678901234-9876543210abcdef-01
tracestate: dash0=...
```

The server's OTel SDK automatically extracts this context and continues the trace.

#### Server CORS Configuration

Your backend MUST allow these headers:

```typescript
// Express CORS
import cors from 'cors';

app.use(cors({
  origin: ['https://yoursite.com', 'http://localhost:3000'],
  allowedHeaders: ['Content-Type', 'Authorization', 'traceparent', 'tracestate'],
  exposedHeaders: ['traceparent', 'tracestate'] // Optional: for response context
}));
```

#### Result

```
Browser: page.load ─────────────────────────────────── 500ms
  └─ Browser: fetch /api/products ──────────────────── 450ms
      └─ Server: GET /api/products ─────────────────── 440ms  ← Same trace!
          └─ Server: db.query ──────────────────────── 380ms
```

**Why this is recommended:**
- Zero browser-side code beyond configuration
- Works with any backend that has OTel instrumentation
- Standard W3C Trace Context format
- Works for SPAs, static sites, and SSR apps

---

### Method 2: Trace Context in HTML (For SSR Page Load)

For server-rendered apps where you want the **initial page load** to be part of the server trace. The server injects trace context into HTML, and the browser continues that specific trace.

**Use when:** You want to correlate the HTML document fetch with browser hydration.

#### Server Side (Node.js/Express)

```typescript
import { trace, context, propagation } from '@opentelemetry/api';

app.get('*', (req, res) => {
  const span = trace.getSpan(context.active());
  const spanContext = span?.spanContext();

  let traceparent = '';
  let tracestate = '';

  if (spanContext) {
    traceparent = `00-${spanContext.traceId}-${spanContext.spanId}-0${spanContext.traceFlags}`;
    const carrier: Record<string, string> = {};
    propagation.inject(context.active(), carrier);
    tracestate = carrier['tracestate'] || '';
  }

  res.send(`
    <!DOCTYPE html>
    <html>
    <head>
      <meta name="traceparent" content="${traceparent}" />
      <meta name="tracestate" content="${tracestate}" />
    </head>
    <body>
      <div id="root"></div>
      <script src="/bundle.js"></script>
    </body>
    </html>
  `);
});
```

#### Server Side (Next.js)

```typescript
// app/layout.tsx
import { trace, context, propagation } from '@opentelemetry/api';

export default function RootLayout({ children }: { children: React.ReactNode }) {
  const span = trace.getSpan(context.active());
  const spanContext = span?.spanContext();

  let traceparent = '';
  let tracestate = '';

  if (spanContext) {
    traceparent = `00-${spanContext.traceId}-${spanContext.spanId}-0${spanContext.traceFlags}`;
    const carrier: Record<string, string> = {};
    propagation.inject(context.active(), carrier);
    tracestate = carrier['tracestate'] || '';
  }

  return (
    <html>
      <head>
        <meta name="traceparent" content={traceparent} />
        <meta name="tracestate" content={tracestate} />
      </head>
      <body>{children}</body>
    </html>
  );
}
```

#### Browser Side

```typescript
import { context, trace } from '@opentelemetry/api';
import { W3CTraceContextPropagator } from '@opentelemetry/core';

export function connectToServerTrace(): void {
  const traceparent = document.querySelector('meta[name="traceparent"]')?.getAttribute('content');
  const tracestate = document.querySelector('meta[name="tracestate"]')?.getAttribute('content');

  if (!traceparent) return;

  const propagator = new W3CTraceContextPropagator();
  const serverContext = propagator.extract(context.active(),
    { traceparent, tracestate: tracestate || '' },
    {
      get: (carrier, key) => carrier[key as keyof typeof carrier],
      keys: (carrier) => Object.keys(carrier)
    }
  );

  // Create browser spans as children of the server trace
  context.with(serverContext, () => {
    const tracer = trace.getTracer('my-frontend');
    tracer.startActiveSpan('browser.hydration', (span) => {
      span.setAttribute('correlation.method', 'meta_tag');
      span.end();
    });
  });
}
```

---

### Method 3: Server-Timing Header

Alternative to meta tags - server sends trace context in response headers.

**Use when:** You want response-level correlation without modifying HTML.

#### Server Side

```typescript
import { trace, context } from '@opentelemetry/api';

app.use((req, res, next) => {
  res.on('finish', () => {
    const span = trace.getSpan(context.active());
    const spanContext = span?.spanContext();

    if (spanContext) {
      const traceparent = `00-${spanContext.traceId}-${spanContext.spanId}-0${spanContext.traceFlags}`;
      res.setHeader('Server-Timing', `traceparent;desc="${traceparent}"`);
    }
  });
  next();
});
```

#### Browser Side

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

---

### Method 4: Initial Data Payload

For SPAs that fetch initial data via API - embed trace context in the response body.

**Use when:** Your SPA loads all data via API calls (no SSR).

#### Server Side

```typescript
app.get('/api/initial-data', (req, res) => {
  const span = trace.getSpan(context.active());
  const spanContext = span?.spanContext();

  res.json({
    data: getInitialData(),
    _trace: spanContext ? {
      traceparent: `00-${spanContext.traceId}-${spanContext.spanId}-0${spanContext.traceFlags}`,
      tracestate: ''
    } : null
  });
});
```

#### Browser Side

```typescript
async function loadInitialData() {
  const response = await fetch('/api/initial-data');
  const { data, _trace } = await response.json();

  if (_trace?.traceparent) {
    const propagator = new W3CTraceContextPropagator();
    const serverContext = propagator.extract(context.active(), _trace, {
      get: (carrier, key) => carrier[key as keyof typeof carrier],
      keys: () => ['traceparent', 'tracestate']
    });

    context.with(serverContext, () => {
      tracer.startActiveSpan('app.initialize', (span) => {
        // Process data...
        span.end();
      });
    });
  }

  return data;
}
```

---

## Method Comparison

| Method | Best For | Complexity | Automatic |
|--------|----------|------------|-----------|
| **Fetch Header** | All apps (Recommended) | Low | Yes |
| Meta Tag | SSR page load correlation | Medium | No |
| Server-Timing | Response-level correlation | Medium | No |
| Data Payload | SPA initial data | Medium | No |

**Most apps only need Method 1** (Fetch Header Propagation). The other methods are for specific scenarios like correlating SSR page loads with browser hydration.

---

## Viewing Correlated Traces in Dash0

### Finding Full-Stack Traces

1. Open [app.dash0.com](https://app.dash0.com)
2. Go to **Tracing** → **Traces**
3. Filter by your frontend service name
4. Click on a trace to see the waterfall

### What You'll See

```
┌──────────────────────────────────────────────────────────────────────┐
│ Trace: abc123def456789...                                             │
├──────────────────────────────────────────────────────────────────────┤
│ my-frontend: documentLoad ────────────────────────────────── 1200ms  │
│   └─ my-frontend: fetch /api/products ─────────────────────── 450ms  │
│       └─ api-server: GET /api/products ────────────────────── 440ms  │
│           └─ api-server: db.query products ────────────────── 380ms  │
│           └─ api-server: cache.set products ───────────────── 10ms   │
│   └─ my-frontend: react.render.ProductList ────────────────── 50ms   │
│   └─ my-frontend: fetch /api/recommendations ──────────────── 200ms  │
│       └─ api-server: GET /api/recommendations ─────────────── 190ms  │
│           └─ recommendation-service: compute ──────────────── 150ms  │
└──────────────────────────────────────────────────────────────────────┘
```

### Useful Filters

```
# All traces from a specific browser session
session.id = "abc123"

# All traces with browser and server spans
service.name in ["my-frontend", "api-server"]

# Traces with errors anywhere
status.code = ERROR

# Slow page loads
name = "documentLoad" AND duration > 2000ms
```

---

## Complete Integration Example

### Server (Express + OTel)

```typescript
// server/instrumentation.ts
import { NodeSDK } from '@opentelemetry/sdk-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-grpc';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';

const sdk = new NodeSDK({
  serviceName: 'api-server',
  traceExporter: new OTLPTraceExporter({
    url: 'https://ingress.eu-west-1.dash0.com:4317',
    headers: { 'Authorization': `Bearer ${process.env.DASH0_AUTH_TOKEN}` }
  }),
  instrumentations: [getNodeAutoInstrumentations()]
});

sdk.start();
```

```typescript
// server/app.ts
import express from 'express';
import cors from 'cors';
import { trace, context, propagation } from '@opentelemetry/api';

const app = express();

// CORS with trace headers
app.use(cors({
  origin: ['https://yoursite.com', 'http://localhost:3000'],
  allowedHeaders: ['Content-Type', 'Authorization', 'traceparent', 'tracestate']
}));

// Inject trace context into HTML
app.get('/', (req, res) => {
  const span = trace.getSpan(context.active());
  const ctx = span?.spanContext();

  const traceparent = ctx
    ? `00-${ctx.traceId}-${ctx.spanId}-0${ctx.traceFlags}`
    : '';

  res.send(`
    <!DOCTYPE html>
    <html>
    <head>
      <meta name="traceparent" content="${traceparent}" />
    </head>
    <body>
      <div id="root"></div>
      <script src="/bundle.js"></script>
    </body>
    </html>
  `);
});

// API endpoints (auto-instrumented)
app.get('/api/products', async (req, res) => {
  const products = await db.query('SELECT * FROM products');
  res.json(products);
});

app.listen(3001);
```

### Browser (React + OTel)

```typescript
// browser/instrumentation.ts
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { ZoneContextManager } from '@opentelemetry/context-zone';
import { Resource } from '@opentelemetry/resources';
import { ATTR_SERVICE_NAME } from '@opentelemetry/semantic-conventions';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { context, trace, propagation } from '@opentelemetry/api';
import { W3CTraceContextPropagator } from '@opentelemetry/core';

export function initTelemetry() {
  const provider = new WebTracerProvider({
    resource: new Resource({
      [ATTR_SERVICE_NAME]: 'my-frontend',
      'session.id': getSessionId()
    })
  });

  provider.addSpanProcessor(new BatchSpanProcessor(
    new OTLPTraceExporter({
      url: 'https://ingress.eu-west-1.dash0.com:4318/v1/traces',
      headers: { 'Authorization': `Bearer ${DASH0_AUTH_TOKEN}` }
    })
  ));

  provider.register({
    contextManager: new ZoneContextManager()
  });

  registerInstrumentations({
    instrumentations: [
      new DocumentLoadInstrumentation(),
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [/localhost:3001/, /api\.yoursite\.com/]
      })
    ]
  });

  // Connect to server trace
  connectToServerTrace();
}

function connectToServerTrace() {
  const traceparent = document.querySelector('meta[name="traceparent"]')?.getAttribute('content');

  if (traceparent) {
    const propagator = new W3CTraceContextPropagator();
    const serverContext = propagator.extract(context.active(), { traceparent }, {
      get: (c, k) => c[k as keyof typeof c],
      keys: () => ['traceparent']
    });

    context.with(serverContext, () => {
      const tracer = trace.getTracer('my-frontend');
      tracer.startActiveSpan('browser.ready', (span) => {
        span.setAttribute('page.url', window.location.href);
        span.end();
      });
    });
  }
}

function getSessionId() {
  let id = sessionStorage.getItem('session_id');
  if (!id) {
    id = crypto.randomUUID();
    sessionStorage.setItem('session_id', id);
  }
  return id;
}
```

```typescript
// browser/index.tsx
import './instrumentation';
import { createRoot } from 'react-dom/client';
import App from './App';

initTelemetry();
createRoot(document.getElementById('root')!).render(<App />);
```

---

## Troubleshooting

### Traces Not Connected

**Symptom**: Browser and server traces have different trace IDs.

**Causes & Solutions**:

1. **Missing propagateTraceHeaderCorsUrls**
   ```typescript
   new FetchInstrumentation({
     propagateTraceHeaderCorsUrls: [/your-api\.com/]  // Add your API domain
   })
   ```

2. **CORS blocking headers**
   ```typescript
   // Server must allow trace headers
   app.use(cors({
     allowedHeaders: ['traceparent', 'tracestate', ...]
   }));
   ```

3. **HTTP vs HTTPS mismatch**
   - Ensure patterns match the actual protocol

### Server Context Not Extracted

**Symptom**: Browser spans don't continue server trace.

**Causes & Solutions**:

1. **Meta tag not rendered**
   - Check server is injecting `<meta name="traceparent" ...>`

2. **Script loads before DOM ready**
   ```typescript
   // Wait for DOM
   if (document.readyState === 'loading') {
     document.addEventListener('DOMContentLoaded', connectToServerTrace);
   } else {
     connectToServerTrace();
   }
   ```

3. **Wrong trace format**
   - traceparent format: `00-{32-hex-traceId}-{16-hex-spanId}-{2-hex-flags}`

### Different Services, Same Trace

**Symptom**: Want to see multiple microservices in one trace.

**Solution**: All services must:
1. Accept `traceparent` header
2. Propagate context to downstream calls
3. Export to the same backend (Dash0)

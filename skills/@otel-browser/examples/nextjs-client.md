# Next.js Client-Side Telemetry Example

Complete OpenTelemetry setup for Next.js App Router with both server and client-side instrumentation.

## Overview

Next.js has two runtime environments:
- **Server**: Node.js (RSC, API routes, middleware)
- **Client**: Browser (client components, hydration)

This example shows how to instrument both and connect them.

## Project Structure

```
my-nextjs-app/
├── instrumentation.ts          # Server-side OTel (Next.js built-in)
├── src/
│   ├── app/
│   │   ├── layout.tsx          # Injects trace context into HTML
│   │   ├── page.tsx
│   │   └── products/
│   │       └── page.tsx
│   ├── components/
│   │   ├── TelemetryProvider.tsx   # Client-side OTel wrapper
│   │   └── TracedErrorBoundary.tsx
│   └── lib/
│       ├── telemetry/
│       │   ├── client.ts       # Browser OTel setup
│       │   ├── server.ts       # Server utilities
│       │   └── web-vitals.ts
│       └── hooks/
│           └── useTracer.ts
├── next.config.js
└── package.json
```

## Dependencies

```bash
# Server-side (Node.js)
npm install @opentelemetry/sdk-node \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-grpc

# Client-side (Browser)
npm install @opentelemetry/api \
  @opentelemetry/sdk-trace-web \
  @opentelemetry/sdk-trace-base \
  @opentelemetry/resources \
  @opentelemetry/semantic-conventions \
  @opentelemetry/exporter-trace-otlp-http \
  @opentelemetry/context-zone \
  @opentelemetry/instrumentation \
  @opentelemetry/instrumentation-document-load \
  @opentelemetry/instrumentation-fetch \
  @opentelemetry/core \
  web-vitals
```

## Server-Side Instrumentation

### instrumentation.ts (Project Root)

Next.js 13.4+ automatically loads this file for server-side instrumentation.

```typescript
import { NodeSDK } from '@opentelemetry/sdk-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-grpc';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { Resource } from '@opentelemetry/resources';
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT
} from '@opentelemetry/semantic-conventions';

export function register() {
  const sdk = new NodeSDK({
    resource: new Resource({
      [ATTR_SERVICE_NAME]: process.env.OTEL_SERVICE_NAME || 'my-nextjs-app',
      [ATTR_SERVICE_VERSION]: process.env.npm_package_version || '1.0.0',
      [ATTR_DEPLOYMENT_ENVIRONMENT]: process.env.NODE_ENV || 'development'
    }),

    traceExporter: new OTLPTraceExporter({
      url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT || 'https://ingress.eu-west-1.dash0.com:4317',
      headers: {
        'Authorization': `Bearer ${process.env.DASH0_AUTH_TOKEN}`
      }
    }),

    instrumentations: [
      getNodeAutoInstrumentations({
        '@opentelemetry/instrumentation-fs': { enabled: false }
      })
    ]
  });

  sdk.start();

  process.on('SIGTERM', () => {
    sdk.shutdown()
      .then(() => console.log('OTel SDK shut down'))
      .catch((err) => console.error('Error shutting down OTel SDK', err))
      .finally(() => process.exit(0));
  });
}
```

### next.config.js

```javascript
/** @type {import('next').NextConfig} */
const nextConfig = {
  experimental: {
    // Enable instrumentation hook
    instrumentationHook: true
  }
};

module.exports = nextConfig;
```

## Client-Side Instrumentation

### src/lib/telemetry/client.ts

```typescript
'use client';

import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import {
  BatchSpanProcessor,
  SimpleSpanProcessor,
  ConsoleSpanExporter
} from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { ZoneContextManager } from '@opentelemetry/context-zone';
import { Resource } from '@opentelemetry/resources';
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT
} from '@opentelemetry/semantic-conventions';
import { ParentBasedSampler, TraceIdRatioBasedSampler } from '@opentelemetry/sdk-trace-base';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { context, trace } from '@opentelemetry/api';
import { W3CTraceContextPropagator } from '@opentelemetry/core';

let initialized = false;
let provider: WebTracerProvider | null = null;

interface TelemetryConfig {
  serviceName: string;
  serviceVersion: string;
  environment: string;
  dash0Endpoint: string;
  dash0Token: string | null;
  sampleRate: number;
}

function getConfig(): TelemetryConfig {
  return {
    serviceName: `${process.env.NEXT_PUBLIC_SERVICE_NAME || 'my-nextjs-app'}-browser`,
    serviceVersion: process.env.NEXT_PUBLIC_APP_VERSION || '1.0.0',
    environment: process.env.NODE_ENV || 'development',
    dash0Endpoint: process.env.NEXT_PUBLIC_DASH0_ENDPOINT || 'https://ingress.eu-west-1.dash0.com:4318/v1/traces',
    dash0Token: process.env.NEXT_PUBLIC_DASH0_AUTH_TOKEN || null,
    sampleRate: process.env.NODE_ENV === 'production' ? 0.1 : 1.0
  };
}

function getSessionId(): string {
  if (typeof window === 'undefined') return '';

  let sessionId = sessionStorage.getItem('otel_session_id');
  if (!sessionId) {
    sessionId = crypto.randomUUID();
    sessionStorage.setItem('otel_session_id', sessionId);
  }
  return sessionId;
}

export function initClientTelemetry(): void {
  if (initialized || typeof window === 'undefined') {
    return;
  }

  const config = getConfig();

  // Create resource
  const resource = new Resource({
    [ATTR_SERVICE_NAME]: config.serviceName,
    [ATTR_SERVICE_VERSION]: config.serviceVersion,
    [ATTR_DEPLOYMENT_ENVIRONMENT]: config.environment,
    'session.id': getSessionId(),
    'browser.language': navigator.language
  });

  // Create provider
  provider = new WebTracerProvider({
    resource,
    sampler: new ParentBasedSampler({
      root: new TraceIdRatioBasedSampler(config.sampleRate)
    })
  });

  // Console exporter for development
  if (config.environment === 'development') {
    provider.addSpanProcessor(new SimpleSpanProcessor(new ConsoleSpanExporter()));
  }

  // OTLP exporter for Dash0
  if (config.dash0Token) {
    provider.addSpanProcessor(
      new BatchSpanProcessor(
        new OTLPTraceExporter({
          url: config.dash0Endpoint,
          headers: {
            'Authorization': `Bearer ${config.dash0Token}`
          }
        }),
        {
          maxQueueSize: 100,
          maxExportBatchSize: 30,
          scheduledDelayMillis: 1000
        }
      )
    );
  }

  // Register provider
  provider.register({
    contextManager: new ZoneContextManager()
  });

  // Register instrumentations
  registerInstrumentations({
    instrumentations: [
      new DocumentLoadInstrumentation(),
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [
          /localhost/,
          new RegExp(window.location.origin.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
        ],
        ignoreUrls: [/_next\/static/, /\.(png|jpg|svg|woff)/]
      })
    ]
  });

  // Connect to server trace
  connectServerTrace();

  // Flush on page hide
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden' && provider) {
      provider.forceFlush();
    }
  });

  initialized = true;
  console.log(`Client telemetry initialized: ${config.serviceName}`);
}

function connectServerTrace(): void {
  const traceparent = document.querySelector('meta[name="traceparent"]')?.getAttribute('content');

  if (!traceparent) return;

  const propagator = new W3CTraceContextPropagator();
  const serverContext = propagator.extract(context.active(), { traceparent }, {
    get: (carrier, key) => carrier[key as keyof typeof carrier],
    keys: () => ['traceparent']
  });

  context.with(serverContext, () => {
    const tracer = trace.getTracer(getConfig().serviceName);
    tracer.startActiveSpan('browser.hydration', (span) => {
      span.setAttribute('page.url', window.location.href);
      span.end();
    });
  });
}

export function getTracer() {
  return trace.getTracer(getConfig().serviceName);
}
```

### src/lib/telemetry/server.ts

```typescript
import { trace, context, propagation } from '@opentelemetry/api';

/**
 * Get trace context for injection into HTML
 */
export function getTraceContext(): { traceparent: string; tracestate: string } {
  const span = trace.getSpan(context.active());
  const spanContext = span?.spanContext();

  if (!spanContext) {
    return { traceparent: '', tracestate: '' };
  }

  // Format: version-traceId-spanId-flags
  const traceparent = `00-${spanContext.traceId}-${spanContext.spanId}-0${spanContext.traceFlags}`;

  // Get tracestate
  const carrier: Record<string, string> = {};
  propagation.inject(context.active(), carrier);
  const tracestate = carrier['tracestate'] || '';

  return { traceparent, tracestate };
}
```

### src/lib/telemetry/web-vitals.ts

```typescript
'use client';

import { onCLS, onINP, onLCP, onFCP, onTTFB, Metric } from 'web-vitals';
import { trace, metrics } from '@opentelemetry/api';

const SERVICE_NAME = `${process.env.NEXT_PUBLIC_SERVICE_NAME || 'my-nextjs-app'}-browser`;

export function initWebVitals() {
  if (typeof window === 'undefined') return;

  const tracer = trace.getTracer(SERVICE_NAME);
  const meter = metrics.getMeter(SERVICE_NAME);

  const histograms = {
    lcp: meter.createHistogram('web_vital.lcp', { unit: 'ms' }),
    cls: meter.createHistogram('web_vital.cls', { unit: '1' }),
    inp: meter.createHistogram('web_vital.inp', { unit: 'ms' }),
    fcp: meter.createHistogram('web_vital.fcp', { unit: 'ms' }),
    ttfb: meter.createHistogram('web_vital.ttfb', { unit: 'ms' })
  };

  const reportVital = (name: keyof typeof histograms, metric: Metric) => {
    const attributes = {
      'page.path': window.location.pathname,
      'vital.rating': metric.rating
    };

    histograms[name].record(metric.value, attributes);

    const span = tracer.startSpan(`web_vital.${name}`, {
      attributes: {
        ...attributes,
        'vital.value': metric.value,
        'vital.delta': metric.delta
      }
    });
    span.end();
  };

  onLCP((m) => reportVital('lcp', m));
  onCLS((m) => reportVital('cls', m));
  onINP((m) => reportVital('inp', m));
  onFCP((m) => reportVital('fcp', m));
  onTTFB((m) => reportVital('ttfb', m));
}
```

## Components

### src/components/TelemetryProvider.tsx

```typescript
'use client';

import { useEffect, ReactNode } from 'react';
import { initClientTelemetry } from '@/lib/telemetry/client';
import { initWebVitals } from '@/lib/telemetry/web-vitals';

interface Props {
  children: ReactNode;
}

export function TelemetryProvider({ children }: Props) {
  useEffect(() => {
    initClientTelemetry();
    initWebVitals();
  }, []);

  return <>{children}</>;
}
```

### src/components/TracedErrorBoundary.tsx

```typescript
'use client';

import { Component, ErrorInfo, ReactNode } from 'react';
import { SpanStatusCode } from '@opentelemetry/api';
import { getTracer } from '@/lib/telemetry/client';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
  boundaryName?: string;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class TracedErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null };

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    const tracer = getTracer();
    const span = tracer.startSpan('react.error_boundary', {
      attributes: {
        'error.boundary': this.props.boundaryName || 'unnamed',
        'page.url': typeof window !== 'undefined' ? window.location.href : ''
      }
    });

    span.recordException(error);
    span.setStatus({ code: SpanStatusCode.ERROR, message: error.message });

    if (errorInfo.componentStack) {
      span.setAttribute('react.component_stack', errorInfo.componentStack);
    }

    span.end();
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback || (
        <div className="error-boundary">
          <h2>Something went wrong</h2>
          <button onClick={() => this.setState({ hasError: false, error: null })}>
            Try again
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
```

## App Router Integration

### src/app/layout.tsx

```typescript
import { ReactNode } from 'react';
import { getTraceContext } from '@/lib/telemetry/server';
import { TelemetryProvider } from '@/components/TelemetryProvider';
import { TracedErrorBoundary } from '@/components/TracedErrorBoundary';

export default function RootLayout({ children }: { children: ReactNode }) {
  // Get server trace context to pass to browser
  const { traceparent, tracestate } = getTraceContext();

  return (
    <html lang="en">
      <head>
        {/* Inject server trace context for browser correlation */}
        {traceparent && (
          <>
            <meta name="traceparent" content={traceparent} />
            <meta name="tracestate" content={tracestate} />
          </>
        )}
      </head>
      <body>
        <TelemetryProvider>
          <TracedErrorBoundary boundaryName="root">
            {children}
          </TracedErrorBoundary>
        </TelemetryProvider>
      </body>
    </html>
  );
}
```

### src/app/page.tsx

```typescript
import { trace, context } from '@opentelemetry/api';
import Link from 'next/link';

export default async function HomePage() {
  // Server-side span
  const tracer = trace.getTracer('my-nextjs-app');

  return tracer.startActiveSpan('render.home', (span) => {
    try {
      span.setAttribute('page.name', 'home');

      return (
        <main>
          <h1>Welcome</h1>
          <nav>
            <Link href="/products">View Products</Link>
          </nav>
        </main>
      );
    } finally {
      span.end();
    }
  });
}
```

### src/app/products/page.tsx

```typescript
import { trace } from '@opentelemetry/api';

interface Product {
  id: string;
  name: string;
  price: number;
}

async function getProducts(): Promise<Product[]> {
  const tracer = trace.getTracer('my-nextjs-app');

  return tracer.startActiveSpan('fetch.products', async (span) => {
    try {
      const response = await fetch('https://api.example.com/products', {
        next: { revalidate: 60 }
      });

      if (!response.ok) {
        throw new Error(`Failed to fetch products: ${response.status}`);
      }

      const products = await response.json();
      span.setAttribute('products.count', products.length);

      return products;
    } finally {
      span.end();
    }
  });
}

export default async function ProductsPage() {
  const products = await getProducts();

  return (
    <main>
      <h1>Products</h1>
      <ul>
        {products.map((product) => (
          <li key={product.id}>
            {product.name} - ${product.price}
            <AddToCartButton productId={product.id} />
          </li>
        ))}
      </ul>
    </main>
  );
}

// Client component for interactivity
'use client';

import { useActionTracer } from '@/lib/hooks/useTracer';

function AddToCartButton({ productId }: { productId: string }) {
  const { trackAction } = useActionTracer();

  const handleClick = async () => {
    await trackAction('add_to_cart', async () => {
      const response = await fetch('/api/cart', {
        method: 'POST',
        body: JSON.stringify({ productId })
      });
      return response.json();
    }, { 'product.id': productId });
  };

  return (
    <button data-track="true" onClick={handleClick}>
      Add to Cart
    </button>
  );
}
```

## Hooks

### src/lib/hooks/useTracer.ts

```typescript
'use client';

import { useCallback } from 'react';
import { SpanStatusCode } from '@opentelemetry/api';
import { getTracer } from '@/lib/telemetry/client';

export function useActionTracer() {
  const trackAction = useCallback(
    async <T>(
      actionName: string,
      action: () => Promise<T>,
      attributes?: Record<string, string | number>
    ): Promise<T> => {
      const tracer = getTracer();

      return tracer.startActiveSpan(`action.${actionName}`, async (span) => {
        try {
          span.setAttribute('page.url', window.location.href);

          if (attributes) {
            Object.entries(attributes).forEach(([key, value]) => {
              span.setAttribute(key, value);
            });
          }

          return await action();
        } catch (error) {
          span.recordException(error as Error);
          span.setStatus({ code: SpanStatusCode.ERROR });
          throw error;
        } finally {
          span.end();
        }
      });
    },
    []
  );

  return { trackAction };
}
```

## Environment Variables

### .env.local

```bash
# Server-side
OTEL_SERVICE_NAME=my-nextjs-app
OTEL_EXPORTER_OTLP_ENDPOINT=https://ingress.eu-west-1.dash0.com:4317
DASH0_AUTH_TOKEN=your-server-token

# Client-side (exposed to browser)
NEXT_PUBLIC_SERVICE_NAME=my-nextjs-app
NEXT_PUBLIC_APP_VERSION=1.0.0
NEXT_PUBLIC_DASH0_ENDPOINT=https://ingress.eu-west-1.dash0.com:4318/v1/traces
NEXT_PUBLIC_DASH0_AUTH_TOKEN=your-client-token
```

## Verification

### Check Server Traces

```bash
# Run the app
npm run dev

# Make requests
curl http://localhost:3000
curl http://localhost:3000/products

# Check Dash0 for "my-nextjs-app" service
```

### Check Client Traces

1. Open browser DevTools → Console
2. Look for "Client telemetry initialized"
3. Navigate between pages
4. Check Dash0 for "my-nextjs-app-browser" service

### Verify Correlation

In Dash0, you should see:
```
my-nextjs-app: GET /products ──────────────────────── 200ms
  └─ my-nextjs-app: fetch.products ────────────────── 150ms
  └─ my-nextjs-app-browser: browser.hydration ─────── 50ms
      └─ my-nextjs-app-browser: fetch /api/cart ───── 100ms
          └─ my-nextjs-app: POST /api/cart ────────── 90ms
```

## Security Note

For production, consider proxying browser telemetry through your backend to avoid exposing the Dash0 token:

```typescript
// src/app/api/telemetry/route.ts
export async function POST(request: Request) {
  const body = await request.arrayBuffer();

  const response = await fetch(process.env.DASH0_ENDPOINT!, {
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

Then configure the client exporter to use `/api/telemetry` instead of the Dash0 endpoint directly.

# React SPA Example

Complete OpenTelemetry setup for a React Single Page Application with server correlation.

## Project Structure

```
my-react-app/
├── src/
│   ├── telemetry/
│   │   ├── instrumentation.ts   # OTel setup
│   │   ├── tracer.ts            # Tracer utilities
│   │   └── web-vitals.ts        # Web Vitals tracking
│   ├── hooks/
│   │   ├── useRouteTracing.ts   # Route change tracking
│   │   └── useActionTracer.ts   # Action tracking hook
│   ├── components/
│   │   └── TracedErrorBoundary.tsx
│   ├── App.tsx
│   └── index.tsx
├── package.json
└── vite.config.ts
```

## Dependencies

```bash
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
  @opentelemetry/instrumentation-user-interaction \
  @opentelemetry/core \
  web-vitals
```

## Instrumentation Setup

### src/telemetry/instrumentation.ts

```typescript
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
import { UserInteractionInstrumentation } from '@opentelemetry/instrumentation-user-interaction';
import { context, trace, propagation } from '@opentelemetry/api';
import { W3CTraceContextPropagator } from '@opentelemetry/core';

// Configuration
const CONFIG = {
  serviceName: import.meta.env.VITE_SERVICE_NAME || 'my-react-app',
  serviceVersion: import.meta.env.VITE_APP_VERSION || '1.0.0',
  environment: import.meta.env.MODE || 'development',
  dash0Endpoint: import.meta.env.VITE_DASH0_ENDPOINT || 'https://ingress.eu-west-1.dash0.com:4318/v1/traces',
  dash0Token: import.meta.env.VITE_DASH0_AUTH_TOKEN,
  apiPatterns: [
    /https:\/\/api\.yoursite\.com.*/,
    /http:\/\/localhost:\d+\/api.*/
  ],
  sampleRate: import.meta.env.MODE === 'production' ? 0.1 : 1.0
};

// Session ID (persists across page reloads within session)
function getSessionId(): string {
  let sessionId = sessionStorage.getItem('otel_session_id');
  if (!sessionId) {
    sessionId = crypto.randomUUID();
    sessionStorage.setItem('otel_session_id', sessionId);
  }
  return sessionId;
}

// Provider instance (for access in other modules)
let provider: WebTracerProvider | null = null;

export function initTelemetry(): WebTracerProvider {
  if (provider) {
    return provider;
  }

  // 1. Create resource
  const resource = new Resource({
    [ATTR_SERVICE_NAME]: CONFIG.serviceName,
    [ATTR_SERVICE_VERSION]: CONFIG.serviceVersion,
    [ATTR_DEPLOYMENT_ENVIRONMENT]: CONFIG.environment,
    'session.id': getSessionId(),
    'browser.language': navigator.language,
    'browser.viewport.width': window.innerWidth,
    'browser.viewport.height': window.innerHeight
  });

  // 2. Create provider with sampling
  provider = new WebTracerProvider({
    resource,
    sampler: new ParentBasedSampler({
      root: new TraceIdRatioBasedSampler(CONFIG.sampleRate)
    })
  });

  // 3. Console exporter for development
  if (CONFIG.environment === 'development') {
    provider.addSpanProcessor(new SimpleSpanProcessor(new ConsoleSpanExporter()));
  }

  // 4. OTLP exporter for Dash0
  if (CONFIG.dash0Token) {
    provider.addSpanProcessor(
      new BatchSpanProcessor(
        new OTLPTraceExporter({
          url: CONFIG.dash0Endpoint,
          headers: {
            'Authorization': `Bearer ${CONFIG.dash0Token}`
          }
        }),
        {
          maxQueueSize: 100,
          maxExportBatchSize: 30,
          scheduledDelayMillis: 1000
        }
      )
    );
  } else {
    console.warn('VITE_DASH0_AUTH_TOKEN not set, traces will only go to console');
  }

  // 5. Register provider with context manager
  provider.register({
    contextManager: new ZoneContextManager()
  });

  // 6. Register auto-instrumentations
  registerInstrumentations({
    instrumentations: [
      new DocumentLoadInstrumentation({
        applyCustomAttributesOnSpan: {
          documentLoad: (span) => {
            span.setAttribute('page.url', window.location.href);
            span.setAttribute('page.title', document.title);
          }
        }
      }),

      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: CONFIG.apiPatterns,
        ignoreUrls: [
          /google-analytics\.com/,
          /sentry\.io/,
          /hotjar\.com/
        ],
        clearTimingResources: true,
        applyCustomAttributesOnSpan: (span, request, response) => {
          const requestId = response.headers.get('x-request-id');
          if (requestId) {
            span.setAttribute('http.request_id', requestId);
          }
        }
      }),

      new UserInteractionInstrumentation({
        eventNames: ['click', 'submit'],
        shouldPreventSpanCreation: (eventType, element) => {
          // Only track elements with data-track attribute
          return !element.hasAttribute('data-track');
        }
      })
    ]
  });

  // 7. Connect to server trace context (if present)
  connectToServerTrace();

  // 8. Flush on page hide
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden' && provider) {
      provider.forceFlush();
    }
  });

  console.log(`Telemetry initialized for ${CONFIG.serviceName} (${CONFIG.environment})`);

  return provider;
}

function connectToServerTrace(): void {
  const traceparent = document.querySelector('meta[name="traceparent"]')?.getAttribute('content');
  const tracestate = document.querySelector('meta[name="tracestate"]')?.getAttribute('content');

  if (!traceparent) {
    return;
  }

  const carrier = { traceparent, tracestate: tracestate || '' };
  const propagator = new W3CTraceContextPropagator();

  const serverContext = propagator.extract(context.active(), carrier, {
    get: (carrier, key) => carrier[key as keyof typeof carrier],
    keys: (carrier) => Object.keys(carrier)
  });

  // Create initial browser span as child of server trace
  context.with(serverContext, () => {
    const tracer = trace.getTracer(CONFIG.serviceName);
    tracer.startActiveSpan('browser.hydration', (span) => {
      span.setAttribute('correlation.method', 'meta_tag');
      span.setAttribute('page.url', window.location.href);
      span.end();
    });
  });
}

export function getProvider(): WebTracerProvider | null {
  return provider;
}
```

### src/telemetry/tracer.ts

```typescript
import { trace, Span, SpanStatusCode, context } from '@opentelemetry/api';

const TRACER_NAME = import.meta.env.VITE_SERVICE_NAME || 'my-react-app';

export function getTracer() {
  return trace.getTracer(TRACER_NAME);
}

// Utility for tracking async operations
export async function withSpan<T>(
  name: string,
  operation: (span: Span) => Promise<T>,
  attributes?: Record<string, string | number | boolean>
): Promise<T> {
  const tracer = getTracer();

  return tracer.startActiveSpan(name, async (span) => {
    try {
      if (attributes) {
        Object.entries(attributes).forEach(([key, value]) => {
          span.setAttribute(key, value);
        });
      }

      const result = await operation(span);
      return result;
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

// Utility for tracking sync operations
export function withSpanSync<T>(
  name: string,
  operation: (span: Span) => T,
  attributes?: Record<string, string | number | boolean>
): T {
  const tracer = getTracer();

  return tracer.startActiveSpan(name, (span) => {
    try {
      if (attributes) {
        Object.entries(attributes).forEach(([key, value]) => {
          span.setAttribute(key, value);
        });
      }

      const result = operation(span);
      return result;
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

### src/telemetry/web-vitals.ts

```typescript
import { onCLS, onINP, onLCP, onFCP, onTTFB, Metric } from 'web-vitals';
import { trace, metrics } from '@opentelemetry/api';

const TRACER_NAME = import.meta.env.VITE_SERVICE_NAME || 'my-react-app';
const tracer = trace.getTracer(TRACER_NAME);
const meter = metrics.getMeter(TRACER_NAME);

// Create histograms for each vital
const vitalHistograms = {
  lcp: meter.createHistogram('web_vital.lcp', {
    description: 'Largest Contentful Paint',
    unit: 'ms'
  }),
  cls: meter.createHistogram('web_vital.cls', {
    description: 'Cumulative Layout Shift',
    unit: '1'
  }),
  inp: meter.createHistogram('web_vital.inp', {
    description: 'Interaction to Next Paint',
    unit: 'ms'
  }),
  fcp: meter.createHistogram('web_vital.fcp', {
    description: 'First Contentful Paint',
    unit: 'ms'
  }),
  ttfb: meter.createHistogram('web_vital.ttfb', {
    description: 'Time to First Byte',
    unit: 'ms'
  })
};

function reportVital(name: keyof typeof vitalHistograms, metric: Metric) {
  const attributes = {
    'page.path': window.location.pathname,
    'vital.id': metric.id,
    'vital.rating': metric.rating
  };

  // Record as metric
  vitalHistograms[name].record(metric.value, attributes);

  // Also create a span for trace correlation
  const span = tracer.startSpan(`web_vital.${name}`, {
    attributes: {
      ...attributes,
      'vital.value': metric.value,
      'vital.delta': metric.delta
    }
  });
  span.end();
}

export function initWebVitals() {
  onLCP((metric) => reportVital('lcp', metric));
  onCLS((metric) => reportVital('cls', metric));
  onINP((metric) => reportVital('inp', metric));
  onFCP((metric) => reportVital('fcp', metric));
  onTTFB((metric) => reportVital('ttfb', metric));
}
```

## React Hooks

### src/hooks/useRouteTracing.ts

```typescript
import { useEffect, useRef } from 'react';
import { useLocation } from 'react-router-dom';
import { Span } from '@opentelemetry/api';
import { getTracer } from '../telemetry/tracer';

export function useRouteTracing() {
  const location = useLocation();
  const spanRef = useRef<Span | null>(null);
  const tracer = getTracer();

  useEffect(() => {
    // End previous route span if exists
    if (spanRef.current?.isRecording()) {
      spanRef.current.end();
    }

    // Start new route span
    const span = tracer.startSpan('route.change', {
      attributes: {
        'route.path': location.pathname,
        'route.search': location.search,
        'route.hash': location.hash,
        'route.full_url': window.location.href
      }
    });
    spanRef.current = span;

    // End span when route is "interactive"
    // Using requestIdleCallback for better accuracy
    const endSpan = () => {
      if (span.isRecording()) {
        span.setAttribute('route.interactive', true);
        span.end();
      }
    };

    if ('requestIdleCallback' in window) {
      const id = requestIdleCallback(endSpan, { timeout: 2000 });
      return () => {
        cancelIdleCallback(id);
        if (span.isRecording()) {
          span.end();
        }
      };
    } else {
      const id = setTimeout(endSpan, 100);
      return () => {
        clearTimeout(id);
        if (span.isRecording()) {
          span.end();
        }
      };
    }
  }, [location.pathname, tracer]);
}
```

### src/hooks/useActionTracer.ts

```typescript
import { useCallback } from 'react';
import { SpanStatusCode } from '@opentelemetry/api';
import { getTracer } from '../telemetry/tracer';

interface TraceOptions {
  attributes?: Record<string, string | number | boolean>;
}

export function useActionTracer() {
  const tracer = getTracer();

  const trackAction = useCallback(
    async <T>(
      actionName: string,
      action: () => Promise<T>,
      options?: TraceOptions
    ): Promise<T> => {
      return tracer.startActiveSpan(`action.${actionName}`, async (span) => {
        try {
          span.setAttribute('page.url', window.location.href);

          if (options?.attributes) {
            Object.entries(options.attributes).forEach(([key, value]) => {
              span.setAttribute(key, value);
            });
          }

          const result = await action();
          return result;
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
    },
    [tracer]
  );

  const trackClick = useCallback(
    (buttonName: string, attributes?: Record<string, string | number>) => {
      const span = tracer.startSpan(`click.${buttonName}`, {
        attributes: {
          'ui.button': buttonName,
          'page.url': window.location.href,
          ...attributes
        }
      });
      span.end();
    },
    [tracer]
  );

  return { trackAction, trackClick };
}
```

## Components

### src/components/TracedErrorBoundary.tsx

```typescript
import { Component, ErrorInfo, ReactNode } from 'react';
import { SpanStatusCode } from '@opentelemetry/api';
import { getTracer } from '../telemetry/tracer';

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
        'page.url': window.location.href
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
        <div>
          <h2>Something went wrong</h2>
          <details>
            <summary>Error details</summary>
            <pre>{this.state.error?.message}</pre>
          </details>
        </div>
      );
    }

    return this.props.children;
  }
}
```

## Application Entry Point

### src/index.tsx

```typescript
// IMPORTANT: Initialize telemetry FIRST
import { initTelemetry } from './telemetry/instrumentation';
import { initWebVitals } from './telemetry/web-vitals';

initTelemetry();
initWebVitals();

// Now import and render the app
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import App from './App';
import { TracedErrorBoundary } from './components/TracedErrorBoundary';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <TracedErrorBoundary boundaryName="root">
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </TracedErrorBoundary>
  </StrictMode>
);
```

### src/App.tsx

```typescript
import { Routes, Route } from 'react-router-dom';
import { useRouteTracing } from './hooks/useRouteTracing';
import { useActionTracer } from './hooks/useActionTracer';
import { TracedErrorBoundary } from './components/TracedErrorBoundary';

function App() {
  // Track route changes
  useRouteTracing();

  return (
    <div className="app">
      <nav>
        <a href="/">Home</a>
        <a href="/products">Products</a>
        <a href="/checkout">Checkout</a>
      </nav>

      <TracedErrorBoundary boundaryName="main-content">
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/products" element={<ProductsPage />} />
          <Route path="/checkout" element={<CheckoutPage />} />
        </Routes>
      </TracedErrorBoundary>
    </div>
  );
}

function HomePage() {
  return <h1>Welcome</h1>;
}

function ProductsPage() {
  const { trackAction } = useActionTracer();

  const handleAddToCart = async (productId: string) => {
    await trackAction(
      'add_to_cart',
      async () => {
        const response = await fetch('/api/cart', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ productId })
        });
        return response.json();
      },
      { 'product.id': productId }
    );
  };

  return (
    <div>
      <h1>Products</h1>
      <button
        data-track="true"
        onClick={() => handleAddToCart('prod-123')}
      >
        Add to Cart
      </button>
    </div>
  );
}

function CheckoutPage() {
  const { trackAction } = useActionTracer();

  const handleCheckout = async () => {
    await trackAction(
      'checkout.complete',
      async () => {
        const response = await fetch('/api/checkout', { method: 'POST' });
        return response.json();
      },
      { 'checkout.step': 'complete' }
    );
  };

  return (
    <div>
      <h1>Checkout</h1>
      <button data-track="true" onClick={handleCheckout}>
        Complete Purchase
      </button>
    </div>
  );
}

export default App;
```

## Environment Variables

### .env

```bash
VITE_SERVICE_NAME=my-react-app
VITE_APP_VERSION=1.0.0
VITE_DASH0_ENDPOINT=https://ingress.eu-west-1.dash0.com:4318/v1/traces
VITE_DASH0_AUTH_TOKEN=your-token-here
```

### .env.production

```bash
VITE_SERVICE_NAME=my-react-app
VITE_APP_VERSION=1.0.0
VITE_DASH0_ENDPOINT=https://ingress.eu-west-1.dash0.com:4318/v1/traces
VITE_DASH0_AUTH_TOKEN=your-production-token
```

## Verification

1. **Check console output** (development):
   ```
   Telemetry initialized for my-react-app (development)
   // Spans logged to console
   ```

2. **Check Dash0**:
   - Open [app.dash0.com](https://app.dash0.com)
   - Filter by `service.name = "my-react-app"`
   - Click traces to see spans

3. **Verify server correlation**:
   - Ensure your backend injects `<meta name="traceparent" ...>`
   - Browser spans should appear as children of server spans

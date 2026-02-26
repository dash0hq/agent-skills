---
title: "Browser Manual Instrumentation"
impact: HIGH
tags:
  - browser
  - manual-instrumentation
  - react
  - web-vitals
  - spa
---

# Browser Manual Instrumentation Reference

Guide to creating custom spans, tracking events, and recording metrics in browser applications.

## Official Documentation

- [Manual Instrumentation](https://opentelemetry.io/docs/languages/js/instrumentation/)
- [Tracing API](https://opentelemetry.io/docs/languages/js/api/tracing/)
- [Metrics API](https://opentelemetry.io/docs/languages/js/api/metrics/)

## Getting Tracer/Meter

```typescript
import { trace, metrics } from '@opentelemetry/api';

// Get a tracer (use service name and version)
const tracer = trace.getTracer('my-frontend', '1.0.0');

// Get a meter for metrics
const meter = metrics.getMeter('my-frontend', '1.0.0');
```

## Creating Custom Spans

### Basic Span

```typescript
import { trace, SpanStatusCode } from '@opentelemetry/api';

const tracer = trace.getTracer('my-frontend');

async function searchProducts(query: string) {
  return tracer.startActiveSpan('products.search', async (span) => {
    try {
      span.setAttribute('search.query', query);

      const results = await api.search(query);

      span.setAttribute('search.results_count', results.length);

      return results;
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

### With Specific Attributes

```typescript
import { trace, SpanKind } from '@opentelemetry/api';

const tracer = trace.getTracer('my-frontend');

function trackButtonClick(buttonName: string, context: Record<string, string>) {
  return tracer.startActiveSpan('ui.button_click', {
    kind: SpanKind.INTERNAL,
    attributes: {
      'ui.button.name': buttonName,
      'ui.page': window.location.pathname,
      ...context
    }
  }, (span) => {
    // Span automatically tracks timing
    span.end();
  });
}
```

## Common Patterns

### Track Route Changes (React Router)

```typescript
import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { trace } from '@opentelemetry/api';

const tracer = trace.getTracer('router');

export function useRouteTracing() {
  const location = useLocation();

  useEffect(() => {
    const span = tracer.startSpan('route.navigate', {
      attributes: {
        'route.path': location.pathname,
        'route.search': location.search,
        'route.hash': location.hash,
        'route.full_url': window.location.href
      }
    });

    // End span when route content is interactive
    const endSpan = () => {
      span.setAttribute('route.loaded', true);
      span.end();
    };

    // Use requestIdleCallback for more accurate "interactive" timing
    if ('requestIdleCallback' in window) {
      requestIdleCallback(endSpan, { timeout: 2000 });
    } else {
      setTimeout(endSpan, 100);
    }

    return () => {
      if (span.isRecording()) {
        span.end();
      }
    };
  }, [location.pathname]);
}

// Usage in App.tsx
function App() {
  useRouteTracing();
  return <Routes>...</Routes>;
}
```

### Track Form Submissions

```typescript
import { trace, SpanStatusCode } from '@opentelemetry/api';

const tracer = trace.getTracer('forms');

async function handleFormSubmit(formName: string, data: FormData) {
  return tracer.startActiveSpan(`form.submit.${formName}`, async (span) => {
    try {
      span.setAttribute('form.name', formName);
      span.setAttribute('form.fields_count', [...data.keys()].length);

      // Validation phase
      span.addEvent('validation.start');
      const errors = await validateForm(data);

      if (errors.length > 0) {
        span.addEvent('validation.failed', {
          'validation.error_count': errors.length
        });
        span.setStatus({ code: SpanStatusCode.ERROR, message: 'Validation failed' });
        return { success: false, errors };
      }
      span.addEvent('validation.passed');

      // Submission phase
      span.addEvent('submit.start');
      const result = await submitForm(formName, data);
      span.addEvent('submit.complete');

      span.setAttribute('form.success', true);
      return { success: true, result };
    } catch (error) {
      span.recordException(error as Error);
      span.setStatus({ code: SpanStatusCode.ERROR });
      throw error;
    } finally {
      span.end();
    }
  });
}
```

### Track Component Render Performance

```typescript
import { trace } from '@opentelemetry/api';
import { Profiler, ProfilerOnRenderCallback, ReactNode } from 'react';

const tracer = trace.getTracer('react-render');

const onRender: ProfilerOnRenderCallback = (
  id,           // Component name
  phase,        // "mount" or "update"
  actualDuration,
  baseDuration,
  startTime,
  commitTime
) => {
  // Convert to milliseconds (times are in ms from page load)
  const span = tracer.startSpan(`react.render.${id}`, {
    startTime: performance.timeOrigin + startTime
  });

  span.setAttribute('react.component', id);
  span.setAttribute('react.phase', phase);
  span.setAttribute('react.actual_duration_ms', actualDuration);
  span.setAttribute('react.base_duration_ms', baseDuration);

  // Flag slow renders
  if (actualDuration > 16) { // More than one frame at 60fps
    span.setAttribute('react.slow_render', true);
  }

  span.end(performance.timeOrigin + commitTime);
};

// Wrapper component
export function TracedComponent({ id, children }: { id: string; children: ReactNode }) {
  return (
    <Profiler id={id} onRender={onRender}>
      {children}
    </Profiler>
  );
}

// Usage
<TracedComponent id="ProductList">
  <ProductList products={products} />
</TracedComponent>
```

### Track User Actions with Context

```typescript
import { trace, context, SpanStatusCode } from '@opentelemetry/api';

const tracer = trace.getTracer('user-actions');

class ActionTracker {
  private userContext: Record<string, string> = {};

  setUser(userId: string, userTier: string) {
    this.userContext = { userId, userTier };
  }

  clearUser() {
    this.userContext = {};
  }

  track<T>(actionName: string, action: () => Promise<T>, attributes?: Record<string, string | number>): Promise<T> {
    return tracer.startActiveSpan(`action.${actionName}`, async (span) => {
      try {
        // Add user context
        Object.entries(this.userContext).forEach(([key, value]) => {
          span.setAttribute(`user.${key}`, value);
        });

        // Add custom attributes
        if (attributes) {
          Object.entries(attributes).forEach(([key, value]) => {
            span.setAttribute(key, value);
          });
        }

        // Add page context
        span.setAttribute('page.url', window.location.href);
        span.setAttribute('page.title', document.title);

        const result = await action();
        return result;
      } catch (error) {
        span.recordException(error as Error);
        span.setStatus({ code: SpanStatusCode.ERROR });
        throw error;
      } finally {
        span.end();
      }
    });
  }
}

// Usage
const tracker = new ActionTracker();

// After login
tracker.setUser(user.id, user.tier);

// Track actions
await tracker.track('checkout.complete', async () => {
  return processCheckout(cart);
}, {
  'cart.items': cart.items.length,
  'cart.total': cart.total
});
```

## Recording Metrics

### Counter (Events)

```typescript
import { metrics } from '@opentelemetry/api';

const meter = metrics.getMeter('my-frontend');

// Count button clicks
const buttonClicks = meter.createCounter('ui.button.clicks', {
  description: 'Number of button clicks',
  unit: '1'
});

function trackClick(buttonName: string) {
  buttonClicks.add(1, {
    'button.name': buttonName,
    'page.path': window.location.pathname
  });
}
```

### Histogram (Durations)

```typescript
const meter = metrics.getMeter('my-frontend');

const renderDuration = meter.createHistogram('react.render.duration', {
  description: 'React component render duration',
  unit: 'ms'
});

// In Profiler callback
renderDuration.record(actualDuration, {
  'component.name': componentName,
  'render.phase': phase
});
```

### Observable Gauge (Current Values)

```typescript
const meter = metrics.getMeter('my-frontend');

// Track current memory usage
meter.createObservableGauge('browser.memory.used', {
  description: 'Browser memory usage',
  unit: 'By'
}, (result) => {
  if ('memory' in performance) {
    const memory = (performance as any).memory;
    result.observe(memory.usedJSHeapSize);
  }
});

// Track viewport size
meter.createObservableGauge('browser.viewport.width', {
  description: 'Browser viewport width',
  unit: 'px'
}, (result) => {
  result.observe(window.innerWidth);
});
```

## Web Vitals Integration

```typescript
import { onCLS, onFID, onLCP, onFCP, onTTFB, onINP, Metric } from 'web-vitals';
import { trace, metrics } from '@opentelemetry/api';

const meter = metrics.getMeter('web-vitals');
const tracer = trace.getTracer('web-vitals');

// Create histograms for each vital
const vitals = {
  lcp: meter.createHistogram('web_vital.lcp', { unit: 'ms', description: 'Largest Contentful Paint' }),
  cls: meter.createHistogram('web_vital.cls', { unit: '1', description: 'Cumulative Layout Shift' }),
  inp: meter.createHistogram('web_vital.inp', { unit: 'ms', description: 'Interaction to Next Paint' }),
  fcp: meter.createHistogram('web_vital.fcp', { unit: 'ms', description: 'First Contentful Paint' }),
  ttfb: meter.createHistogram('web_vital.ttfb', { unit: 'ms', description: 'Time to First Byte' })
};

function reportVital(name: keyof typeof vitals, metric: Metric) {
  const attributes = {
    'page.path': window.location.pathname,
    'vital.id': metric.id,
    'vital.rating': metric.rating // 'good', 'needs-improvement', 'poor'
  };

  // Record metric
  vitals[name].record(metric.value, attributes);

  // Create span for correlation
  const span = tracer.startSpan(`web_vital.${name}`, {
    attributes: {
      ...attributes,
      'vital.value': metric.value,
      'vital.delta': metric.delta
    }
  });
  span.end();
}

// Register callbacks
onLCP((metric) => reportVital('lcp', metric));
onCLS((metric) => reportVital('cls', metric));
onINP((metric) => reportVital('inp', metric));
onFCP((metric) => reportVital('fcp', metric));
onTTFB((metric) => reportVital('ttfb', metric));
```

## Error Recording

### Global Error Handler

```typescript
import { trace, SpanStatusCode } from '@opentelemetry/api';

const tracer = trace.getTracer('error-handler');

// Uncaught errors
window.addEventListener('error', (event) => {
  const span = tracer.startSpan('error.uncaught');

  span.recordException({
    name: event.error?.name || 'Error',
    message: event.message,
    stack: event.error?.stack
  });

  span.setStatus({ code: SpanStatusCode.ERROR, message: event.message });
  span.setAttribute('error.type', 'uncaught');
  span.setAttribute('error.filename', event.filename || 'unknown');
  span.setAttribute('error.lineno', event.lineno);
  span.setAttribute('error.colno', event.colno);
  span.setAttribute('page.url', window.location.href);
  span.end();
});

// Unhandled promise rejections
window.addEventListener('unhandledrejection', (event) => {
  const span = tracer.startSpan('error.unhandled_rejection');

  const message = event.reason instanceof Error
    ? event.reason.message
    : String(event.reason);

  span.recordException({
    name: 'UnhandledRejection',
    message,
    stack: event.reason?.stack
  });

  span.setStatus({ code: SpanStatusCode.ERROR, message });
  span.setAttribute('error.type', 'unhandled_rejection');
  span.setAttribute('page.url', window.location.href);
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
  boundaryName?: string;
}

interface State {
  hasError: boolean;
}

export class TracedErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
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
      return this.props.fallback;
    }
    return this.props.children;
  }
}
```

## Context Propagation

### Preserving Context Across Async Boundaries

```typescript
import { context, trace } from '@opentelemetry/api';

const tracer = trace.getTracer('async');

// Context is automatically preserved with ZoneContextManager
// But if you need manual control:

function manualContextPropagation() {
  tracer.startActiveSpan('parent', (parentSpan) => {
    const currentContext = context.active();

    // Context preserved in promise
    Promise.resolve().then(() => {
      context.with(currentContext, () => {
        tracer.startActiveSpan('child', (childSpan) => {
          // childSpan is connected to parentSpan
          childSpan.end();
        });
      });
    });

    // Context preserved in setTimeout
    const ctx = context.active();
    setTimeout(() => {
      context.with(ctx, () => {
        tracer.startActiveSpan('delayed-child', (span) => {
          span.end();
        });
      });
    }, 100);

    parentSpan.end();
  });
}
```

## Best Practices

### 1. Naming Conventions

```typescript
// Good: verb.noun or domain.action
'products.search'
'cart.add_item'
'checkout.complete'
'route.navigate'
'form.submit.contact'

// Bad: too generic or too specific
'doSomething'
'click'
'POST /api/v1/products/12345/reviews'
```

### 2. Attribute Guidelines

```typescript
// Good: bounded cardinality, useful for filtering
span.setAttribute('product.category', 'electronics');
span.setAttribute('user.tier', 'premium');
span.setAttribute('error.type', 'validation');

// Bad: unbounded cardinality or PII
span.setAttribute('user.email', email);          // PII
span.setAttribute('product.id', productId);      // High cardinality
span.setAttribute('request.body', JSON.stringify(body));  // Too large
```

### 3. Don't Over-Instrument

```typescript
// Bad: span for every tiny operation
items.forEach(item => {
  tracer.startActiveSpan('process.item', (span) => {
    process(item);
    span.end();
  });
});

// Good: one span with events
tracer.startActiveSpan('process.batch', (span) => {
  span.setAttribute('batch.size', items.length);
  items.forEach((item, i) => {
    process(item);
    if (i % 100 === 0) {
      span.addEvent('progress', { processed: i });
    }
  });
  span.end();
});
```

### 4. Always End Spans

```typescript
// Use try/finally pattern
tracer.startActiveSpan('operation', (span) => {
  try {
    doWork();
  } catch (error) {
    span.recordException(error);
    span.setStatus({ code: SpanStatusCode.ERROR });
    throw error;
  } finally {
    span.end(); // Always called
  }
});
```

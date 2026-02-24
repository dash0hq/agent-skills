# Manual Instrumentation Reference

Guide to creating custom spans, metrics, and logs with OpenTelemetry in Node.js.

## Official Documentation

- [Tracing API](https://opentelemetry.io/docs/languages/js/instrumentation/#create-spans)
- [Metrics API](https://opentelemetry.io/docs/languages/js/instrumentation/#create-and-use-metrics)
- [Context Propagation](https://opentelemetry.io/docs/languages/js/instrumentation/#context-propagation)

## Getting Tracer/Meter/Logger Instances

### Tracer

```typescript
import { trace } from '@opentelemetry/api';

// Get tracer with name and version
const tracer = trace.getTracer('my-service', '1.0.0');

// Or with schema URL
const tracer = trace.getTracer(
  'my-service',
  '1.0.0',
  { schemaUrl: 'https://opentelemetry.io/schemas/1.7.0' }
);
```

### Meter

```typescript
import { metrics } from '@opentelemetry/api';

// Get meter with name and version
const meter = metrics.getMeter('my-service', '1.0.0');
```

### Logger

```typescript
import { logs } from '@opentelemetry/api-logs';

// Get logger with name and version
const logger = logs.getLogger('my-service', '1.0.0');
```

## Creating Custom Spans

### Basic Span Creation

```typescript
import { trace, SpanStatusCode, SpanKind } from '@opentelemetry/api';

const tracer = trace.getTracer('my-service');

// Method 1: startActiveSpan (recommended)
async function processOrder(order: Order) {
  return tracer.startActiveSpan('order.process', async (span) => {
    try {
      // Span is automatically set as active
      // Child spans will be linked automatically
      const result = await doWork(order);
      return result;
    } finally {
      span.end(); // Always end the span
    }
  });
}

// Method 2: startSpan (when you need manual context)
function processOrderManual(order: Order) {
  const span = tracer.startSpan('order.process');

  try {
    const result = doWork(order);
    return result;
  } finally {
    span.end();
  }
}
```

### Span with Options

```typescript
import { SpanKind, context, trace } from '@opentelemetry/api';

const span = tracer.startSpan(
  'external.api.call',
  {
    kind: SpanKind.CLIENT, // CLIENT, SERVER, PRODUCER, CONSUMER, INTERNAL
    attributes: {
      'http.method': 'POST',
      'http.url': 'https://api.example.com/orders'
    },
    links: [
      {
        context: otherSpanContext,
        attributes: { 'link.type': 'related' }
      }
    ],
    startTime: Date.now() // Custom start time
  },
  context.active() // Parent context
);
```

### Span Kinds

```typescript
import { SpanKind } from '@opentelemetry/api';

// SERVER: Handling incoming request
tracer.startSpan('handle.request', { kind: SpanKind.SERVER });

// CLIENT: Making outgoing request
tracer.startSpan('call.api', { kind: SpanKind.CLIENT });

// PRODUCER: Creating a message
tracer.startSpan('publish.event', { kind: SpanKind.PRODUCER });

// CONSUMER: Processing a message
tracer.startSpan('process.message', { kind: SpanKind.CONSUMER });

// INTERNAL: Internal operation (default)
tracer.startSpan('process.data', { kind: SpanKind.INTERNAL });
```

### Adding Attributes

```typescript
// At creation
tracer.startSpan('order.process', {
  attributes: {
    'order.id': order.id,
    'order.total': order.total
  }
});

// After creation
span.setAttribute('order.id', order.id);
span.setAttribute('order.total', order.total);

// Multiple at once
span.setAttributes({
  'order.id': order.id,
  'order.total': order.total,
  'customer.tier': customer.tier
});
```

### Span Events

```typescript
// Add point-in-time events
span.addEvent('cache.lookup', {
  'cache.key': 'user:123',
  'cache.hit': true
});

span.addEvent('validation.complete', {
  'validation.errors': 0
});

// With timestamp
span.addEvent('retry.attempt', {
  'retry.count': 2
}, Date.now());
```

### Recording Errors

```typescript
try {
  await riskyOperation();
} catch (error) {
  // Record exception details
  span.recordException(error);

  // Set error status
  span.setStatus({
    code: SpanStatusCode.ERROR,
    message: error.message
  });

  // Optionally add error attributes
  span.setAttribute('error.type', error.name);
  span.setAttribute('error.recoverable', false);

  throw error; // Re-throw if needed
}
```

### Setting Status

```typescript
import { SpanStatusCode } from '@opentelemetry/api';

// Unset (default) - operation completed, status unknown/irrelevant
span.setStatus({ code: SpanStatusCode.UNSET });

// OK - operation succeeded explicitly
span.setStatus({ code: SpanStatusCode.OK });

// Error - operation failed
span.setStatus({
  code: SpanStatusCode.ERROR,
  message: 'Database connection failed'
});
```

## Context Propagation

### Automatic Context (with startActiveSpan)

```typescript
async function parentOperation() {
  return tracer.startActiveSpan('parent', async (parentSpan) => {
    // Child spans automatically linked
    await childOperation(); // Uses active context

    parentSpan.end();
  });
}

async function childOperation() {
  return tracer.startActiveSpan('child', async (childSpan) => {
    // This span is automatically a child of 'parent'
    childSpan.end();
  });
}
```

### Manual Context Propagation

```typescript
import { context, trace } from '@opentelemetry/api';

// Capture current context
const currentContext = context.active();

// Run code with specific context
context.with(currentContext, () => {
  // Spans created here use currentContext as parent
  const span = tracer.startSpan('operation');
  span.end();
});
```

### Async Context Propagation

```typescript
import { context } from '@opentelemetry/api';

// Context preserved across await
async function asyncOperation() {
  return tracer.startActiveSpan('async.operation', async (span) => {
    await step1(); // Context preserved
    await step2(); // Context preserved
    span.end();
  });
}

// Context preserved with Promise.all
async function parallelOperations() {
  return tracer.startActiveSpan('parallel', async (span) => {
    await Promise.all([
      operation1(), // Child of 'parallel'
      operation2(), // Child of 'parallel'
      operation3()  // Child of 'parallel'
    ]);
    span.end();
  });
}
```

### Context with Callbacks

```typescript
import { context, trace } from '@opentelemetry/api';

// Problem: Context lost in callback
setTimeout(() => {
  const span = tracer.startSpan('callback'); // No parent!
  span.end();
}, 100);

// Solution: Bind context
const ctx = context.active();
setTimeout(context.bind(ctx, () => {
  const span = tracer.startSpan('callback'); // Has correct parent
  span.end();
}), 100);

// Or capture and restore
const ctx = context.active();
setTimeout(() => {
  context.with(ctx, () => {
    const span = tracer.startSpan('callback'); // Has correct parent
    span.end();
  });
}, 100);
```

### HTTP Context Propagation

```typescript
import { context, propagation } from '@opentelemetry/api';

// Inject context into outgoing request headers
function makeOutgoingRequest(url: string, data: any) {
  const headers: Record<string, string> = {};

  // Inject trace context
  propagation.inject(context.active(), headers);

  // headers now contains: traceparent, tracestate

  return fetch(url, {
    method: 'POST',
    headers,
    body: JSON.stringify(data)
  });
}

// Extract context from incoming request
function handleIncomingRequest(req: Request) {
  // Extract trace context from headers
  const extractedContext = propagation.extract(context.active(), req.headers);

  // Run handler with extracted context
  return context.with(extractedContext, () => {
    return tracer.startActiveSpan('handle.request', async (span) => {
      // Process request
      span.end();
    });
  });
}
```

## Recording Metrics

### Counter

```typescript
import { metrics } from '@opentelemetry/api';

const meter = metrics.getMeter('my-service');

// Create counter
const requestCounter = meter.createCounter('http.server.requests', {
  description: 'Total number of HTTP requests',
  unit: '1'
});

// Usage
requestCounter.add(1, {
  'http.method': 'GET',
  'http.route': '/api/users',
  'http.status_code': 200
});
```

### Histogram

```typescript
const requestDuration = meter.createHistogram('http.server.duration', {
  description: 'HTTP request duration',
  unit: 'ms'
});

// Usage
const startTime = Date.now();
// ... process request ...
const duration = Date.now() - startTime;

requestDuration.record(duration, {
  'http.method': 'GET',
  'http.route': '/api/users',
  'http.status_code': 200
});
```

### UpDownCounter

```typescript
const activeConnections = meter.createUpDownCounter('db.connections.active', {
  description: 'Number of active database connections',
  unit: '1'
});

// Usage
function onConnectionOpened() {
  activeConnections.add(1);
}

function onConnectionClosed() {
  activeConnections.add(-1);
}
```

### Observable Gauge

```typescript
// Register callback for periodic measurement
meter.createObservableGauge('process.memory.heap', {
  description: 'Heap memory usage in bytes',
  unit: 'By'
}, (observableResult) => {
  observableResult.observe(process.memoryUsage().heapUsed, {
    'memory.type': 'heap'
  });
});

// With multiple observations
meter.createObservableGauge('cpu.usage', {
  description: 'CPU usage by core'
}, (observableResult) => {
  const cpus = os.cpus();
  cpus.forEach((cpu, index) => {
    const usage = calculateCpuUsage(cpu);
    observableResult.observe(usage, { 'cpu.core': index.toString() });
  });
});
```

### Observable Counter

```typescript
// For monotonically increasing values you don't control
meter.createObservableCounter('process.cpu.time', {
  description: 'Total CPU time in seconds',
  unit: 's'
}, (observableResult) => {
  const usage = process.cpuUsage();
  observableResult.observe(
    (usage.user + usage.system) / 1_000_000,
    { 'cpu.mode': 'total' }
  );
});
```

### Batch Observable

```typescript
// Multiple metrics from same callback
meter.addBatchObservableCallback(
  (observableResult) => {
    const memUsage = process.memoryUsage();

    heapUsedGauge.observe(memUsage.heapUsed);
    heapTotalGauge.observe(memUsage.heapTotal);
    rssGauge.observe(memUsage.rss);
  },
  [heapUsedGauge, heapTotalGauge, rssGauge]
);
```

## Injecting Trace Context into Logs

### With pino

```typescript
import pino from 'pino';
import { trace, context } from '@opentelemetry/api';

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

// Usage
logger.info({ order_id: '123' }, 'Processing order');
// Output: {"trace_id":"abc...","span_id":"def...","order_id":"123","msg":"Processing order"}
```

### With winston

```typescript
import winston from 'winston';
import { trace, context } from '@opentelemetry/api';

const addTraceContext = winston.format((info) => {
  const span = trace.getSpan(context.active());
  if (span) {
    const spanContext = span.spanContext();
    info.trace_id = spanContext.traceId;
    info.span_id = spanContext.spanId;
  }
  return info;
});

const logger = winston.createLogger({
  format: winston.format.combine(
    addTraceContext(),
    winston.format.json()
  ),
  transports: [new winston.transports.Console()]
});
```

### With OTel Logs API

```typescript
import { logs, SeverityNumber } from '@opentelemetry/api-logs';
import { trace, context } from '@opentelemetry/api';

const logger = logs.getLogger('my-service');

function logWithContext(level: SeverityNumber, message: string, attrs: Record<string, any>) {
  const span = trace.getSpan(context.active());

  logger.emit({
    severityNumber: level,
    severityText: SeverityNumber[level],
    body: message,
    attributes: attrs,
    context: span?.spanContext()
  });
}

// Usage
logWithContext(SeverityNumber.INFO, 'Order processed', {
  'order.id': order.id,
  'order.total': order.total
});
```

## Complete Example: Business Operation

```typescript
import { trace, metrics, SpanStatusCode, context } from '@opentelemetry/api';
import pino from 'pino';

const tracer = trace.getTracer('order-service', '1.0.0');
const meter = metrics.getMeter('order-service', '1.0.0');

// Metrics
const ordersCreated = meter.createCounter('orders.created');
const orderProcessingDuration = meter.createHistogram('orders.processing.duration');
const ordersInProgress = meter.createUpDownCounter('orders.in_progress');

// Logger with trace context
const logger = pino({
  mixin() {
    const span = trace.getSpan(context.active());
    if (!span) return {};
    const ctx = span.spanContext();
    return { trace_id: ctx.traceId, span_id: ctx.spanId };
  }
});

interface Order {
  id: string;
  customerId: string;
  items: Array<{ productId: string; quantity: number; price: number }>;
  total: number;
}

async function processOrder(orderData: CreateOrderInput): Promise<Order> {
  const startTime = Date.now();
  ordersInProgress.add(1);

  return tracer.startActiveSpan('order.process', async (span) => {
    try {
      // Set initial attributes
      span.setAttribute('order.customer_id', orderData.customerId);
      span.setAttribute('order.items_count', orderData.items.length);

      logger.info({ customer_id: orderData.customerId }, 'Starting order processing');

      // Validate order
      span.addEvent('validation.start');
      await validateOrder(orderData);
      span.addEvent('validation.complete');

      // Calculate total
      const total = await calculateTotal(orderData.items);
      span.setAttribute('order.total', total);

      // Create order in database
      const order = await createOrderInDb({
        ...orderData,
        total
      });

      span.setAttribute('order.id', order.id);

      // Send confirmation
      await sendOrderConfirmation(order);

      // Record metrics
      ordersCreated.add(1, {
        'customer.tier': orderData.customerTier,
        'order.source': orderData.source
      });

      logger.info({ order_id: order.id }, 'Order processed successfully');

      return order;
    } catch (error) {
      span.recordException(error);
      span.setStatus({
        code: SpanStatusCode.ERROR,
        message: error.message
      });

      logger.error({ error: error.message }, 'Order processing failed');

      throw error;
    } finally {
      ordersInProgress.add(-1);
      orderProcessingDuration.record(Date.now() - startTime, {
        'status': span.status.code === SpanStatusCode.ERROR ? 'error' : 'success'
      });
      span.end();
    }
  });
}

// Child operation - automatically linked
async function validateOrder(order: CreateOrderInput): Promise<void> {
  return tracer.startActiveSpan('order.validate', async (span) => {
    try {
      // Validation logic
      if (!order.items.length) {
        throw new Error('Order must have at least one item');
      }

      span.setAttribute('validation.rules_checked', 5);
    } finally {
      span.end();
    }
  });
}

// Another child operation
async function calculateTotal(items: OrderItem[]): Promise<number> {
  return tracer.startActiveSpan('order.calculate_total', async (span) => {
    try {
      const total = items.reduce((sum, item) => sum + item.price * item.quantity, 0);
      span.setAttribute('order.subtotal', total);
      return total;
    } finally {
      span.end();
    }
  });
}
```

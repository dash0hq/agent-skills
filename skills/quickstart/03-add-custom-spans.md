# Add Custom Spans

Time: ~10 minutes

Auto-instrumentation covers HTTP, databases, and frameworks. But what about *your* code? This guide shows you how to add custom spans for business logic.

## When to Add Custom Spans

Add custom spans when you want to:
- Track time spent in a specific function
- Add business context (order ID, user type, etc.)
- See where time is spent within a request
- Debug performance issues in your code

## Prerequisites

- Completed [02-send-to-dash0.md](./02-send-to-dash0.md) (Dash0 configured)
- Express app from previous guide

## Step 1: Get the Tracer

The tracer is how you create spans. Add to your app:

```javascript
// Get the OTel API
const { trace } = require('@opentelemetry/api');

// Get a tracer for your service
// The name helps identify where spans came from
const tracer = trace.getTracer('my-service');
```

## Step 2: Create Your First Custom Span

Here's a function that processes an order:

```javascript
const { trace, SpanStatusCode } = require('@opentelemetry/api');
const tracer = trace.getTracer('order-service');

async function processOrder(orderId, items) {
  // startActiveSpan:
  // - Creates a span
  // - Makes it the "active" span (child spans link to it)
  // - Calls your function
  // - Ends the span when done

  return tracer.startActiveSpan('order.process', async (span) => {
    try {
      // ADD CONTEXT: Attributes describe what this span is about
      span.setAttribute('order.id', orderId);
      span.setAttribute('order.items_count', items.length);

      // Your business logic
      const total = await calculateTotal(items);
      span.setAttribute('order.total', total);

      const result = await saveOrder(orderId, items, total);

      return result;
    } catch (error) {
      // RECORD ERRORS: This is critical for debugging
      span.recordException(error);
      span.setStatus({
        code: SpanStatusCode.ERROR,
        message: error.message
      });
      throw error;
    } finally {
      // ALWAYS END THE SPAN
      // (startActiveSpan does this automatically, but be explicit)
      span.end();
    }
  });
}
```

## Step 3: Nested Spans

Child functions automatically become child spans:

```javascript
async function calculateTotal(items) {
  // This span becomes a child of 'order.process'
  return tracer.startActiveSpan('order.calculate_total', async (span) => {
    try {
      let total = 0;
      for (const item of items) {
        total += item.price * item.quantity;
      }

      span.setAttribute('calculated.total', total);
      return total;
    } finally {
      span.end();
    }
  });
}

async function saveOrder(orderId, items, total) {
  // This is also a child of 'order.process'
  return tracer.startActiveSpan('order.save', async (span) => {
    try {
      span.setAttribute('db.operation', 'INSERT');

      // Simulate database save
      await new Promise(resolve => setTimeout(resolve, 50));

      span.addEvent('order.saved', { 'order.id': orderId });

      return { id: orderId, total, status: 'saved' };
    } finally {
      span.end();
    }
  });
}
```

## Step 4: Complete Example

Create `app-with-spans.js`:

```javascript
const express = require('express');
const { trace, SpanStatusCode } = require('@opentelemetry/api');

const app = express();
app.use(express.json());

// Get tracer
const tracer = trace.getTracer('order-service');

// Business logic with custom spans
async function processOrder(orderId, items) {
  return tracer.startActiveSpan('order.process', async (span) => {
    try {
      span.setAttribute('order.id', orderId);
      span.setAttribute('order.items_count', items.length);

      // Child span: calculate total
      const total = await tracer.startActiveSpan('order.calculate_total', async (calcSpan) => {
        try {
          const sum = items.reduce((acc, item) => acc + item.price * item.quantity, 0);
          calcSpan.setAttribute('calculated.total', sum);
          return sum;
        } finally {
          calcSpan.end();
        }
      });

      span.setAttribute('order.total', total);

      // Child span: save to database
      const saved = await tracer.startActiveSpan('order.save_to_db', async (saveSpan) => {
        try {
          saveSpan.setAttribute('db.system', 'postgresql');
          saveSpan.setAttribute('db.operation', 'INSERT');

          // Simulate database delay
          await new Promise(resolve => setTimeout(resolve, 50));

          saveSpan.addEvent('row_inserted', { 'order.id': orderId });

          return { id: orderId, total, status: 'created' };
        } finally {
          saveSpan.end();
        }
      });

      return saved;
    } catch (error) {
      span.recordException(error);
      span.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
      throw error;
    } finally {
      span.end();
    }
  });
}

// Route
app.post('/orders', async (req, res) => {
  try {
    const orderId = `order_${Date.now()}`;
    const items = req.body.items || [
      { name: 'Widget', price: 9.99, quantity: 2 }
    ];

    const result = await processOrder(orderId, items);

    res.status(201).json(result);
  } catch (error) {
    res.status(500).json({ error: error.message });
  }
});

app.listen(3000, () => {
  console.log('Server on http://localhost:3000');
  console.log('Test with: curl -X POST http://localhost:3000/orders -H "Content-Type: application/json" -d \'{"items":[{"name":"Book","price":15.99,"quantity":1}]}\'');
});
```

## Step 5: Run and Test

```bash
# Set your Dash0 credentials
export DASH0_AUTH_TOKEN="your-token-here"
export DASH0_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"

# Run app
node --import ./instrumentation.js app-with-spans.js

# Create an order
curl -X POST http://localhost:3000/orders \
  -H "Content-Type: application/json" \
  -d '{"items":[{"name":"Book","price":15.99,"quantity":1}]}'
```

## Step 6: View in Dash0

Open [app.dash0.com](https://app.dash0.com), go to Tracing → Traces, select "order-service".

You'll see:

```
POST /orders ─────────────────────────────────────────────── 65ms
  └─ middleware - query ───── 0.2ms
  └─ middleware - expressInit ── 0.1ms
  └─ middleware - jsonParser ── 0.5ms
  └─ request handler - /orders ─────────────────────────── 55ms
      └─ order.process ────────────────────────────────── 52ms
          └─ order.calculate_total ──── 0.3ms
          └─ order.save_to_db ───────────────────────── 50ms
```

Click on `order.process` to see the attributes:
- `order.id: order_1234567890`
- `order.items_count: 1`
- `order.total: 15.99`

## Best Practices

### 1. Name Spans Well

```javascript
// Good: verb.noun pattern
'order.process'
'payment.authorize'
'user.authenticate'
'email.send'

// Bad: too vague or too specific
'doStuff'
'processOrderForUserIdAbcAndSendEmail'
'/api/v1/orders'  // URLs belong in attributes, not names
```

### 2. Add Useful Attributes

```javascript
// Good: bounded values, useful for filtering
span.setAttribute('order.id', orderId);
span.setAttribute('customer.tier', 'premium');
span.setAttribute('payment.method', 'credit_card');

// Bad: unbounded or PII
span.setAttribute('user.email', email);  // PII risk
span.setAttribute('timestamp', Date.now());  // Use span timing
span.setAttribute('request.body', JSON.stringify(body));  // Too large
```

### 3. Always Handle Errors

```javascript
try {
  // work
} catch (error) {
  span.recordException(error);  // Captures stack trace
  span.setStatus({
    code: SpanStatusCode.ERROR,
    message: error.message
  });
  throw error;
} finally {
  span.end();  // Always end, even on error
}
```

### 4. Use Events for Point-in-Time Occurrences

```javascript
// Events mark specific moments within a span
span.addEvent('cache.miss', { 'cache.key': key });
span.addEvent('retry.attempt', { 'attempt': 2 });
span.addEvent('validation.complete', { 'errors': 0 });
```

## Common Patterns

### Wrap External Calls

```javascript
async function callExternalApi(url, data) {
  return tracer.startActiveSpan('external.api.call', async (span) => {
    try {
      span.setAttribute('http.url', url);
      span.setAttribute('http.method', 'POST');

      const response = await fetch(url, {
        method: 'POST',
        body: JSON.stringify(data)
      });

      span.setAttribute('http.status_code', response.status);

      if (!response.ok) {
        span.setStatus({ code: SpanStatusCode.ERROR });
      }

      return response.json();
    } finally {
      span.end();
    }
  });
}
```

### Batch Operations

```javascript
async function processBatch(items) {
  // Don't create a span per item in a large batch
  return tracer.startActiveSpan('batch.process', async (span) => {
    try {
      span.setAttribute('batch.size', items.length);

      for (let i = 0; i < items.length; i++) {
        await processItem(items[i]);
        // Use events for progress, not spans
        if (i % 100 === 0) {
          span.addEvent('batch.progress', { 'processed': i });
        }
      }

      span.setAttribute('batch.processed', items.length);
    } finally {
      span.end();
    }
  });
}
```

## What You Learned

1. `tracer.startActiveSpan()` creates spans linked to parents
2. `span.setAttribute()` adds searchable context
3. `span.recordException()` captures errors with stack traces
4. `span.addEvent()` marks moments within a span
5. Always `span.end()` when done (use `finally` block)

## Next Steps

- Review the full [Express API example](../@otel-nodejs/examples/express-api.md)
- Learn about [metrics](../@otel-telemetry/references/signals/metrics.md)
- Read the [LEARNING-PATH.md](../LEARNING-PATH.md) for what to learn next

# Common Mistakes & Troubleshooting

Solutions to the most common OpenTelemetry problems, especially for newcomers.

---

## No Traces Appearing

### Problem: "I installed the packages but nothing happens"

**Cause:** Forgot the `--import` flag.

```bash
# WRONG - instrumentation loads too late
node app.js

# RIGHT - instrumentation loads first
node --import ./instrumentation.js app.js
```

**Why this matters:** OTel works by wrapping modules (http, express) before they're used. Without `--import`, your app imports those modules first, then OTel loads, and there's nothing left to wrap.

**Verify:** Add `console.log('OTel loaded')` to your instrumentation file. If you don't see it at startup, the file isn't loading.

---

### Problem: "Traces appear locally but not in Dash0/backend"

**Cause 1:** Wrong endpoint URL or missing auth token.

```javascript
// WRONG - missing auth token
traceExporter: new OTLPTraceExporter({
  url: 'https://ingress.eu-west-1.dash0.com:4317'
})

// RIGHT - include auth token
traceExporter: new OTLPTraceExporter({
  url: 'https://ingress.eu-west-1.dash0.com:4317',
  headers: {
    'Authorization': `Bearer ${process.env.DASH0_AUTH_TOKEN}`
  }
})
```

**Cause 2:** Wrong region.

```bash
# Check your Dash0 region matches your account
# EU: ingress.eu-west-1.dash0.com
# US: ingress.us-east-1.dash0.com
```

**Cause 3:** Auth token invalid or expired.

```bash
# Verify your token is set
echo $DASH0_AUTH_TOKEN
```

**Cause 4:** Wrong service name filter in UI.

Check that the service name in your code matches what you're searching for in Dash0.

---

### Problem: "Console exporter shows traces but OTLP exporter doesn't send them"

**Cause:** Exporter configuration mismatch.

```javascript
// Check you imported the right exporter
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-proto');
// NOT: require('@opentelemetry/exporter-trace-otlp-grpc') with HTTP endpoint
```

**Debug:** Temporarily add both exporters:

```javascript
const { ConsoleSpanExporter } = require('@opentelemetry/sdk-trace-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-proto');
const { BatchSpanProcessor, SimpleSpanProcessor } = require('@opentelemetry/sdk-trace-node');

// See traces immediately in console
provider.addSpanProcessor(new SimpleSpanProcessor(new ConsoleSpanExporter()));

// Also send to backend
provider.addSpanProcessor(new BatchSpanProcessor(new OTLPTraceExporter({
  url: 'http://localhost:4318/v1/traces'
})));
```

---

## Incomplete Traces / Missing Spans

### Problem: "My spans don't have a parent"

**Cause:** Async context lost.

```javascript
// WRONG - context lost in callback
setTimeout(() => {
  const span = tracer.startSpan('orphan'); // No parent!
  span.end();
}, 100);

// RIGHT - preserve context
const { context } = require('@opentelemetry/api');
const ctx = context.active();
setTimeout(() => {
  context.with(ctx, () => {
    const span = tracer.startSpan('connected'); // Has parent!
    span.end();
  });
}, 100);
```

**Better approach:** Use `startActiveSpan` which handles context:

```javascript
// This automatically links child spans
await tracer.startActiveSpan('parent', async (parentSpan) => {
  await tracer.startActiveSpan('child', async (childSpan) => {
    // child is automatically linked to parent
    childSpan.end();
  });
  parentSpan.end();
});
```

---

### Problem: "Auto-instrumentation didn't detect my Express/Fastify app"

**Cause 1:** Import order wrong.

```javascript
// WRONG - express imported before OTel
const express = require('express');
require('./instrumentation'); // Too late!

// RIGHT - use --import flag
// instrumentation.js loads first, wraps express
// then your app.js imports express (already wrapped)
```

**Cause 2:** Instrumentation not included.

```javascript
// Make sure you have the right instrumentation
const { ExpressInstrumentation } = require('@opentelemetry/instrumentation-express');

const sdk = new NodeSDK({
  instrumentations: [
    new HttpInstrumentation(),
    new ExpressInstrumentation(), // Don't forget this!
  ]
});
```

---

### Problem: "Some HTTP requests create spans, others don't"

**Cause:** Requests being filtered.

```javascript
// Check your ignoreIncomingPaths config
new HttpInstrumentation({
  ignoreIncomingPaths: ['/health', '/ready'], // These won't create spans
})
```

**Debug:** Temporarily remove all ignore patterns.

---

## Metrics Issues

### Problem: "Metrics appear but traces don't (or vice versa)"

**Cause:** Different exporters configured for each signal.

```javascript
// Traces going to Dash0
traceExporter: new OTLPTraceExporter({
  url: 'http://localhost:4318/v1/traces'
}),

// But metrics going nowhere (no exporter!)
// Or going to different endpoint
metricReader: new PeriodicExportingMetricReader({
  exporter: new OTLPMetricExporter({
    url: 'http://localhost:4318/v1/metrics' // Same endpoint, different path
  })
})
```

**Solution:** Verify both use the same OTLP endpoint if using same backend.

---

### Problem: "Metric cardinality explosion / huge bills"

**Cause:** Unbounded labels.

```javascript
// WRONG - user_id has millions of values
counter.add(1, { user_id: userId });

// RIGHT - use bounded values
counter.add(1, { user_tier: 'premium' }); // Only 3-4 possible values
```

See [@otel-telemetry/references/signals/metrics.md](./@otel-telemetry/references/signals/metrics.md#cardinality-control) for detailed guidance.

---

## Performance Issues

### Problem: "High memory usage with OTel enabled"

**Cause 1:** Creating spans in tight loops.

```javascript
// WRONG - creates millions of spans
for (const item of hugeArray) {
  const span = tracer.startSpan('item'); // Memory explosion
  process(item);
  span.end();
}

// RIGHT - one span, use events
const span = tracer.startSpan('batch');
for (const item of hugeArray) {
  span.addEvent('item.processed');
  process(item);
}
span.end();
```

**Cause 2:** Span/metric buffer too large.

```javascript
// Reduce buffer sizes if needed
new BatchSpanProcessor(exporter, {
  maxQueueSize: 1024,      // Default is 2048
  maxExportBatchSize: 256  // Default is 512
});
```

---

### Problem: "App slower after adding OTel"

**Cause:** Usually sampling misconfiguration or exporter issues.

**Quick fixes:**
1. Enable sampling: `new TraceIdRatioBasedSampler(0.1)` for 10%
2. Use batch processor (default), not simple processor
3. Check exporter isn't blocking (network issues)

```javascript
// Verify you're using BatchSpanProcessor, not SimpleSpanProcessor
const sdk = new NodeSDK({
  // BatchSpanProcessor is the default in NodeSDK
  traceExporter: new OTLPTraceExporter({ ... })
});
```

---

## Configuration Issues

### Problem: "Environment variables not working"

**Verify variables are set:**

```bash
# Check what's set
env | grep OTEL

# Common variables
export OTEL_SERVICE_NAME=my-service
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_TRACES_SAMPLER=parentbased_traceidratio
export OTEL_TRACES_SAMPLER_ARG=0.1
```

**Note:** Environment variables are read at SDK initialization. Setting them after won't work.

---

### Problem: "TypeScript errors with OTel packages"

**Solution:** Install type packages:

```bash
npm install -D @types/node
```

And make sure your `tsconfig.json` includes:

```json
{
  "compilerOptions": {
    "esModuleInterop": true,
    "moduleResolution": "node"
  }
}
```

---

## Shutdown Issues

### Problem: "Some spans lost on app shutdown"

**Cause:** Not waiting for export to complete.

```javascript
// WRONG - process exits before spans exported
process.on('SIGTERM', () => process.exit(0));

// RIGHT - flush before exit
process.on('SIGTERM', async () => {
  await sdk.shutdown(); // Flushes pending spans
  process.exit(0);
});
```

---

## Quick Diagnostic Checklist

Run through this when traces aren't working:

```
[ ] 1. Using --import flag?
      node --import ./instrumentation.js app.js

[ ] 2. Instrumentation file loads?
      Add console.log at top of instrumentation.js

[ ] 3. Service name set?
      Check OTEL_SERVICE_NAME or serviceName config

[ ] 4. Endpoint correct?
      4317 = gRPC, 4318 = HTTP
      Check protocol matches exporter type

[ ] 5. Backend running?
      curl http://localhost:4318/v1/traces -X POST -d '{}'

[ ] 6. Sampling not 0?
      Check sampler config isn't dropping everything

[ ] 7. Checking right service in UI?
      Service name must match exactly
```

---

## Getting Help

If you're still stuck:

1. Enable debug logging:
   ```bash
   export OTEL_LOG_LEVEL=debug
   ```

2. Check the [official OTel JS documentation](https://opentelemetry.io/docs/languages/js/)

3. Search [OTel JS GitHub issues](https://github.com/open-telemetry/opentelemetry-js/issues)

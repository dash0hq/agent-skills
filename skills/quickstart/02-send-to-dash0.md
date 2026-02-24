# See Traces in Dash0

Time: ~10 minutes | Requires Dash0 account

Console output is great for learning, but you want to *see* your traces. Dash0 provides a beautiful, OpenTelemetry-native UI for exploring traces, metrics, and logs.

## What You'll Get

```
Your App  →  Dash0  →  Beautiful trace visualization
```

## Prerequisites

- Completed [01-first-trace.md](./01-first-trace.md) or equivalent
- A Dash0 account ([sign up free](https://www.dash0.com))

## Step 1: Get Your Dash0 Credentials

1. Log into [Dash0](https://app.dash0.com)
2. Go to **Settings** → **Auth Tokens**
3. Create a new token or copy an existing one
4. Note your ingress endpoint (e.g., `ingress.eu-west-1.dash0.com` or `ingress.us-east-1.dash0.com`)

You'll need:
- **Endpoint**: `https://ingress.<region>.dash0.com:4317`
- **Auth Token**: Your Dash0 auth token

## Step 2: Install OTLP Exporter

```bash
npm install @opentelemetry/exporter-trace-otlp-grpc @grpc/grpc-js
```

## Step 3: Update Instrumentation

Replace your `instrumentation.js`:

```javascript
// instrumentation.js
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
const { HttpInstrumentation } = require('@opentelemetry/instrumentation-http');

// Get credentials from environment variables
const DASH0_ENDPOINT = process.env.DASH0_ENDPOINT || 'https://ingress.eu-west-1.dash0.com:4317';
const DASH0_AUTH_TOKEN = process.env.DASH0_AUTH_TOKEN;

if (!DASH0_AUTH_TOKEN) {
  console.error('DASH0_AUTH_TOKEN environment variable is required');
  process.exit(1);
}

const sdk = new NodeSDK({
  serviceName: process.env.OTEL_SERVICE_NAME || 'my-service',

  // Send traces to Dash0
  traceExporter: new OTLPTraceExporter({
    url: DASH0_ENDPOINT,
    headers: {
      'Authorization': `Bearer ${DASH0_AUTH_TOKEN}`
    }
  }),

  instrumentations: [new HttpInstrumentation()]
});

sdk.start();

console.log(`OpenTelemetry initialized - sending traces to Dash0`);

process.on('SIGTERM', () => {
  sdk.shutdown().finally(() => process.exit(0));
});
```

## Step 4: Set Environment Variables

```bash
# Set your Dash0 credentials
export DASH0_AUTH_TOKEN="your-auth-token-here"
export DASH0_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"  # or us-east-1
export OTEL_SERVICE_NAME="my-first-service"
```

## Step 5: Run Your App

```bash
node --import ./instrumentation.js app.js
```

## Step 6: Generate Some Traffic

```bash
# Make several requests
curl http://localhost:3000
curl http://localhost:3000
curl http://localhost:3000
```

## Step 7: View in Dash0

1. Open [app.dash0.com](https://app.dash0.com)
2. Go to **Tracing** → **Traces**
3. Select your service from the filter
4. Click on a trace to see the waterfall view

## What You'll See

```
┌─────────────────────────────────────────────────────────────────┐
│ Dash0 Traces                                                    │
├─────────────────────────────────────────────────────────────────┤
│ Service: my-first-service        Time Range: Last 15 minutes    │
│                                                                 │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ GET /                                            52.3ms     │ │
│ │ my-first-service                                            │ │
│ │ ● Success                                                   │ │
│ ├─────────────────────────────────────────────────────────────┤ │
│ │ GET /                                            48.1ms     │ │
│ │ my-first-service                                            │ │
│ │ ● Success                                                   │ │
│ └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

Click on a trace to see:
- Full waterfall timeline
- Span attributes
- Related logs (if configured)
- Service map

## Step 8: Add Express for Richer Traces

Let's see nested spans. Install Express:

```bash
npm install express @opentelemetry/instrumentation-express
```

Update `instrumentation.js`:

```javascript
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
const { HttpInstrumentation } = require('@opentelemetry/instrumentation-http');
const { ExpressInstrumentation } = require('@opentelemetry/instrumentation-express');

const DASH0_ENDPOINT = process.env.DASH0_ENDPOINT || 'https://ingress.eu-west-1.dash0.com:4317';
const DASH0_AUTH_TOKEN = process.env.DASH0_AUTH_TOKEN;

const sdk = new NodeSDK({
  serviceName: process.env.OTEL_SERVICE_NAME || 'express-service',

  traceExporter: new OTLPTraceExporter({
    url: DASH0_ENDPOINT,
    headers: {
      'Authorization': `Bearer ${DASH0_AUTH_TOKEN}`
    }
  }),

  instrumentations: [
    new HttpInstrumentation(),
    new ExpressInstrumentation()
  ]
});

sdk.start();
```

Create `express-app.js`:

```javascript
const express = require('express');
const app = express();

app.get('/', (req, res) => {
  res.json({ message: 'Hello!' });
});

app.get('/users/:id', (req, res) => {
  // Simulate database call
  setTimeout(() => {
    res.json({ id: req.params.id, name: 'Alice' });
  }, 100);
});

app.listen(3000, () => {
  console.log('Express app on http://localhost:3000');
});
```

Run and test:

```bash
node --import ./instrumentation.js express-app.js

# In another terminal
curl http://localhost:3000/users/42
```

In Dash0 you'll see nested spans:

```
GET /users/:id  ─────────────────────────────────── 105ms
  └─ middleware - query     ── 0.5ms
  └─ middleware - expressInit  ── 0.3ms
  └─ request handler - /users/:id  ────────────── 100ms
```

## Using Environment Variables (Recommended)

For production, use standard OTel environment variables:

```bash
export OTEL_SERVICE_NAME="my-service"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer your-token-here"
```

Then simplify your instrumentation:

```javascript
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
const { HttpInstrumentation } = require('@opentelemetry/instrumentation-http');

// SDK reads from OTEL_* environment variables automatically
const sdk = new NodeSDK({
  traceExporter: new OTLPTraceExporter(),
  instrumentations: [new HttpInstrumentation()]
});

sdk.start();
```

## Troubleshooting

**Traces not appearing in Dash0?**

1. Check your auth token is valid:
   ```bash
   echo $DASH0_AUTH_TOKEN  # Should print your token
   ```

2. Check your endpoint region matches your account:
   - EU: `ingress.eu-west-1.dash0.com`
   - US: `ingress.us-east-1.dash0.com`

3. Verify network connectivity:
   ```bash
   # Test gRPC endpoint
   grpcurl -H "Authorization: Bearer $DASH0_AUTH_TOKEN" \
     ingress.eu-west-1.dash0.com:4317 list
   ```

4. Check for errors in your app's console output

5. Ensure the auth token has the correct permissions

**Getting "Unauthenticated" errors?**
- Verify your token is correct
- Check you're using `Bearer` prefix in the Authorization header
- Ensure the token hasn't expired

## What You Learned

1. Dash0 accepts OTLP traces natively (no collector needed)
2. Authentication uses a Bearer token in headers
3. Standard OTEL_* environment variables work
4. Dash0 shows traces, metrics, and logs in one place

## Next Steps

- [Add custom spans](./03-add-custom-spans.md) for your business logic
- Explore Dash0's service map and analytics
- [Add metrics](../@otel-telemetry/references/signals/metrics.md) to your instrumentation

## Dash0 Resources

- [Dash0 Documentation](https://www.dash0.com/documentation)
- [OpenTelemetry Integration Guide](https://www.dash0.com/documentation/opentelemetry)
- [Dash0 Support](https://www.dash0.com/support)

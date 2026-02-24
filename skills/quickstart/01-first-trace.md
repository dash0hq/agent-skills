# Your First Trace (Console Output)

Time: ~5 minutes | No backend required

This guide gets your first OpenTelemetry trace working with console output. Perfect for understanding what traces look like before setting up a real backend.

## What You'll Build

A simple HTTP server that:
1. Receives requests
2. Automatically creates trace spans
3. Prints them to your console

## Prerequisites

- Node.js 18+
- npm

## Step 1: Create Project

```bash
mkdir otel-quickstart && cd otel-quickstart
npm init -y
```

## Step 2: Install Packages

```bash
npm install @opentelemetry/sdk-node \
  @opentelemetry/sdk-trace-node \
  @opentelemetry/instrumentation-http
```

**What these packages do:**
- `sdk-node`: The main SDK that coordinates everything
- `sdk-trace-node`: Trace processing and export
- `instrumentation-http`: Auto-instruments Node's http module

## Step 3: Create Instrumentation File

Create `instrumentation.js`:

```javascript
// instrumentation.js
//
// WHY THIS FILE EXISTS:
// OpenTelemetry works by wrapping modules (http, express, etc.)
// BEFORE your code imports them. This file must run first.

const { NodeSDK } = require('@opentelemetry/sdk-node');
const { ConsoleSpanExporter } = require('@opentelemetry/sdk-trace-node');
const { HttpInstrumentation } = require('@opentelemetry/instrumentation-http');

const sdk = new NodeSDK({
  // SERVICE NAME: Identifies your service in traces
  // Always set this - it's how you'll find your traces later
  serviceName: 'my-first-service',

  // EXPORTER: Where traces go
  // ConsoleSpanExporter = print to terminal (great for learning)
  // In production, you'd use OTLPTraceExporter instead
  traceExporter: new ConsoleSpanExporter(),

  // INSTRUMENTATIONS: What to auto-trace
  // HttpInstrumentation wraps Node's http module
  // Every incoming/outgoing HTTP request creates a span
  instrumentations: [
    new HttpInstrumentation({
      // Optional: ignore health checks to reduce noise
      ignoreIncomingPaths: ['/health']
    })
  ]
});

// START THE SDK
// This registers the instrumentations
sdk.start();

// GRACEFUL SHUTDOWN
// Ensures pending spans are exported before process exits
process.on('SIGTERM', () => {
  sdk.shutdown()
    .then(() => console.log('OTel shut down'))
    .finally(() => process.exit(0));
});

console.log('OpenTelemetry initialized');
```

## Step 4: Create Your App

Create `app.js`:

```javascript
// app.js
const http = require('http');

const server = http.createServer((req, res) => {
  // Simulate some work
  const start = Date.now();
  while (Date.now() - start < 50) {} // 50ms delay

  res.writeHead(200, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify({ message: 'Hello from traced server!' }));
});

const PORT = 3000;
server.listen(PORT, () => {
  console.log(`Server running at http://localhost:${PORT}`);
  console.log('Make a request with: curl http://localhost:3000');
});
```

## Step 5: Run Your App

```bash
node --import ./instrumentation.js app.js
```

**Why `--import`?** This flag ensures `instrumentation.js` runs *before* your app imports any modules. OTel needs to wrap `http` before you use it.

## Step 6: Make a Request

In another terminal:

```bash
curl http://localhost:3000
```

## Step 7: Read Your Trace

You'll see output like this:

```javascript
{
  resource: {
    attributes: {
      'service.name': 'my-first-service',
      'telemetry.sdk.language': 'nodejs',
      'telemetry.sdk.name': 'opentelemetry',
      'telemetry.sdk.version': '1.x.x'
    }
  },
  traceId: 'abc123def456...',           // Unique ID for this request
  parentId: undefined,                   // No parent = root span
  name: 'GET',                           // HTTP method
  id: 'span123...',                      // This span's ID
  kind: 1,                               // 1 = SERVER
  timestamp: 1699900000000000,           // When span started (microseconds)
  duration: [0, 52000000],               // Duration (seconds, nanoseconds)
  attributes: {
    'http.url': 'http://localhost:3000/',
    'http.method': 'GET',
    'http.status_code': 200,
    'http.target': '/',
    'net.host.name': 'localhost',
    'net.host.port': 3000
  },
  status: { code: 0 },                   // 0 = UNSET (success)
  events: [],
  links: []
}
```

## Understanding the Output

```
┌────────────────────────────────────────────────────────────────┐
│ TRACE (traceId: abc123...)                                     │
│   = The full journey of one request                            │
│                                                                │
│   ┌──────────────────────────────────────────────────────────┐ │
│   │ SPAN (id: span123...)                                    │ │
│   │   name: "GET"                                            │ │
│   │   kind: SERVER (received incoming request)               │ │
│   │   duration: 52ms                                         │ │
│   │   attributes:                                            │ │
│   │     - http.method: GET                                   │ │
│   │     - http.status_code: 200                              │ │
│   │     - http.url: http://localhost:3000/                   │ │
│   └──────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────┘
```

**Key fields:**
- `traceId`: Groups all spans from one request
- `parentId`: Links child spans to parents (undefined = root)
- `duration`: How long this operation took
- `attributes`: Context about the operation

## What You Learned

1. OTel auto-instruments HTTP without changing your app code
2. Traces are made of spans (one request = one trace, possibly many spans)
3. Spans have attributes that describe what happened
4. The `--import` flag is essential for instrumentation to work

## Troubleshooting

**No output when I make a request?**
- Check you used `--import ./instrumentation.js` (not `--require`)
- Check the file path is correct
- Add `console.log` to instrumentation.js to verify it loaded

**Getting errors about imports?**
- Use `require()` syntax as shown (CommonJS)
- Or see the ESM version below

## ESM Version

If using ES modules (`"type": "module"` in package.json):

```javascript
// instrumentation.mjs
import { NodeSDK } from '@opentelemetry/sdk-node';
import { ConsoleSpanExporter } from '@opentelemetry/sdk-trace-node';
import { HttpInstrumentation } from '@opentelemetry/instrumentation-http';

const sdk = new NodeSDK({
  serviceName: 'my-first-service',
  traceExporter: new ConsoleSpanExporter(),
  instrumentations: [new HttpInstrumentation()]
});

sdk.start();
```

```bash
node --import ./instrumentation.mjs app.mjs
```

## Next Steps

Now you can:
1. [See traces in Dash0](./02-send-to-dash0.md) - Beautiful trace visualization!
2. [Add custom spans](./03-add-custom-spans.md) - Track your business logic

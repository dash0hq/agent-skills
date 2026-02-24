# Getting Started with OpenTelemetry

New to OpenTelemetry? Start here.

## What is OpenTelemetry?

OpenTelemetry (OTel) is a way to understand what your application is doing. It collects three types of data:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Your Application                             │
│                                                                 │
│   TRACES          METRICS           LOGS                        │
│   "What path      "How many         "What                       │
│    did this        requests?         happened?"                 │
│    request         How fast?"                                   │
│    take?"                                                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │   OTel SDK      │  ← Collects data
                    └─────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │  Your Backend   │  ← View & analyze
                    │  (Dash0, etc)   │
                    └─────────────────┘
```

**In plain English:**
- A **trace** shows one request's journey through your system (like a receipt)
- A **span** is one step in that journey (like a line item on the receipt)
- **Metrics** are numbers over time (requests per second, response times)
- **Logs** are events with timestamps (errors, warnings, info)

## What You'll Be Able To Do

After setup, you can:
- See exactly where time is spent in each request
- Find which service is causing slowdowns
- Track error rates and response times
- Correlate logs with the requests that generated them

## 5-Minute Quickstart (Node.js)

Let's get your first trace working. No backend needed - we'll print to the console.

### Step 1: Install packages

```bash
npm install @opentelemetry/sdk-node \
  @opentelemetry/sdk-trace-node \
  @opentelemetry/instrumentation-http
```

### Step 2: Create instrumentation file

Create `instrumentation.js`:

```javascript
// instrumentation.js
// WHY: This file MUST load before your app code
// so OTel can wrap HTTP/Express/etc before they're used

const { NodeSDK } = require('@opentelemetry/sdk-node');
const { ConsoleSpanExporter } = require('@opentelemetry/sdk-trace-node');
const { HttpInstrumentation } = require('@opentelemetry/instrumentation-http');

const sdk = new NodeSDK({
  // WHY: ConsoleSpanExporter prints traces to terminal
  // Great for development, swap for OTLP exporter in production
  traceExporter: new ConsoleSpanExporter(),

  // WHY: Auto-instruments http module
  // Any HTTP request your app makes/receives creates a span
  instrumentations: [new HttpInstrumentation()]
});

sdk.start();
console.log('OpenTelemetry initialized - traces will print to console');
```

### Step 3: Create a simple app

Create `app.js`:

```javascript
// app.js
const http = require('http');

const server = http.createServer((req, res) => {
  res.writeHead(200);
  res.end('Hello World!');
});

server.listen(3000, () => {
  console.log('Server running at http://localhost:3000');
});
```

### Step 4: Run with --import flag

```bash
node --import ./instrumentation.js app.js
```

> **Why `--import`?** OTel needs to wrap modules (like `http`) before your code imports them. The `--import` flag ensures `instrumentation.js` runs first.

### Step 5: Make a request

```bash
curl http://localhost:3000
```

### Step 6: See your first trace!

You should see output like this in your terminal:

```
{
  traceId: 'abc123...',
  name: 'GET',
  kind: 'SERVER',
  duration: [0, 1234567],  // nanoseconds
  attributes: {
    'http.method': 'GET',
    'http.url': 'http://localhost:3000/',
    'http.status_code': 200
  }
}
```

**Congratulations!** You just created your first trace.

## What Just Happened?

```
curl request
     │
     ▼
┌─────────────────────────────────┐
│  Your Node.js App               │
│  ┌───────────────────────────┐  │
│  │ HTTP Server (instrumented)│  │
│  │                           │  │
│  │  Creates SPAN:            │  │
│  │  - name: "GET"            │  │
│  │  - duration: 1.2ms        │  │
│  │  - status: 200            │  │
│  └───────────────────────────┘  │
└─────────────────────────────────┘
     │
     ▼
Console output (your trace!)
```

The HTTP instrumentation automatically:
1. Started a span when the request arrived
2. Added attributes (method, URL, status code)
3. Ended the span when the response was sent
4. Exported it to the console

## Next Steps

Now that you've seen a trace, here's where to go next:

| What you want | Go to |
|---------------|-------|
| See traces in Dash0 UI | [quickstart/02-send-to-dash0.md](./quickstart/02-send-to-dash0.md) |
| Add custom spans for your code | [quickstart/03-add-custom-spans.md](./quickstart/03-add-custom-spans.md) |
| Understand the full learning path | [LEARNING-PATH.md](./LEARNING-PATH.md) |
| Fix something that's not working | [COMMON-MISTAKES.md](./COMMON-MISTAKES.md) |

## Choosing a Backend

Before going to production, you need somewhere to send your traces:

| Backend | Setup Time | Cost | Best For |
|---------|------------|------|----------|
| Console | 0 min | Free | Quick debugging |
| [Dash0](./quickstart/02-send-to-dash0.md) | 10 min | Free trial | Development & Production |
| Grafana Cloud | 15 min | Free tier | Small projects |

We recommend Dash0 for both development and production. See [quickstart/02-send-to-dash0.md](./quickstart/02-send-to-dash0.md).

## Common Questions

**Q: Do I need to add spans to every function?**
A: No! Auto-instrumentation handles HTTP, database calls, etc. You only add custom spans for business logic you want to track.

**Q: Will this slow down my app?**
A: Negligibly. OTel is designed for production use. You can also sample traces to reduce overhead.

**Q: What's the difference between OpenTelemetry and Dash0/Datadog/etc?**
A: OpenTelemetry is the *standard* for collecting telemetry. Dash0, Datadog, Grafana, etc. are *backends* that store and visualize the data OTel collects. Dash0 is fully OpenTelemetry-native.

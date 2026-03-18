---
title: "Node.js Instrumentation"
impact: HIGH
tags:
  - nodejs
  - backend
  - server
---

# Node.js Instrumentation

Instrument Node.js applications to generate traces, logs, and metrics for deep insights into behavior and performance.

## Use cases

- **HTTP Request Monitoring**: Understand outgoing and incoming HTTP requests through traces and metrics, with drill-downs to database level
- **Database Performance**: Observe which database statements execute and measure their duration for optimization
- **Error Detection**: Reveal uncaught errors and the context in which they happened

## Installation

```bash
npm install @opentelemetry/auto-instrumentations-node
```

**Note**: Installing the package alone is insufficient—you must activate the SDK AND enable exporters.

## Environment variables

All environment variables that control the SDK behavior:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_SERVICE_NAME` | Yes | `unknown_service` | Identifies your service in telemetry data |
| `OTEL_TRACES_EXPORTER` | Yes | `none` | **Must set to `otlp`** to export traces |
| `OTEL_METRICS_EXPORTER` | No | `none` | Set to `otlp` to export metrics |
| `OTEL_LOGS_EXPORTER` | No | `none` | Set to `otlp` to export logs |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Yes | `http://localhost:4317` | OTLP collector endpoint |
| `OTEL_EXPORTER_OTLP_HEADERS` | No | - | Headers for authentication (e.g., `Authorization=Bearer TOKEN`) |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | No | `http/protobuf` when using `auto-instrumentations-node`; `grpc` otherwise | Protocol: `grpc`, `http/protobuf`, or `http/json` |
| `OTEL_RESOURCE_ATTRIBUTES` | No | - | Additional resource attributes (e.g., `deployment.environment=production`) |

**Critical**: Without `OTEL_TRACES_EXPORTER=otlp`, the SDK defaults to `none` and no telemetry is exported.

**Protocol mismatch pitfall.**
`@opentelemetry/auto-instrumentations-node` defaults to `http/protobuf`, not `grpc`.
When targeting a Collector gRPC receiver on port 4317, always set `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` explicitly.
Omitting it causes a parse error on the Collector side (`Parse Error: Expected HTTP/`) and silent span loss on the SDK side.

### Where to get configuration values

1. **OTLP Endpoint**: Your observability platform's OTLP endpoint
   - In Dash0: [Settings → Organization → Endpoints](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Cb29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
   - Format: `https://<region>.your-platform.com`
2. **Auth Token**: API token for telemetry ingestion
   - In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)
3. **Service Name**: Choose a descriptive name (e.g., `order-api`, `checkout-service`)

## Configuration

### 1. Activate the SDK

The SDK must be loaded before your application code. The method depends on your module system:

**ESM Projects** (package.json has `"type": "module"` or using `.mjs` files):
```bash
export NODE_OPTIONS="--import @opentelemetry/auto-instrumentations-node/register"
```

**CommonJS Projects** (default, or using `.cjs` files):
```bash
export NODE_OPTIONS="--require @opentelemetry/auto-instrumentations-node/register"
```

**Note**: Tools like npm, pnpm, and yarn are Node.js applications, so you may observe instrumentation data from package managers when running commands.

### 2. Set service name

```bash
export OTEL_SERVICE_NAME="my-service"
```

### 3. Enable exporters

**This step is required** - without it, no telemetry is sent:

```bash
# Required for traces
export OTEL_TRACES_EXPORTER="otlp"

# Optional: also export metrics and logs
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"
```

### 4. Configure endpoint

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"
```

### 5. Optional: target specific dataset

```bash
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN,Dash0-Dataset=my-dataset"
```

## Complete setup

### Using environment variables

```bash
# Service identification
export OTEL_SERVICE_NAME="my-service"

# Enable exporters (required!)
export OTEL_TRACES_EXPORTER="otlp"
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"

# Configure endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"

# Activate SDK (use --import for ESM, --require for CommonJS)
export NODE_OPTIONS="--import @opentelemetry/auto-instrumentations-node/register"

node app.js
```

### Using .env.local file

Node.js does not automatically load `.env` files. Use the `--env-file` flag (Node.js 20.6+):

**.env.local:**
```bash
OTEL_SERVICE_NAME=my-service
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=https://<OTLP_ENDPOINT>
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer YOUR_AUTH_TOKEN
NODE_OPTIONS=--import @opentelemetry/auto-instrumentations-node/register
```

**Run with:**
```bash
node --env-file=.env.local app.js
```

**Note**: The `--env-file` flag requires Node.js 20.6 or later.

### Using package.json scripts

Add instrumented scripts to your `package.json`:

```json
{
  "scripts": {
    "start": "node app.js",
    "start:otel": "node --env-file=.env.local app.js",
    "start:otel:console": "OTEL_SERVICE_NAME=my-service OTEL_TRACES_EXPORTER=console node --import @opentelemetry/auto-instrumentations-node/register app.js",
    "dev": "node --env-file=.env.local --watch app.js"
  }
}
```

**.env.local** (create this file):
```bash
OTEL_SERVICE_NAME=my-service
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=https://<OTLP_ENDPOINT>
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer YOUR_AUTH_TOKEN
NODE_OPTIONS=--import @opentelemetry/auto-instrumentations-node/register
```

**Usage:**
```bash
npm run start:otel          # Run with OTLP export to backend
npm run start:otel:console  # Run with console output (no collector needed)
npm run dev                 # Development with watch mode + telemetry
```

## Local development

### Console exporter

For development without a collector, use the console exporter to see telemetry in your terminal:

```bash
export OTEL_SERVICE_NAME="my-service"
export OTEL_TRACES_EXPORTER="console"
export OTEL_METRICS_EXPORTER="console"
export OTEL_LOGS_EXPORTER="console"
export NODE_OPTIONS="--import @opentelemetry/auto-instrumentations-node/register"

node app.js
```

This prints spans, metrics, and logs directly to stdout—useful for verifying instrumentation works before configuring a remote backend.

### Without a collector

If you set `OTEL_TRACES_EXPORTER=otlp` but have no collector running, you'll see connection errors. This is expected behavior:

```
Error: 14 UNAVAILABLE: No connection established. Last error: connect ECONNREFUSED 127.0.0.1:4317
```

**Options:**
1. Use `console` exporter during development (recommended for quick testing)
2. Run a local OpenTelemetry Collector
3. Point directly to your observability backend

## Resource configuration

Set `service.name`, `service.version`, and `deployment.environment.name` for every deployment.
See [resource attributes](../resources.md) for the full list of required and recommended attributes.

## Kubernetes setup

See [Kubernetes deployment](../platforms/k8s.md) for pod metadata injection, resource attributes, and Dash0 Kubernetes Operator guidance.

## Supported libraries

The auto-instrumentation package automatically instruments:

| Category | Libraries |
|----------|-----------|
| HTTP | http, https, express, fastify, koa, hapi |
| Database | pg, mysql, mysql2, mongodb, redis, ioredis |
| ORM | knex, sequelize, typeorm, prisma |
| Messaging | amqplib, kafkajs |
| AWS | aws-sdk, @aws-sdk/* |
| Logging | pino, winston, bunyan |
| GraphQL | graphql |
| gRPC | @grpc/grpc-js |

Refer to [OpenTelemetry documentation](https://opentelemetry.io/ecosystem/registry/?language=js) for the complete list.

## Custom spans

Add business context to auto-instrumented traces:

```javascript
import { trace, SpanStatusCode } from "@opentelemetry/api";

const tracer = trace.getTracer("my-service");

async function processOrder(order) {
  return tracer.startActiveSpan("order.process", async (span) => {
    try {
      span.setAttribute("order.id", order.id);
      span.setAttribute("order.total", order.total);
      const result = await saveOrder(order);
      return result;
    } catch (error) {
      span.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
      const ctx = span.spanContext();
      logger.error({
        'trace_id': ctx.traceId,
        'span_id': ctx.spanId,
        'exception.type': error.name,
        'exception.message': error.message,
        'exception.stacktrace': error.stack,
      }, 'order.process.failed');
      throw error;
    } finally {
      span.end();
    }
  });
}
```

### Retrieving the active span

Auto-instrumentation creates spans you do not control directly (e.g., the `SERVER` span for an HTTP request).
To enrich these spans with business context or set their status, retrieve the active span from the current context.
See [adding attributes to auto-instrumented spans](../spans.md#adding-attributes-to-auto-instrumented-spans) for when to use this pattern.

```javascript
import { trace } from "@opentelemetry/api";

app.post("/api/orders", async (req, res) => {
  const span = trace.getActiveSpan();
  span?.setAttribute("order.id", req.body.orderId);
  span?.setAttribute("tenant.id", req.headers["x-tenant-id"]);
  // ... handler logic
});
```

`trace.getActiveSpan()` returns `undefined` if no span is active (e.g., when instrumentation is disabled).
Always use optional chaining (`?.`) when calling methods on the result.

### Span status rules

See [span status code](../spans.md#span-status-code) for the full rules.
This section shows how to apply them in Node.js.

#### Always include a status message with `ERROR`

The `message` field on the status object must contain the error class and a short explanation — enough to understand the failure without opening the full trace.

```javascript
// BAD: no status message
span.setStatus({ code: SpanStatusCode.ERROR });

// BAD: generic message with no diagnostic value
span.setStatus({ code: SpanStatusCode.ERROR, message: 'something went wrong' });

// GOOD: specific message with error class and context
span.setStatus({
  code: SpanStatusCode.ERROR,
  message: `TimeoutError: upstream payment service did not respond within 5s`,
});
```

Do not include stack traces in the status message.
Record those in a log record with `exception.stacktrace` instead.

```javascript
// BAD: stack trace in the status message
span.setStatus({ code: SpanStatusCode.ERROR, message: error.stack });

// GOOD: short message only
span.setStatus({ code: SpanStatusCode.ERROR, message: error.message });
```

#### Use `OK` only for confirmed success

Set status to `OK` when application logic has explicitly verified the operation succeeded.
Leave status `UNSET` if the code simply did not encounter an error.

```javascript
// GOOD: explicit confirmation from downstream
const response = await fetch(url);
if (response.ok) {
  span.setStatus({ code: SpanStatusCode.OK });
}

// BAD: setting OK speculatively
span.setStatus({ code: SpanStatusCode.OK });
return await someFunction(); // might still fail after this point
```

## Structured logging

Configure your logging framework to serialize exceptions into a single structured field so that stack traces do not break the one-line-per-record contract.
See [logs](../logs.md) for general guidance on structured logging and exception stack traces.

### pino

pino serializes errors into structured JSON by default when passed as the first argument.
The `err` serializer extracts `message`, `type`, and `stack` as separate fields, keeping each log record on a single line.

```javascript
import pino from 'pino';

const logger = pino();

try {
  processOrder(order);
} catch (err) {
  logger.error({ err, order_id: order.id }, 'order.failed');
}
```

Pass the error as `{ err }` in the first argument, not as the message string.
If you log `error.stack` directly as the message, pino prints it as multi-line text.

### winston

winston does not serialize errors by default.
Enable the `errors` format with `{ stack: true }` to capture the stack trace as a structured field.

```javascript
import winston from 'winston';

const logger = winston.createLogger({
  format: winston.format.combine(
    winston.format.errors({ stack: true }),
    winston.format.json(),
  ),
  transports: [new winston.transports.Console()],
});

try {
  processOrder(order);
} catch (err) {
  logger.error('order.failed', { error: err, order_id: order.id });
}
```

Without `winston.format.errors({ stack: true })`, the stack trace is silently dropped from JSON output.

## Graceful shutdown

The Node.js auto-instrumentation registers shutdown hooks for `SIGTERM` and `SIGINT` automatically.
No additional code is needed for normal process termination.

However, unhandled exceptions and unhandled promise rejections cause immediate process exit before the SDK flushes its buffers.
Register handlers that flush the tracer provider before exiting so that spans from the failing request are not lost.

```javascript
import { trace } from "@opentelemetry/api";

function forceFlushAll() {
  const promises = [];
  let tp = trace.getTracerProvider();
  // The auto-instrumentation wraps the real provider in a ProxyTracerProvider
  // that does not expose forceFlush(). Unwrap it to reach the SDK provider.
  if (typeof tp.forceFlush !== "function" && typeof tp.getDelegate === "function") {
    tp = tp.getDelegate();
  }
  if (typeof tp.forceFlush === "function") promises.push(tp.forceFlush());
  return Promise.allSettled(promises);
}

process.on("uncaughtException", (error) => {
  logger.error({
    'exception.type': error.name,
    'exception.message': error.message,
    'exception.stacktrace': error.stack,
  }, "uncaught.exception");
  forceFlushAll().finally(() => process.exit(1));
});

process.on("unhandledRejection", (reason) => {
  const error = reason instanceof Error ? reason : new Error(String(reason));
  logger.error({
    'exception.type': error.name,
    'exception.message': error.message,
    'exception.stacktrace': error.stack,
  }, "unhandled.rejection");
  forceFlushAll().finally(() => process.exit(1));
});
```

`forceFlush()` on the tracer provider only flushes span processors — it does not flush the logger or meter providers.
In the auto-instrumented setup, the `logger` reference here is a pino/winston logger writing to stdout (see [structured logging](#structured-logging)), so the log record reaches the Collector through stdout capture, not through the OTel log provider.
If you use the OTel Logs SDK directly, add its provider to `forceFlushAll()`.

`trace.getTracerProvider()` returns a `ProxyTracerProvider` that does not expose `forceFlush()`.
Call `getDelegate()` to unwrap it and reach the SDK-level provider (`NodeTracerProvider`) where `forceFlush()` is defined.
The call returns a promise; `finally` ensures the process exits even if the flush fails or times out.

## Troubleshooting

### No telemetry appearing

**Check exporters are enabled:**
```bash
echo $OTEL_TRACES_EXPORTER  # Should be "otlp" or "console", not empty
```

The SDK defaults `OTEL_TRACES_EXPORTER` to `none`, which silently discards all telemetry.

**Verify SDK is loaded:**
```bash
echo $NODE_OPTIONS  # Should contain --import or --require
```

### ECONNREFUSED errors

```
Error: 14 UNAVAILABLE: connect ECONNREFUSED 127.0.0.1:4317
```

This means the SDK is working but cannot reach the collector:
- **No collector running**: Start a local collector or use `OTEL_TRACES_EXPORTER=console`
- **Wrong endpoint**: Check `OTEL_EXPORTER_OTLP_ENDPOINT` is correct
- **Port mismatch**: gRPC uses 4317, HTTP uses 4318

### Environment variables not loading

If using `.env.local`:
- Ensure you're using `--env-file=.env.local` flag
- Requires Node.js 20.6+
- Check file path is correct relative to where you run the command

### ESM/CommonJS mismatch

**Symptom**: SDK loads but no instrumentation happens

**Fix**: Match the flag to your module system:
- ESM (`"type": "module"` in package.json): Use `--import`
- CommonJS (default): Use `--require`

### "Exporter is empty" or similar warnings

Usually means `OTEL_TRACES_EXPORTER` (or metrics/logs) is not set. Set it explicitly:
```bash
export OTEL_TRACES_EXPORTER="otlp"
```

## Resources

- [OpenTelemetry Node.js Documentation](https://opentelemetry.io/docs/languages/js/getting-started/nodejs/)
- [Auto-Instrumentation Package](https://www.npmjs.com/package/@opentelemetry/auto-instrumentations-node)
- [Environment Variable Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)

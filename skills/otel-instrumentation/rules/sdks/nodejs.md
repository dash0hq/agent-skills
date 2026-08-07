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

### Verifying dependencies

Never write a package name or version into `package.json` from memory; verify it against the npm registry first, per [verify-dependencies](../verify-dependencies.md):

```bash
npm view @opentelemetry/instrumentation-undici version         # latest published version
npm view @opentelemetry/instrumentation-undici versions --json # every published version
npm view @opentelemetry/instrumentation-undici deprecated      # prints the notice when deprecated, nothing otherwise
```

A range that matches no published version prints nothing: `npm view '<pkg>@^1.2.3' version`.
Prefer `npm install <pkg>` (no version) over hand-editing `package.json`, so npm resolves and records the real latest version.
Check the package's runtime requirement against the project's Node.js version (`engines` in `package.json`, base images in Dockerfiles, `.nvmrc`) with `npm view <pkg> engines` — npm only warns on an engines mismatch by default, so an incompatible package installs fine and breaks at runtime.

`package-lock.json` must move with `package.json`: a hand-edited manifest leaves the lockfile stale, and `npm ci` fails on the mismatch by design:

```text
npm error code EUSAGE
npm error `npm ci` can only install packages when your package.json and package-lock.json or npm-shrinkwrap.json are in sync. Please update your lock file with `npm install` before continuing.
npm error Missing: @opentelemetry/instrumentation-undici@0.15.0 from lock file
```

Regenerate the lockfile where npm runs (`npm install` rewrites it) per [verify-dependencies](../verify-dependencies.md#keeping-the-lockfile-in-step).
Only as a fallback, when no environment with npm exists outside the image build, have the builder stage reconcile by running `npm install` instead of `npm ci` — this trades away the exact-lockfile reproducibility `npm ci` guarantees, so prefer regenerating the lockfile and keeping `npm ci`.

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
| `OTEL_RESOURCE_ATTRIBUTES` | No | - | Additional resource attributes (e.g., `deployment.environment.name=production`) |

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

#### Dockerfile activation

When activating the SDK in a Dockerfile, set `NODE_OPTIONS` with the `ENV` instruction in the quoted `name=value` form (use `--import` instead of `--require` for ESM projects, per the module system rules below):

```dockerfile
ENV NODE_OPTIONS="--require @opentelemetry/auto-instrumentations-node/register"
```

Never write the legacy space-separated `ENV` form.
The classic (non-BuildKit) Docker builder — still what `docker build` through the Engine API uses, for example on stock GitHub Actions runners — rejects it at parse time whenever the value contains spaces:

```dockerfile
# BAD: legacy space-separated ENV form — the classic Docker builder fails the build with
# "Syntax error - can't find = in ... Must be of the form: name=value"
ENV NODE_OPTIONS --require @opentelemetry/auto-instrumentations-node/register
```

Place the `ENV NODE_OPTIONS=…` instruction **after** the `RUN` step that installs dependencies.
`ENV` applies to every subsequent build step, and npm, pnpm, and yarn are themselves Node.js processes — so a `NODE_OPTIONS` that preloads a package which is not installed yet makes the install step itself crash:

```dockerfile
# BAD: NODE_OPTIONS is set before the dependencies exist, so the npm process
# crashes with "Error: Cannot find module '@opentelemetry/auto-instrumentations-node/register'"
ENV NODE_OPTIONS="--require @opentelemetry/auto-instrumentations-node/register"
COPY package*.json ./
RUN npm install --omit=dev
```

```dockerfile
# GOOD: dependencies are installed first; NODE_OPTIONS only affects later steps and the container runtime
COPY package*.json ./
RUN npm install --omit=dev
ENV NODE_OPTIONS="--require @opentelemetry/auto-instrumentations-node/register"
```

#### Module system in code snippets

Before writing or copying any Node.js snippet from this file into an application, determine the target file's module system and translate the snippet if needed.
Mixing `import` and `require` in the same file causes `SyntaxError: Cannot use import statement outside a module` (ESM syntax in a CJS file) or `ReferenceError: require is not defined in ES module scope` (CJS syntax in an ESM file).

**Detection rules**, in order:

1. If the file extension is `.mjs`, treat as ESM.
   If `.cjs`, treat as CommonJS.
2. Otherwise, read the nearest `package.json` walking up from the file.
   `"type": "module"` means ESM; `"type": "commonjs"` or a missing `"type"` field means CommonJS.
3. If the surrounding file already uses `import ... from` or `export`, match ESM.
   If it uses `require(...)` or `module.exports`, match CommonJS.
4. For a new file with no other signals, default to CommonJS unless the project's `package.json` sets `"type": "module"`.

**Snippet default**: every JavaScript snippet in this file below uses ESM `import` syntax.
If the target file is CommonJS, translate before pasting.

**Translation table**:

| ESM `import` | CommonJS `require` |
|--------------|--------------------|
| `import { trace, SpanStatusCode } from '@opentelemetry/api';` | `const { trace, SpanStatusCode } = require('@opentelemetry/api');` |
| `import pino from 'pino';` | `const pino = require('pino');` |
| `import * as api from '@opentelemetry/api';` | `const api = require('@opentelemetry/api');` |
| `import { X, Y as Z } from 'pkg';` | `const { X, Y: Z } = require('pkg');` |
| `export function foo() {}` | `module.exports.foo = function foo() {};` |
| `export default foo;` | `module.exports = foo;` |

For default imports from CommonJS packages (e.g., `import pino from 'pino'`), the runtime resolves this to the package's `module.exports`.
Named imports from CommonJS packages work when the package publishes an ESM entry point; if not, use the namespace form (`import * as pkg from 'pkg'`) and then destructure.

TypeScript source files always use `import` regardless of the compiled output — the emit target (`"module"` in `tsconfig.json`) determines the runtime module system, so no translation is needed at the source level.

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

<!-- eval:skip -->
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

## Database query parameters

The `@opentelemetry/instrumentation-pg`, `@opentelemetry/instrumentation-mysql2`, `@opentelemetry/instrumentation-mongodb`, and related packages do **not** capture prepared-statement parameter values out of the box and expose no env var.
Read [capturing database query parameters](../capture-database-query-parameters.md) first — it covers the cross-language risks and the Collector-side defence-in-depth that must be in place before enabling capture.

Use the `requestHook` (or `responseHook`) configuration point to emit `db.query.parameter.<key>` from the parameters the instrumentation already has in hand.

### `pg` (PostgreSQL)

```javascript
import { PgInstrumentation } from '@opentelemetry/instrumentation-pg';

new PgInstrumentation({
  requestHook: (span, requestInfo) => {
    const params = requestInfo.params;
    if (!Array.isArray(params)) return;
    for (let i = 0; i < params.length; i++) {
      const value = params[i];
      if (value == null) continue;
      const type = typeof value;
      if (type !== 'string' && type !== 'number' && type !== 'boolean' && !(value instanceof Date)) {
        continue;
      }
      span.setAttribute(`db.query.parameter.${i}`, String(value));
    }
  },
});
```

The same pattern applies to `@opentelemetry/instrumentation-mysql2` (`responseHook` receives the `values` array) and other DB instrumentations that surface the parameter list via a hook.

If the instrumentation library does not expose a hook, fall back to a custom `SpanProcessor` that reads `db.query.text` and reconstructs parameters from the call site (only viable when parameters are also stashed on the span at the call site).

The whitelist in the snippet above mirrors the Java type set — extend it only after confirming the additional types are safe for the dataset.

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

<!-- eval:skip -->
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

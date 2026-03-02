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

## Use Cases

- **HTTP Request Monitoring**: Understand outgoing and incoming HTTP requests through traces and metrics, with drill-downs to database level
- **Database Performance**: Observe which database statements execute and measure their duration for optimization
- **Error Detection**: Reveal uncaught errors and the context in which they happened

---

## Installation

```bash
npm install @opentelemetry/auto-instrumentations-node
```

**Note**: Installing the package alone is insufficient—you must activate the SDK AND enable exporters.

---

## Environment Variables

All environment variables that control the SDK behavior:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_SERVICE_NAME` | Yes | `unknown_service` | Identifies your service in telemetry data |
| `OTEL_TRACES_EXPORTER` | Yes | `none` | **Must set to `otlp`** to export traces |
| `OTEL_METRICS_EXPORTER` | No | `none` | Set to `otlp` to export metrics |
| `OTEL_LOGS_EXPORTER` | No | `none` | Set to `otlp` to export logs |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Yes | `http://localhost:4317` | OTLP collector endpoint |
| `OTEL_EXPORTER_OTLP_HEADERS` | No | - | Headers for authentication (e.g., `Authorization=Bearer TOKEN`) |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | No | `grpc` | Protocol: `grpc`, `http/protobuf`, or `http/json` |
| `OTEL_RESOURCE_ATTRIBUTES` | No | - | Additional resource attributes (e.g., `deployment.environment=production`) |

**Critical**: Without `OTEL_TRACES_EXPORTER=otlp`, the SDK defaults to `none` and no telemetry is exported.

### Where to Get Configuration Values

1. **OTLP Endpoint**: Your observability platform's OTLP endpoint
   - In Dash0: [Settings → Organization → Endpoints](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Cb29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
   - Format: `https://<region>.your-platform.com`
2. **Auth Token**: API token for telemetry ingestion
   - In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)
3. **Service Name**: Choose a descriptive name (e.g., `order-api`, `checkout-service`)

---

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

### 2. Set Service Name

```bash
export OTEL_SERVICE_NAME="my-service"
```

### 3. Enable Exporters

**This step is required** - without it, no telemetry is sent:

```bash
# Required for traces
export OTEL_TRACES_EXPORTER="otlp"

# Optional: also export metrics and logs
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"
```

### 4. Configure Endpoint

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"
```

### 5. Optional: Target Specific Dataset

```bash
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN,Dash0-Dataset=my-dataset"
```

---

## Complete Setup

### Using Environment Variables

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

### Using .env.local File

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

### Using package.json Scripts

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

---

## Local Development

### Console Exporter

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

### Without a Collector

If you set `OTEL_TRACES_EXPORTER=otlp` but have no collector running, you'll see connection errors. This is expected behavior:

```
Error: 14 UNAVAILABLE: No connection established. Last error: connect ECONNREFUSED 127.0.0.1:4317
```

**Options:**
1. Use `console` exporter during development (recommended for quick testing)
2. Run a local OpenTelemetry Collector
3. Point directly to your observability backend

---

## Kubernetes Setup

When not using the Dash0 Kubernetes Operator, extend resource attributes with Pod information for proper log, trace, and metrics correlation:

```bash
export OTEL_RESOURCE_ATTRIBUTES="k8s.pod.name=$(hostname),k8s.pod.uid=$(POD_UID)"
```

**Recommended**: Use the [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator) for automatic instrumentation of Node.js workloads.

---

## Supported Libraries

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

---

## Custom Spans

Add business context to auto-instrumented traces:

```javascript
import { trace } from "@opentelemetry/api";

const tracer = trace.getTracer("my-service");

async function processOrder(order) {
  return tracer.startActiveSpan("order.process", async (span) => {
    try {
      span.setAttribute("order.id", order.id);
      span.setAttribute("order.total", order.total);
      const result = await saveOrder(order);
      return result;
    } catch (error) {
      span.recordException(error);
      throw error;
    } finally {
      span.end();
    }
  });
}
```

---

## Troubleshooting

### No Telemetry Appearing

**Check exporters are enabled:**
```bash
echo $OTEL_TRACES_EXPORTER  # Should be "otlp" or "console", not empty
```

The SDK defaults `OTEL_TRACES_EXPORTER` to `none`, which silently discards all telemetry.

**Verify SDK is loaded:**
```bash
echo $NODE_OPTIONS  # Should contain --import or --require
```

### ECONNREFUSED Errors

```
Error: 14 UNAVAILABLE: connect ECONNREFUSED 127.0.0.1:4317
```

This means the SDK is working but cannot reach the collector:
- **No collector running**: Start a local collector or use `OTEL_TRACES_EXPORTER=console`
- **Wrong endpoint**: Check `OTEL_EXPORTER_OTLP_ENDPOINT` is correct
- **Port mismatch**: gRPC uses 4317, HTTP uses 4318

### Environment Variables Not Loading

If using `.env.local`:
- Ensure you're using `--env-file=.env.local` flag
- Requires Node.js 20.6+
- Check file path is correct relative to where you run the command

### ESM/CommonJS Mismatch

**Symptom**: SDK loads but no instrumentation happens

**Fix**: Match the flag to your module system:
- ESM (`"type": "module"` in package.json): Use `--import`
- CommonJS (default): Use `--require`

### "Exporter is Empty" or Similar Warnings

Usually means `OTEL_TRACES_EXPORTER` (or metrics/logs) is not set. Set it explicitly:
```bash
export OTEL_TRACES_EXPORTER="otlp"
```

---

## Resources

- [OpenTelemetry Node.js Documentation](https://opentelemetry.io/docs/languages/js/getting-started/nodejs/)
- [Auto-Instrumentation Package](https://www.npmjs.com/package/@opentelemetry/auto-instrumentations-node)
- [Environment Variable Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)

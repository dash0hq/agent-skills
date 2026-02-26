---
title: "Node.js SDK Setup"
impact: HIGH
tags:
  - nodejs
  - sdk
  - setup
  - configuration
  - typescript
---

# Node.js SDK Setup Reference

## Quick Start (Environment Variables)

The simplest way to instrument a Node.js application:

```bash
npm install @opentelemetry/auto-instrumentations-node
```

```bash
export OTEL_SERVICE_NAME="my-service"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer ${DASH0_AUTH_TOKEN}"
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.0.0,deployment.environment.name=production"
export NODE_OPTIONS="--import @opentelemetry/auto-instrumentations-node/register"

node app.js
```

That's it. Auto-instrumentation handles HTTP, databases, and most frameworks automatically.

---

## Production Environment Variables

| Variable | Priority | Example | Purpose |
|----------|----------|---------|---------|
| `OTEL_SERVICE_NAME` | MUST | `payment-service` | Identifies your service |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | MUST | `https://ingress.eu-west-1.dash0.com:4317` | Where to send telemetry |
| `OTEL_EXPORTER_OTLP_HEADERS` | MUST | `Authorization=Bearer token` | Authentication |
| `OTEL_RESOURCE_ATTRIBUTES` | SHOULD | `service.version=1.0.0,deployment.environment.name=prod` | Resource context |
| `OTEL_TRACES_SAMPLER` | SHOULD | `parentbased_traceidratio` | Sampling strategy |
| `OTEL_TRACES_SAMPLER_ARG` | SHOULD | `0.1` | Sample 10% of traces |
| `OTEL_LOG_LEVEL` | MAY | `info` | SDK logging verbosity |
| `OTEL_PROPAGATORS` | MAY | `tracecontext,baggage` | Context propagation |

---

## Startup Verification

When correctly configured, you should see:

```
@opentelemetry/instrumentation-http Applying instrumentation patch
@opentelemetry/instrumentation-express Applying instrumentation patch
...
```

Enable debug logging to verify:

```bash
export OTEL_LOG_LEVEL=debug
```

### Quick Verification Test

```bash
# Start your app
node --import @opentelemetry/auto-instrumentations-node/register app.js

# In another terminal, hit an endpoint
curl http://localhost:3000/health

# Check your observability backend for the span
```

---

## Official Documentation

- [Node.js SDK Documentation](https://opentelemetry.io/docs/languages/js/getting-started/nodejs/)
- [Environment Variables](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)

---

## Package Installation

```bash
# Core (required)
npm install @opentelemetry/sdk-node @opentelemetry/api

# Auto-instrumentation (recommended)
npm install @opentelemetry/auto-instrumentations-node

# Exporters (choose based on protocol)
npm install @opentelemetry/exporter-trace-otlp-proto    # HTTP/protobuf
npm install @opentelemetry/exporter-metrics-otlp-proto
npm install @opentelemetry/exporter-logs-otlp-proto
```

---

## Custom instrumentation.ts (When Needed)

Use a custom setup when you need:
- Custom samplers
- Specific instrumentation configuration
- Multiple exporters
- Programmatic resource detection

```typescript
// instrumentation.ts
import { NodeSDK } from "@opentelemetry/sdk-node";
import { getNodeAutoInstrumentations } from "@opentelemetry/auto-instrumentations-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-proto";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-proto";
import { PeriodicExportingMetricReader } from "@opentelemetry/sdk-metrics";
import { Resource } from "@opentelemetry/resources";
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT_NAME,
  ATTR_SERVICE_NAMESPACE
} from "@opentelemetry/semantic-conventions";

const resource = new Resource({
  [ATTR_SERVICE_NAME]: process.env.OTEL_SERVICE_NAME || "my-service",
  [ATTR_SERVICE_VERSION]: process.env.npm_package_version || "0.0.0",
  [ATTR_DEPLOYMENT_ENVIRONMENT_NAME]: process.env.NODE_ENV || "development",
  [ATTR_SERVICE_NAMESPACE]: "my-team"
});

const sdk = new NodeSDK({
  resource,
  traceExporter: new OTLPTraceExporter(),
  metricReader: new PeriodicExportingMetricReader({
    exporter: new OTLPMetricExporter(),
    exportIntervalMillis: 60000
  }),
  instrumentations: [
    getNodeAutoInstrumentations({
      "@opentelemetry/instrumentation-fs": { enabled: false },
      "@opentelemetry/instrumentation-http": {
        ignoreIncomingPaths: ["/health", "/ready", "/metrics"]
      }
    })
  ]
});

sdk.start();

// Graceful shutdown
const shutdown = async () => {
  await sdk.shutdown();
  process.exit(0);
};

process.on("SIGTERM", shutdown);
process.on("SIGINT", shutdown);
```

### Running with Custom Setup

```bash
# TypeScript with tsx
node --import tsx --import ./instrumentation.ts app.ts

# Compiled JavaScript
node --import ./dist/instrumentation.js dist/app.js
```

### package.json Scripts

```json
{
  "scripts": {
    "start": "node --import ./dist/instrumentation.js dist/app.js",
    "dev": "tsx --import ./instrumentation.ts src/app.ts"
  }
}
```

---

## Docker Configuration

```dockerfile
FROM node:20-alpine

WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY dist ./dist

# Environment variables (set in deployment, not Dockerfile)
ENV NODE_OPTIONS="--import ./dist/instrumentation.js"

CMD ["node", "dist/app.js"]
```

```yaml
# docker-compose.yml or Kubernetes
environment:
  - OTEL_SERVICE_NAME=my-service
  - OTEL_EXPORTER_OTLP_ENDPOINT=https://ingress.eu-west-1.dash0.com:4317
  - OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer ${DASH0_AUTH_TOKEN}
  - OTEL_RESOURCE_ATTRIBUTES=deployment.environment.name=production
  - NODE_OPTIONS=--import ./dist/instrumentation.js
```

---

## Sampling Configuration

### Via Environment Variables (Recommended)

```bash
export OTEL_TRACES_SAMPLER=parentbased_traceidratio
export OTEL_TRACES_SAMPLER_ARG=0.1  # 10%
```

### Available Samplers

| Sampler | Use Case |
|---------|----------|
| `always_on` | Development, debugging |
| `always_off` | Disable tracing |
| `traceidratio` | Sample X% of all traces |
| `parentbased_always_on` | Follow parent, default on |
| `parentbased_traceidratio` | Follow parent, default X% (recommended for production) |

---

## Resource Attributes

### Essential Attributes

| Attribute | Required | Example |
|-----------|----------|---------|
| `service.name` | Yes | `payment-service` |
| `service.version` | Recommended | `2.1.0` |
| `deployment.environment.name` | Recommended | `production` |
| `service.namespace` | Optional | `checkout-team` |
| `service.instance.id` | Optional | `pod-abc-123` |

### Setting via Environment

```bash
export OTEL_SERVICE_NAME="payment-service"
export OTEL_RESOURCE_ATTRIBUTES="service.version=2.1.0,deployment.environment.name=production,service.namespace=checkout"
```

---

## Pre-Deployment Checklist

- [ ] `OTEL_SERVICE_NAME` set (not "unknown_service")
- [ ] `OTEL_EXPORTER_OTLP_ENDPOINT` correct for environment
- [ ] Auth token configured and valid
- [ ] `--import` flag added to `NODE_OPTIONS` or start command
- [ ] Graceful shutdown handler registered
- [ ] Sampling rate appropriate (1-10% prod, 100% dev)
- [ ] Health endpoints excluded from tracing
- [ ] Test span sent and verified in backend

---

## Troubleshooting

| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| No spans in backend | SDK not initialized | Verify `--import` flag in startup |
| "unknown_service" in traces | `OTEL_SERVICE_NAME` not set | Set the environment variable |
| Connection refused | Wrong endpoint | Check `OTEL_EXPORTER_OTLP_ENDPOINT` URL and port |
| 401/403 errors | Auth failure | Verify `OTEL_EXPORTER_OTLP_HEADERS` format |
| Missing HTTP spans | Instrumentation not loaded | Ensure `--import` runs before app code |
| Spans not flushed on exit | No shutdown handler | Add SIGTERM/SIGINT handlers |
| High memory usage | Too many spans | Enable sampling, check for span loops |

### Debug Commands

```bash
# Enable verbose SDK logging
export OTEL_LOG_LEVEL=debug

# Test endpoint connectivity
curl -v https://ingress.eu-west-1.dash0.com:4317

# Verify instrumentation loading
node --import @opentelemetry/auto-instrumentations-node/register -e "console.log('OK')"
```

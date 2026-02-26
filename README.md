# Skills

A collection of expert skills for implementing high-quality observability with OpenTelemetry.

## Available Skills

| Skill | Description |
|-------|-------------|
| [otel-instrumentation](./skills/otel-instrumentation/) | OpenTelemetry instrumentation for Node.js and browsers |

## Quick Start

**Node.js:**
```bash
npm install @opentelemetry/auto-instrumentations-node

export OTEL_SERVICE_NAME="my-service"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_TOKEN"
export NODE_OPTIONS="--require @opentelemetry/auto-instrumentations-node/register"

node app.js
```

**Browser:**
```bash
npm install @dash0/sdk-web
```

```javascript
import { init } from "@dash0/sdk-web";

init({
  serviceName: "my-frontend",
  endpoint: { url: "https://ingress.eu-west-1.dash0.com:4318", authToken: "YOUR_TOKEN" }
});
```

## What is OpenTelemetry?

OpenTelemetry (OTel) is the industry standard for collecting observability data:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Your Application                             │
│                                                                  │
│   TRACES              METRICS              LOGS                  │
│   "Request flow"      "How many/fast"      "What happened"       │
└──────────────────────────┬───────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────────┐
│   OTel SDK  ──▶  Collector  ──▶  Backend (Dash0, Grafana, etc.) │
└──────────────────────────────────────────────────────────────────┘
```

## Key Principles

- **Signal density over volume** - Every telemetry item should help detect, localize, or explain issues
- **Push reduction early** - SDK sampling → Collector filtering → Backend retention
- **SLO-aware policies** - Never sample data feeding your SLOs

## Official Documentation

- [OpenTelemetry Docs](https://opentelemetry.io/docs/)
- [Dash0 Integration Hub](https://www.dash0.com/hub/integrations)

## License

MIT

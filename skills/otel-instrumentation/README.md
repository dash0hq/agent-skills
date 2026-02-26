# otel-instrumentation

Expert guidance for implementing high-quality, cost-efficient OpenTelemetry telemetry across Node.js and browser applications.

## Rules

| Rule | Description |
|------|-------------|
| [telemetry](./rules/telemetry.md) | Spans, metrics, logs, cardinality, anti-patterns |
| [nodejs](./rules/nodejs.md) | Node.js instrumentation with auto-instrumentation |
| [browser](./rules/browser.md) | Browser instrumentation with Dash0 SDK or OTel |

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

## Key Principles

- **Signal density over volume** - Every telemetry item should help detect, localize, or explain issues
- **Push reduction early** - SDK sampling → Collector filtering → Backend retention
- **SLO-aware policies** - Never sample data feeding your SLOs

## Installation

```bash
npx skills add dash0/otel-instrumentation
```

## Official Documentation

- [OpenTelemetry Docs](https://opentelemetry.io/docs/)
- [Dash0 Integration Hub](https://www.dash0.com/hub/integrations)

## License

MIT

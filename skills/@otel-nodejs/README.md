# @otel-nodejs

OpenTelemetry implementation skill for Node.js and TypeScript applications.

## New Here?

If you've never used OpenTelemetry:
1. Start with [GETTING-STARTED.md](../GETTING-STARTED.md) (5 min)
2. Do [quickstart/01-first-trace.md](../quickstart/01-first-trace.md)
3. Then come back here for the full setup

## What This Skill Covers

| Topic | What You'll Learn |
|-------|-------------------|
| SDK setup | Initialization patterns, `--import` flag |
| Auto-instrumentation | What gets traced automatically |
| Manual instrumentation | Custom spans, metrics, logs |
| Framework integration | Express, Fastify, NestJS, Next.js |
| Logger integration | pino, winston with trace correlation |

## Reading Order

### Basic Setup (Week 1)

1. **SKILL.md** - SDK initialization, auto-instrumentation overview
2. **references/auto-instrumentation.md** - What gets instrumented
3. **examples/express-api.md** - Complete working example

### Adding Custom Telemetry

4. **references/manual-instrumentation.md** - Custom spans, metrics
5. **references/sdk-setup.md** - Advanced configuration

### Framework-Specific

- **examples/nextjs-app.md** - Next.js App Router setup

## File Guide

| File | When to Read | Content Type |
|------|--------------|--------------|
| [SKILL.md](./SKILL.md) | First | Procedural guide |
| [references/sdk-setup.md](./references/sdk-setup.md) | Deep dive | Reference |
| [references/auto-instrumentation.md](./references/auto-instrumentation.md) | Setup | Reference |
| [references/manual-instrumentation.md](./references/manual-instrumentation.md) | Custom spans | Reference |
| [examples/express-api.md](./examples/express-api.md) | Learning | Tutorial |
| [examples/nextjs-app.md](./examples/nextjs-app.md) | Next.js | Tutorial |

## Prerequisites

```
Node.js 18+
TypeScript 4.7+ (optional)
```

## Minimum Setup

```bash
npm install @opentelemetry/sdk-node \
  @opentelemetry/sdk-trace-node \
  @opentelemetry/instrumentation-http
```

```javascript
// instrumentation.js
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { ConsoleSpanExporter } = require('@opentelemetry/sdk-trace-node');
const { HttpInstrumentation } = require('@opentelemetry/instrumentation-http');

const sdk = new NodeSDK({
  serviceName: 'my-service',
  traceExporter: new ConsoleSpanExporter(),
  instrumentations: [new HttpInstrumentation()]
});

sdk.start();
```

```bash
# The --import flag is required!
node --import ./instrumentation.js app.js
```

## Production Setup

```bash
npm install @opentelemetry/sdk-node \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-proto
```

```javascript
// instrumentation.js
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-proto');

const sdk = new NodeSDK({
  serviceName: process.env.OTEL_SERVICE_NAME || 'my-service',
  traceExporter: new OTLPTraceExporter({
    url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT
  }),
  instrumentations: [getNodeAutoInstrumentations()]
});

sdk.start();
```

## Installation

```bash
npx skills add mthines/skills/otel-nodejs
```

## Usage

This skill activates when you ask about:

- "Set up OTel for Express"
- "Add tracing to my Node.js app"
- "Configure auto-instrumentation"
- "Add custom spans for business logic"
- "Integrate pino with trace context"

## Related Skills

- `@otel-telemetry` - Core concepts (read for signal selection, sampling)

## Troubleshooting

See [COMMON-MISTAKES.md](../COMMON-MISTAKES.md) for solutions to:
- Traces not appearing
- Missing parent spans
- Auto-instrumentation not working

## Official Documentation

- [OpenTelemetry JS](https://opentelemetry.io/docs/languages/js/)
- [Getting Started](https://opentelemetry.io/docs/languages/js/getting-started/nodejs/)
- [API Reference](https://open-telemetry.github.io/opentelemetry-js/)
- [Instrumentation Libraries](https://opentelemetry.io/docs/languages/js/libraries/)

## License

MIT

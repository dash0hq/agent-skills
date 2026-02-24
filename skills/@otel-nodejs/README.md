# @otel-nodejs

OpenTelemetry implementation skill for Node.js and TypeScript applications.

## What This Skill Provides

- **SDK setup** - Initialization patterns and configuration
- **Auto-instrumentation** - Automatic tracing for common libraries
- **Manual instrumentation** - Custom spans, metrics, and logs
- **Framework integration** - Express, Fastify, NestJS, Next.js guides
- **Logger integration** - pino, winston with trace correlation

## Installation

```bash
npx skills add mthines/skills/otel-nodejs
```

## Prerequisites

- Node.js 18+
- TypeScript 4.7+ (if using TypeScript)

## Usage

This skill activates when you ask about:

- Node.js observability setup
- Express/Fastify/NestJS instrumentation
- TypeScript telemetry
- pino/winston OTel integration
- Manual span creation in Node.js

## References

| File | Description |
|------|-------------|
| [sdk-setup.md](./references/sdk-setup.md) | SDK initialization patterns |
| [auto-instrumentation.md](./references/auto-instrumentation.md) | Automatic instrumentation setup |
| [manual-instrumentation.md](./references/manual-instrumentation.md) | Custom spans, metrics, logs |

## Examples

| File | Description |
|------|-------------|
| [express-api.md](./examples/express-api.md) | Complete Express REST API example |
| [nextjs-app.md](./examples/nextjs-app.md) | Next.js App Router example |

## Quick Start

```bash
npm install @opentelemetry/sdk-node \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-proto
```

```typescript
// instrumentation.ts
import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-proto';

const sdk = new NodeSDK({
  traceExporter: new OTLPTraceExporter(),
  instrumentations: [getNodeAutoInstrumentations()]
});

sdk.start();
```

```bash
node --import ./instrumentation.js app.js
```

## Related Skills

- `@otel-telemetry` - Core concepts and best practices (required)

## Official Documentation

- [OpenTelemetry JS](https://opentelemetry.io/docs/languages/js/)
- [OTel JS API Reference](https://open-telemetry.github.io/opentelemetry-js/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

## License

MIT

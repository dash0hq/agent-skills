# otel-instrumentation

Expert guidance for implementing high-quality, cost-efficient OpenTelemetry telemetry across Node.js and browser applications.

## Structure

```
otel-instrumentation/
├── SKILL.md              # Skill manifest and entry point
├── README.md             # This file
└── rules/
    ├── telemetry.md      # Spans, metrics, logs, cardinality
    ├── nodejs.md         # Node.js instrumentation
    ├── browser.md        # Browser instrumentation
    └── nextjs.md         # Next.js full-stack instrumentation
```

## Getting Started

Install the skill:

```bash
npx skills add dash0/otel-instrumentation
```

The skill activates automatically when working on observability tasks.

## Rules

| Rule | Impact | Description |
|------|--------|-------------|
| [telemetry](./rules/telemetry.md) | CRITICAL | Spans, metrics, logs, cardinality management |
| [nodejs](./rules/nodejs.md) | HIGH | Node.js auto-instrumentation setup |
| [browser](./rules/browser.md) | HIGH | Browser instrumentation with Dash0 SDK |
| [nextjs](./rules/nextjs.md) | HIGH | Next.js full-stack instrumentation (App Router) |

## Rule File Structure

Each rule follows a consistent format:

```yaml
---
title: "Rule Title"
impact: CRITICAL | HIGH | MEDIUM | LOW
tags:
  - telemetry
  - spans
---
```

**Content sections:**
1. Core concepts and quick start
2. Implementation with code examples
3. Best practices (do's and don'ts)
4. Troubleshooting common issues

## Impact Levels

| Level | Meaning |
|-------|---------|
| CRITICAL | Affects data quality, costs, or system reliability |
| HIGH | Significant impact on observability effectiveness |
| MEDIUM | Improves telemetry quality or developer experience |
| LOW | Nice-to-have optimizations |

## Quick Start

**Node.js:**
```bash
npm install @opentelemetry/auto-instrumentations-node

export OTEL_SERVICE_NAME="my-service"
export OTEL_TRACES_EXPORTER="otlp"  # Required! Defaults to "none"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_TOKEN"
# Use --import for ESM projects, --require for CommonJS
export NODE_OPTIONS="--import @opentelemetry/auto-instrumentations-node/register"

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

## Resources

- [OpenTelemetry Docs](https://opentelemetry.io/docs/)
- [Dash0 Integration Hub](https://www.dash0.com/hub/integrations)
- [Dash0 Guides](https://www.dash0.com/guides?category=opentelemetry)

## Contributing

1. Follow the rule template in `rules/_template.md`
2. Use concrete code examples over abstract explanations
3. Include both "good" and "bad" patterns
4. Keep examples copy-pasteable

## License

MIT

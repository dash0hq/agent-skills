# otel-instrumentation

Expert guidance for implementing high-quality, cost-efficient OpenTelemetry telemetry across multiple languages and platforms.

## Structure

```
otel-instrumentation/
├── SKILL.md              # Skill manifest and entry point
├── README.md             # This file
└── rules/
    ├── telemetry.md      # Signal overview and correlation
    ├── resources.md      # Resource attributes
    ├── metrics.md        # Instrument types, naming, cardinality
    ├── logs.md           # Structured logging, severity, trace correlation
    └── sdks/
        ├── nodejs.md     # Node.js instrumentation
        ├── go.md         # Go instrumentation
        ├── python.md     # Python instrumentation
        ├── java.md       # Java instrumentation
        ├── dotnet.md     # .NET instrumentation
        ├── ruby.md       # Ruby instrumentation
        ├── php.md        # PHP instrumentation
        ├── browser.md    # Browser instrumentation
        └── nextjs.md     # Next.js full-stack instrumentation
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
| [telemetry](./rules/telemetry.md) | CRITICAL | Signal overview and correlation |
| [resources](./rules/resources.md) | CRITICAL | Resource attributes - service identity, environment, Kubernetes |
| [metrics](./rules/metrics.md) | CRITICAL | Instrument types, naming, units, cardinality |
| [logs](./rules/logs.md) | CRITICAL | Structured logging, severity, trace correlation |
| [nodejs](./rules/sdks/nodejs.md) | HIGH | Node.js auto-instrumentation setup |
| [go](./rules/sdks/go.md) | HIGH | Go instrumentation setup |
| [python](./rules/sdks/python.md) | HIGH | Python auto-instrumentation setup |
| [java](./rules/sdks/java.md) | HIGH | Java auto-instrumentation setup |
| [dotnet](./rules/sdks/dotnet.md) | HIGH | .NET auto-instrumentation setup |
| [ruby](./rules/sdks/ruby.md) | HIGH | Ruby instrumentation setup |
| [php](./rules/sdks/php.md) | HIGH | PHP auto-instrumentation setup |
| [browser](./rules/sdks/browser.md) | HIGH | Browser instrumentation with Dash0 SDK |
| [nextjs](./rules/sdks/nextjs.md) | HIGH | Next.js full-stack instrumentation (App Router) |

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

**Get your credentials:**
- **OTLP Endpoint**: In Dash0: [Settings → Organization → Endpoints](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Cb29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
- **Auth Token**: In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)

**Node.js:**
```bash
npm install @opentelemetry/auto-instrumentations-node

export OTEL_SERVICE_NAME="my-service"
export OTEL_TRACES_EXPORTER="otlp"  # Required! Defaults to "none"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer <AUTH_TOKEN>"
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
  endpoint: { url: "https://<OTLP_ENDPOINT>", authToken: "<AUTH_TOKEN>" }
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

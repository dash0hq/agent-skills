# otel-instrumentation

Expert guidance for implementing high-quality, cost-efficient OpenTelemetry telemetry across Node.js and browser applications.

## Quick Start

If you're new to OpenTelemetry, start with these rules:

1. `core-setup` - Basic SDK configuration
2. `signal-spans` - Understanding traces
3. Your platform: `nodejs-sdk-setup` or `browser-sdk-setup`

## Rule Categories

| Section | Rules | Use When |
|---------|-------|----------|
| **Signal Types** | `signal-spans`, `signal-metrics`, `signal-logs` | Understanding telemetry signals |
| **Core Concepts** | `core-setup`, `core-telemetry-quality` | Production setup, cost control |
| **Dash0** | `dash0-overview`, `dash0-mcp-server`, `dash0-agent0` | Using Dash0 platform |
| **Node.js** | `nodejs-sdk-setup`, `nodejs-auto-instrumentation`, `nodejs-manual-instrumentation` | Backend instrumentation |
| **Browser** | `browser-sdk-setup`, `browser-auto-instrumentation`, `browser-manual-instrumentation`, `browser-server-correlation` | Frontend instrumentation |

## Key Principles

### Signal Density Over Volume

Every telemetry item should serve one of three purposes:
- **Detect** - Help identify that something is wrong
- **Localize** - Help pinpoint where the problem is
- **Explain** - Help understand why it happened

If it doesn't serve one of these purposes, don't emit it.

### Push Reduction Early

```
SDK (sampling)  →  Collector (filtering)  →  Backend (retention)
     ↓                    ↓                       ↓
  Cheapest           Centralized              Most flexible
```

### SLO-Aware Policies

- **Never sample** metrics used in SLO calculations
- **Always capture** error traces for SLO violations
- **Prioritize** data needed for incident response

## Installation

```bash
npx skills add dash0/skills/otel-instrumentation
```

## Official Documentation

- [OpenTelemetry Docs](https://opentelemetry.io/docs/)
- [Signals Overview](https://opentelemetry.io/docs/concepts/signals/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
- [Collector](https://opentelemetry.io/docs/collector/)

## License

MIT

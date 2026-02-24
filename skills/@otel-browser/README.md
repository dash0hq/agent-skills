# @otel-browser

OpenTelemetry skill for browser/client-side applications with TypeScript.

## New Here?

If you're new to OpenTelemetry, start with [GETTING-STARTED.md](../GETTING-STARTED.md) first. This skill is for **browser-specific implementation** after you understand the basics.

## What This Skill Covers

| Topic | What You'll Learn |
|-------|-------------------|
| Browser SDK setup | WebTracerProvider, context managers, exporters |
| Auto-instrumentation | Document load, fetch, user interactions |
| Server correlation | Connecting frontend traces to backend traces |
| Web Vitals | Tracking LCP, CLS, INP alongside traces |
| Framework integration | React, Vue, Next.js client-side |

## Reading Order

### For Getting Started

1. **SKILL.md** (start here) - Full setup guide, quick start
2. **references/sdk-setup.md** - Detailed SDK configuration
3. **references/auto-instrumentation.md** - What gets auto-instrumented

### For Full-Stack Tracing

1. **references/server-correlation.md** - Connecting browser to server traces
2. **examples/react-spa.md** or **examples/nextjs-client.md** - Complete examples

## File Guide

| File | When to Read | Content Type |
|------|--------------|--------------|
| [SKILL.md](./SKILL.md) | First | Procedural guide |
| [references/sdk-setup.md](./references/sdk-setup.md) | SDK configuration | Reference |
| [references/auto-instrumentation.md](./references/auto-instrumentation.md) | Auto-instrumentation | Reference |
| [references/manual-instrumentation.md](./references/manual-instrumentation.md) | Custom spans | Reference |
| [references/server-correlation.md](./references/server-correlation.md) | Full-stack tracing | Reference |
| [examples/react-spa.md](./examples/react-spa.md) | React apps | Example |
| [examples/nextjs-client.md](./examples/nextjs-client.md) | Next.js apps | Example |

## Installation

```bash
npx skills add mthines/skills/otel-browser
```

## Usage

This skill activates when you ask about:

- "Add tracing to my React app"
- "Track user interactions"
- "Connect frontend and backend traces"
- "Monitor Web Vitals with OTel"
- "Browser observability setup"

## Key Concept: Full-Stack Correlation

The power of browser telemetry is seeing the complete user journey:

```
┌─────────────────────────────────────────────────────────────────┐
│                        Single Trace                              │
├─────────────────────────────────────────────────────────────────┤
│ Browser: page.load ──────────────────────────────────  200ms    │
│   └─ Browser: button.click ───────────────────────────  50ms    │
│       └─ Browser: fetch /api/order ──────────────────  150ms    │
│           └─ Server: POST /api/order ────────────────  140ms    │
│               └─ Server: db.query ───────────────────   80ms    │
│               └─ Server: cache.set ──────────────────   10ms    │
└─────────────────────────────────────────────────────────────────┘
```

## Core Packages

```bash
# Minimal
npm install @opentelemetry/api \
  @opentelemetry/sdk-trace-web \
  @opentelemetry/exporter-trace-otlp-http \
  @opentelemetry/context-zone

# With auto-instrumentation
npm install @opentelemetry/instrumentation-document-load \
  @opentelemetry/instrumentation-fetch \
  @opentelemetry/instrumentation-user-interaction
```

## Official Documentation

- [OTel JavaScript Browser](https://opentelemetry.io/docs/languages/js/getting-started/browser/)
- [Web Instrumentations](https://github.com/open-telemetry/opentelemetry-js-contrib/tree/main/plugins/web)
- [W3C Trace Context](https://www.w3.org/TR/trace-context/)

## License

MIT

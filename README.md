# Skills

A collection of expert skills for implementing high-quality observability with OpenTelemetry.

## Available Skills

| Skill | Description | Use When |
|-------|-------------|----------|
| [`@otel-telemetry`](./skills/@otel-telemetry/) | Core OTel concepts, signals, sampling, cost optimization | Learning concepts, designing telemetry strategy |
| [`@otel-nodejs`](./skills/@otel-nodejs/) | Node.js/TypeScript SDK setup and instrumentation | Building Node.js backends (Express, Fastify, NestJS) |
| [`@otel-browser`](./skills/@otel-browser/) | Browser/client-side instrumentation with server correlation | Building React, Vue, Next.js frontends |

## Quick Start

New to OpenTelemetry? Start here:

1. **[Getting Started](./skills/GETTING-STARTED.md)** - See your first trace in 5 minutes
2. **[Learning Path](./skills/LEARNING-PATH.md)** - Structured progression from beginner to production
3. **[Common Mistakes](./skills/COMMON-MISTAKES.md)** - Solutions to frequent problems

## What is OpenTelemetry?

OpenTelemetry (OTel) is the industry standard for collecting observability data:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Your Application                             │
│                                                                  │
│   TRACES              METRICS              LOGS                  │
│   "Request paths"     "How many/fast"      "What happened"       │
└──────────────────────────┬───────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────────┐
│   OTel SDK  ──▶  Collector  ──▶  Backend (Dash0, Grafana, etc.) │
└──────────────────────────────────────────────────────────────────┘
```

## Full-Stack Tracing

Connect browser to server for complete visibility:

```
Browser: page.load ──────────────────────────────────  200ms
  └─ Browser: button.click ───────────────────────────  50ms
      └─ Browser: fetch /api/order ──────────────────  150ms
          └─ Server: POST /api/order ────────────────  140ms
              └─ Server: db.query ───────────────────   80ms
```

## Installation

```bash
# Core concepts and best practices
npx skills add dash0/skills/otel-telemetry

# Node.js implementation
npx skills add dash0/skills/otel-nodejs

# Browser/client-side implementation
npx skills add dash0/skills/otel-browser
```

## Repository Structure

```
skills/
├── GETTING-STARTED.md          # First trace in 5 minutes
├── LEARNING-PATH.md            # Reading order guide
├── COMMON-MISTAKES.md          # Troubleshooting
├── quickstart/                 # Hands-on tutorials
│   ├── 01-first-trace.md
│   ├── 02-send-to-dash0.md
│   └── 03-add-custom-spans.md
├── @otel-telemetry/            # Core skill
│   ├── SKILL.md
│   ├── references/signals/     # Spans, metrics, logs
│   └── examples/               # Cost optimization
├── @otel-nodejs/               # Node.js skill
│   ├── SKILL.md
│   ├── references/             # SDK, auto/manual instrumentation
│   └── examples/               # Express, Next.js
└── @otel-browser/              # Browser skill
    ├── SKILL.md
    ├── references/             # SDK, server correlation
    └── examples/               # React SPA, Next.js client
```

## Key Principles

- **Signal density over volume** - Every telemetry item should help detect, localize, or explain issues
- **Push reduction early** - SDK sampling → Collector filtering → Backend retention
- **SLO-aware policies** - Never sample data feeding your SLOs
- **Cost as a constraint** - Define your budget, optimize until it fits

## Official Documentation

- [OpenTelemetry Docs](https://opentelemetry.io/docs/)
- [OTel JavaScript](https://opentelemetry.io/docs/languages/js/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

## License

MIT

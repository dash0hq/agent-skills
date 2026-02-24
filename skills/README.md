# OpenTelemetry Skills

Expert guidance for implementing high-quality, cost-efficient observability with OpenTelemetry.

## New to OpenTelemetry?

**Start here:** [GETTING-STARTED.md](./GETTING-STARTED.md) - See your first trace in 5 minutes.

Then follow the [LEARNING-PATH.md](./LEARNING-PATH.md) for a structured progression from beginner to production-ready.

Stuck? Check [COMMON-MISTAKES.md](./COMMON-MISTAKES.md) for solutions to common problems.

## What is OpenTelemetry?

OpenTelemetry (OTel) is the standard for collecting observability data from your applications:

```
┌─────────────────────────────────────────┐
│  Your Application                       │
│                                         │
│  TRACES    METRICS    LOGS              │
│  "Request  "How many  "What             │
│   paths"    & how     happened"         │
│             fast"                       │
└────────────────┬────────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────┐
│  OTel SDK → Collector → Backend        │
│  (collect)   (process)   (store/view)  │
└────────────────────────────────────────┘
```

- **Traces** show one request's journey through your system
- **Metrics** are numbers over time (requests/sec, latency)
- **Logs** are timestamped events

## Available Skills

| Skill | Description |
|-------|-------------|
| `@otel-telemetry` | Core concepts, signals, sampling strategies, cost optimization |
| `@otel-nodejs` | Node.js/TypeScript SDK setup, instrumentation, examples |
| `@otel-browser` | Browser/client-side instrumentation, Web Vitals, server correlation |

## Quickstart Guides

| Guide | Time | What You'll Learn |
|-------|------|-------------------|
| [01-first-trace.md](./quickstart/01-first-trace.md) | 10 min | Console output, understanding spans |
| [02-send-to-dash0.md](./quickstart/02-send-to-dash0.md) | 10 min | See traces in Dash0 |
| [03-add-custom-spans.md](./quickstart/03-add-custom-spans.md) | 15 min | Track your business logic |

## Choose Your Backend

Before going to production, pick where your data goes:

| Backend | Setup | Cost | Best For |
|---------|-------|------|----------|
| Console | 0 min | Free | Learning, debugging |
| [Dash0](./quickstart/02-send-to-dash0.md) | 10 min | Free trial | Development & Production |
| Grafana Cloud | 15 min | Free tier | Small projects |

## Install Skills

```bash
# Core concepts and best practices
npx skills add mthines/skills/otel-telemetry

# Node.js implementation
npx skills add mthines/skills/otel-nodejs

# Browser/client-side implementation
npx skills add mthines/skills/otel-browser
```

## File Organization

```
skills/
├── GETTING-STARTED.md      ← Start here
├── LEARNING-PATH.md        ← Reading order
├── COMMON-MISTAKES.md      ← Troubleshooting
├── quickstart/             ← Hands-on tutorials
│   ├── 01-first-trace.md
│   ├── 02-send-to-dash0.md
│   └── 03-add-custom-spans.md
├── @otel-telemetry/        ← Core skill (concepts)
│   ├── SKILL.md            ← Signal selection, sampling
│   ├── references/         ← Per-signal deep dives
│   └── examples/           ← Cost optimization scenarios
├── @otel-nodejs/           ← Language skill (Node.js)
│   ├── SKILL.md            ← SDK setup, framework guides
│   ├── references/         ← Auto/manual instrumentation
│   └── examples/           ← Express, Next.js examples
└── @otel-browser/          ← Browser skill (client-side)
    ├── SKILL.md            ← Browser SDK, Web Vitals
    ├── references/         ← Server correlation, auto-instrumentation
    └── examples/           ← React SPA, Next.js client
```

## Key Principles

- **Signal density over volume** - Every telemetry item should help detect, localize, or explain issues
- **Push reduction early** - SDK sampling → Collector filtering → Backend retention
- **SLO-aware policies** - Never sample data feeding your SLOs
- **Environment-specific** - Different policies for prod vs dev
- **Cost as a constraint** - Define your budget, optimize until it fits

## Official Documentation

- [OpenTelemetry Docs](https://opentelemetry.io/docs/)
- [OTel JavaScript](https://opentelemetry.io/docs/languages/js/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

## Full-Stack Tracing

Connect browser to server for complete visibility:

```
┌─────────────────────────────────────────────────────────────────┐
│                        Single Trace                             │
├─────────────────────────────────────────────────────────────────┤
│ Browser: page.load ──────────────────────────────────  200ms    │
│   └─ Browser: button.click ───────────────────────────  50ms    │
│       └─ Browser: fetch /api/order ──────────────────  150ms    │
│           └─ Server: POST /api/order ────────────────  140ms    │
│               └─ Server: db.query ───────────────────   80ms    │
└─────────────────────────────────────────────────────────────────┘
```

See [@otel-browser/references/server-correlation.md](./@otel-browser/references/server-correlation.md) for setup.

## Future Language Skills

Coming soon:
- `@otel-python` - Python with Django/FastAPI/Flask
- `@otel-go` - Go with chi/gin/fiber
- `@otel-java` - Java with Spring Boot
- `@otel-dotnet` - .NET with ASP.NET Core

## License

MIT

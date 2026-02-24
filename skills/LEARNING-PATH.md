# OpenTelemetry Learning Path

A structured guide to learning OpenTelemetry, from your first trace to production deployment.

## Overview

```
Day 1          Week 1              Week 2+            Production
─────────────────────────────────────────────────────────────────
First trace → Auto-instrumentation → Custom spans → Cost optimization
Console out → Dash0               → Metrics/logs → Sampling strategies
```

---

## Day 1: Your First Traces

**Goal:** See a trace, understand what it is.

### Reading Order

| Step | File | Time | What You'll Learn |
|------|------|------|-------------------|
| 1 | [GETTING-STARTED.md](./GETTING-STARTED.md) | 10 min | What OTel is, 5-min quickstart |
| 2 | [quickstart/01-first-trace.md](./quickstart/01-first-trace.md) | 10 min | Console output, understanding spans |
| 3 | [quickstart/02-send-to-dash0.md](./quickstart/02-send-to-dash0.md) | 10 min | Visual trace exploration in Dash0 |

### Checkpoint

You should be able to:
- [ ] Run an app with OTel and see traces in Dash0
- [ ] Explain what a trace and span are
- [ ] Know why `--import` flag is needed

---

## Week 1: Basic Instrumentation

**Goal:** Instrument a real application with auto-instrumentation.

### Reading Order

| Step | File | Time | What You'll Learn |
|------|------|------|-------------------|
| 1 | [quickstart/03-add-custom-spans.md](./quickstart/03-add-custom-spans.md) | 15 min | Creating custom spans |
| 2 | [@otel-nodejs/SKILL.md](./@otel-nodejs/SKILL.md) | 30 min | Full Node.js setup patterns |
| 3 | [@otel-nodejs/references/auto-instrumentation.md](./@otel-nodejs/references/auto-instrumentation.md) | 20 min | What gets auto-instrumented |
| 4 | [@otel-nodejs/examples/express-api.md](./@otel-nodejs/examples/express-api.md) | 30 min | Complete working example |

### Checkpoint

You should be able to:
- [ ] Set up OTel for an Express/Fastify app
- [ ] Add custom spans for business logic
- [ ] See nested spans in Dash0 (HTTP → Express → your code)
- [ ] Add attributes and events to spans

---

## Week 2: Metrics and Logs

**Goal:** Add metrics and correlated logs.

### Reading Order

| Step | File | Time | What You'll Learn |
|------|------|------|-------------------|
| 1 | [@otel-telemetry/SKILL.md](./@otel-telemetry/SKILL.md) (Signal Selection) | 15 min | When to use traces vs metrics vs logs |
| 2 | [@otel-telemetry/references/signals/metrics.md](./@otel-telemetry/references/signals/metrics.md) | 30 min | Counter, histogram, gauge |
| 3 | [@otel-telemetry/references/signals/logs.md](./@otel-telemetry/references/signals/logs.md) | 20 min | Structured logging with trace context |
| 4 | [@otel-nodejs/references/manual-instrumentation.md](./@otel-nodejs/references/manual-instrumentation.md) | 20 min | Recording metrics in Node.js |

### Checkpoint

You should be able to:
- [ ] Choose the right signal (trace/metric/log) for a use case
- [ ] Create counters and histograms for golden signals
- [ ] Inject trace context into logs (pino/winston)
- [ ] Correlate logs with traces in your backend

---

## Production Readiness

**Goal:** Configure OTel for production with cost controls.

### Reading Order

| Step | File | Time | What You'll Learn |
|------|------|------|-------------------|
| 1 | [@otel-telemetry/references/setup.md](./@otel-telemetry/references/setup.md) | 20 min | Backend configuration, Dash0 |
| 2 | [@otel-telemetry/references/telemetry-quality.md](./@otel-telemetry/references/telemetry-quality.md) | 25 min | Cardinality, attribute hygiene |
| 3 | [@otel-telemetry/SKILL.md](./@otel-telemetry/SKILL.md) (Sampling) | 20 min | Head/tail sampling strategies |
| 4 | [@otel-telemetry/examples/cost-optimization.md](./@otel-telemetry/examples/cost-optimization.md) | 30 min | Real-world cost reduction |
| 5 | [@otel-telemetry/references/collector.md](./@otel-telemetry/references/collector.md) | 30 min | Collector deployment |

### Checkpoint

You should be able to:
- [ ] Configure sampling for production (1-10% default, 100% errors)
- [ ] Control cardinality in metrics
- [ ] Deploy OTel Collector for centralized policy
- [ ] Estimate and control telemetry costs

---

## Quick Reference by Task

### "I want to..."

| Task | Go to |
|------|-------|
| See my first trace | [GETTING-STARTED.md](./GETTING-STARTED.md) |
| Debug why traces aren't appearing | [COMMON-MISTAKES.md](./COMMON-MISTAKES.md) |
| Add tracing to Express app | [@otel-nodejs/examples/express-api.md](./@otel-nodejs/examples/express-api.md) |
| Add tracing to Next.js | [@otel-nodejs/examples/nextjs-app.md](./@otel-nodejs/examples/nextjs-app.md) |
| Track custom business operations | [quickstart/03-add-custom-spans.md](./quickstart/03-add-custom-spans.md) |
| Add metrics (counters, histograms) | [@otel-telemetry/references/signals/metrics.md](./@otel-telemetry/references/signals/metrics.md) |
| Correlate logs with traces | [@otel-telemetry/references/signals/logs.md](./@otel-telemetry/references/signals/logs.md) |
| Control cardinality / attribute hygiene | [@otel-telemetry/references/telemetry-quality.md](./@otel-telemetry/references/telemetry-quality.md) |
| Reduce telemetry costs | [@otel-telemetry/examples/cost-optimization.md](./@otel-telemetry/examples/cost-optimization.md) |
| Set up Collector | [@otel-telemetry/references/collector.md](./@otel-telemetry/references/collector.md) |
| Connect to Dash0 | [@otel-telemetry/references/setup.md](./@otel-telemetry/references/setup.md) |

---

## File Categories

### Tutorials (Follow Along)
- `GETTING-STARTED.md` - First trace in 5 minutes
- `quickstart/*.md` - Progressive hands-on guides
- `@otel-nodejs/examples/*.md` - Complete application examples

### Procedural Guides (How To)
- `@otel-telemetry/SKILL.md` - Requirements gathering, signal selection
- `@otel-nodejs/SKILL.md` - Node.js setup procedures

### Reference (Look Up)
- `@otel-telemetry/references/*.md` - Signal-specific details
- `@otel-nodejs/references/*.md` - Node.js SDK details

### Troubleshooting
- `COMMON-MISTAKES.md` - FAQ and debugging

---

## Time Investment

| Level | Time | Result |
|-------|------|--------|
| Basic traces | 1-2 hours | Console/Dash0 traces working |
| Auto-instrumentation | 3-4 hours | Production-like Express app |
| All signals | 6-8 hours | Traces + metrics + logs |
| Production ready | 10-12 hours | Sampling, collector, cost control |

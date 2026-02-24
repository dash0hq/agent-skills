# @otel-telemetry

Core OpenTelemetry skill providing expert guidance on signals, sampling strategies, and cost optimization.

## New Here?

If you're new to OpenTelemetry, start with [GETTING-STARTED.md](../GETTING-STARTED.md) first. This skill is for **concepts and best practices** after you've seen your first trace.

## What This Skill Covers

| Topic | What You'll Learn |
|-------|-------------------|
| Signal selection | When to use traces vs metrics vs logs |
| Telemetry quality | Cardinality, attribute hygiene, signal density |
| Sampling strategies | Head sampling, tail sampling, SLO-aware policies |
| Collector configuration | Centralized policy enforcement |
| Cost optimization | Real-world strategies to reduce telemetry costs |

## Reading Order

### For Understanding Concepts

1. **SKILL.md** (start here) - Signal selection, sampling design, requirements gathering
2. **references/telemetry-quality.md** - Cardinality, attribute hygiene, signal density
3. **references/signals/spans.md** - When you're working with traces
4. **references/signals/metrics.md** - When you're adding metrics
5. **references/signals/logs.md** - When you're structuring logs

### For Production Setup

1. **references/setup.md** - Backend configuration, Dash0 integration
2. **references/collector.md** - Deploying OTel Collector
3. **examples/cost-optimization.md** - Reducing telemetry costs

## File Guide

| File | When to Read | Content Type |
|------|--------------|--------------|
| [SKILL.md](./SKILL.md) | First | Procedural guide, decision frameworks |
| [references/telemetry-quality.md](./references/telemetry-quality.md) | Quality & cardinality | Reference |
| [references/signals/spans.md](./references/signals/spans.md) | Adding traces | Reference |
| [references/signals/metrics.md](./references/signals/metrics.md) | Adding metrics | Reference |
| [references/signals/logs.md](./references/signals/logs.md) | Adding logs | Reference |
| [references/setup.md](./references/setup.md) | Production setup | Reference |
| [references/collector.md](./references/collector.md) | Advanced setup | Reference |
| [examples/cost-optimization.md](./examples/cost-optimization.md) | Reducing costs | Examples |

## Installation

```bash
npx skills add dash0/skills/otel-telemetry
```

## Usage

This skill activates when you ask about:

- "Which signal should I use?"
- "How do I reduce telemetry costs?"
- "Set up sampling for production"
- "Configure OTel Collector"
- "Control metric cardinality"

## Key Principle

**Signal density over volume** - Every telemetry item should help:
- **Detect** - Identify that something is wrong
- **Localize** - Pinpoint where the problem is
- **Explain** - Understand why it happened

If it doesn't serve one of these purposes, don't emit it.

## Official Documentation

- [OpenTelemetry Docs](https://opentelemetry.io/docs/)
- [Signals Overview](https://opentelemetry.io/docs/concepts/signals/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
- [Collector](https://opentelemetry.io/docs/collector/)
- [Sampling](https://opentelemetry.io/docs/concepts/sampling/)

## License

MIT

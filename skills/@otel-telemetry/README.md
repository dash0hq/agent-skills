# @otel-telemetry

Core OpenTelemetry skill providing expert guidance on signals, sampling strategies, and cost optimization.

## What This Skill Provides

- **Signal selection** - When to use traces vs metrics vs logs
- **Sampling strategies** - Head sampling, tail sampling, and SLO-aware policies
- **Cardinality management** - The #1 cost driver in observability
- **Collector configuration** - Centralized policy enforcement
- **Cost optimization** - Real-world strategies to reduce telemetry costs

## Installation

```bash
npx skills add mthines/skills/otel-telemetry
```

## Usage

This skill activates when you ask about:

- Observability and telemetry setup
- Tracing, metrics, or logging integration
- OpenTelemetry configuration
- Cost optimization for telemetry
- Sampling strategies

## References

| File | Description |
|------|-------------|
| [spans.md](./references/spans.md) | Traces and spans signal reference |
| [metrics.md](./references/metrics.md) | Metrics signal reference |
| [logs.md](./references/logs.md) | Logs signal reference |
| [collector.md](./references/collector.md) | OTel Collector configuration |
| [setup.md](./references/setup.md) | General setup + Dash0 integration |

## Examples

| File | Description |
|------|-------------|
| [cost-optimization.md](./examples/cost-optimization.md) | Real-world cost optimization scenarios |

## Key Principle

**Signal density over volume** - Every telemetry item should help detect, localize, or explain issues. If it doesn't serve one of these purposes, don't emit it.

## Official Documentation

- [OpenTelemetry Docs](https://opentelemetry.io/docs/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
- [Collector](https://opentelemetry.io/docs/collector/)
- [Sampling](https://opentelemetry.io/docs/concepts/sampling/)

## License

MIT

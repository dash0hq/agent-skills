# OpenTelemetry Skills

Expert guidance for implementing high-quality, cost-efficient observability with OpenTelemetry.

## Available Skills

| Skill | Description |
|-------|-------------|
| `@otel-telemetry` | Core concepts, signals, sampling strategies, and best practices |
| `@otel-nodejs` | Node.js/TypeScript SDK setup, instrumentation, and examples |

## Quick Start

### 1. Install the Core Skill

```bash
npx skills add mthines/skills/otel-telemetry
```

### 2. Install a Language Skill

```bash
npx skills add mthines/skills/otel-nodejs
```

### 3. Ask for Help

```
"Help me add tracing to my Express API"
"Set up metrics for my Node.js service"
"Configure OpenTelemetry for cost efficiency"
```

## Learning Path

1. **Start with concepts** - Use `@otel-telemetry` to understand signals, sampling, and cost optimization
2. **Choose your signal** - Traces for requests, metrics for aggregates, logs for events
3. **Implement** - Use your language skill for SDK setup and instrumentation
4. **Optimize** - Apply sampling strategies and cardinality controls

## What These Skills Do

The skills guide you through:

- **Requirements gathering** - Understanding what you need to observe
- **Signal selection** - Choosing between traces, metrics, and logs
- **Implementation** - Setting up SDK and instrumentation
- **Cost optimization** - Sampling strategies and cardinality management
- **Verification** - Testing that telemetry is working correctly

## Key Principles

- **Signal density over volume** - Every telemetry item should help detect, localize, or explain issues
- **Push reduction early** - SDK sampling → Collector filtering → Backend retention
- **SLO-aware policies** - Never sample data feeding your SLOs
- **Environment-specific** - Different policies for prod vs dev
- **Cost as a constraint** - Define your budget, optimize until it fits

## Future Language Skills

Coming soon:

- `@otel-python` - Python with Django/FastAPI/Flask
- `@otel-go` - Go with chi/gin/fiber
- `@otel-java` - Java with Spring Boot
- `@otel-dotnet` - .NET with ASP.NET Core

## License

MIT

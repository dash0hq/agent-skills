# Dash0 Agent Skills

A collection of skills for AI coding agents. Skills are packaged instructions and scripts that extend agent capabilities.

Skills follow the [Agent Skills](https://agentskills.io/) format.

## Available Skills

### otel-instrumentation

Expert guidance for implementing high-quality, cost-efficient OpenTelemetry telemetry. Covers Node.js and browser instrumentation.

**Use when:**
- Setting up observability for a new service
- Adding traces, metrics, or logs to an application
- Debugging instrumentation issues
- Optimizing telemetry costs (cardinality, sampling)
- Connecting browser traces to backend traces

**Rules covered:**
- Telemetry (spans, metrics, logs, cardinality, anti-patterns)
- Node.js (auto-instrumentation, environment variables, Kubernetes)
- Browser (Dash0 SDK, OpenTelemetry JS, server correlation)
- Next.js (App Router, full-stack instrumentation, common gotchas)

**Platforms:**
- Node.js (Express, Fastify, NestJS, etc.)
- Browser (React, Vue, Next.js, etc.)
- Dash0 or any OTLP-compatible backend

## Installation

```bash
npx skills add dash0/otel-instrumentation
```

## Usage

Skills are automatically available once installed. The agent will use them when relevant tasks are detected.

**Examples:**
```
Add OpenTelemetry to my Node.js app
```
```
Set up browser tracing with Dash0
```
```
Help me reduce my telemetry costs
```

## Skill Structure

Each skill contains:
- `SKILL.md` - Instructions for the agent
- `rules/` - Focused guidance documents
- `README.md` - Human-readable documentation

## Key Principles

- **Signal density over volume** - Every telemetry item should help detect, localize, or explain issues
- **Push reduction early** - SDK sampling → Collector filtering → Backend retention
- **SLO-aware policies** - Never sample data feeding your SLOs

## License

MIT

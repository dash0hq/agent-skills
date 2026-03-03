# Dash0 Agent Skills

A collection of skills for AI coding agents to make applications observable with OpenTelemetry and [Dash0](https://www.dash0.com).
Skills are packaged instructions and scripts that extend agent capabilities, following the [Agent Skills](https://agentskills.io/) format.

## Installation

```bash
npx skills add dash0hq/agent-skills
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
```
Write an OTTL expression to redact sensitive headers
```
```
Which attributes should I use for my database spans?
```
```
Help me fix my span naming to follow semantic conventions
```

## Why semantic conventions matter

[OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/) define standardized names, types, and semantics for telemetry attributes, metric names, span names, and status codes.
Following them is the single highest-leverage thing you can do for observability quality.

When instrumentation follows semantic conventions:
- Auto-instrumentation libraries, dashboards, and alerting rules work out of the box.
- Service maps, operation grouping, and error tracking derive correct results without manual configuration.
- Cross-service queries return consistent results because every service speaks the same attribute language.

When conventions are missing or inconsistent, these capabilities degrade silently — no errors, just incomplete data, broken topology views, and fragmented queries.

## Instrumentation score

Some of the guidance in these skills is aligned with the [Instrumentation Score](https://github.com/instrumentation-score/spec) specification — a vendor-neutral corpus of guidance that quantifies how well a service follows OpenTelemetry best practices.
The spec defines impact-weighted rules across resources, spans, metrics, and logs.
Following the guidance in these skills helps your services score higher, which directly translates to better observability outcomes.

## Available skills

### [otel-instrumentation](./skills/otel-instrumentation/SKILL.md)

Expert guidance for implementing high-quality, cost-efficient OpenTelemetry telemetry.
Covers backend and browser instrumentation across multiple languages.

**Use when:**
- Setting up observability for a new service
- Adding traces, metrics, or logs to an application
- Debugging instrumentation issues
- Optimizing telemetry costs (cardinality, sampling)
- Connecting browser traces to backend traces

**Rules covered:**
- Telemetry (signal overview and correlation)
- Resources (service identity, environment, Kubernetes attributes)
- Metrics (instrument types, naming, units, cardinality)
- Logs (structured logging, severity, trace correlation)
- Node.js (auto-instrumentation, environment variables, Kubernetes)
- Go (SDK setup, instrumentation libraries, context propagation)
- Python (auto-instrumentation, Flask, Django, FastAPI)
- Java (javaagent, Spring Boot, JVM system properties)
- .NET (auto-instrumentation, ASP.NET Core, ActivitySource)
- Ruby (SDK setup, Rails, Sinatra)
- PHP (auto-instrumentation, Laravel, Symfony)
- Browser (Dash0 SDK, OpenTelemetry JS, server correlation)
- Next.js (App Router, full-stack instrumentation, common gotchas)

**Platforms:**
- Node.js (Express, Fastify, NestJS, etc.)
- Go (net/http, gin, echo, fiber, etc.)
- Python (Flask, Django, FastAPI, etc.)
- Java (Spring Boot, Servlet, JAX-RS, etc.)
- .NET (ASP.NET Core, Entity Framework, etc.)
- Ruby (Rails, Sinatra, etc.)
- PHP (Laravel, Symfony, etc.)
- Browser (React, Vue, Next.js, etc.)
- Dash0 or any OTLP-compatible backend

### [otel-semantic-conventions](./skills/otel-semantic-conventions/SKILL.md)

Expert guidance for selecting, applying, and reviewing OpenTelemetry semantic conventions — the standardized names, types, and semantics for telemetry attributes, span names, and status codes.

**Use when:**
- Choosing attributes for spans, metrics, or logs
- Naming spans or selecting span kinds
- Mapping HTTP status codes to span status
- Reviewing telemetry for semantic convention compliance
- Migrating from old to new attribute names
- Understanding Dash0 derived attributes

**Rules covered:**
- Attributes (registry, selection, placement, common attributes by domain, namespaces)
- Spans (naming patterns, span kind, status code mapping)
- Versioning (stability levels, migration, Dash0 auto-upgrades)
- Dash0 (derived attributes, feature dependencies)

### [otel-ottl](./skills/otel-ottl/SKILL.md)

Expert guidance for writing and debugging OpenTelemetry Transformation Language (OTTL) expressions for the OpenTelemetry Collector's transform and filter processors.

**Use when:**
- Writing OTTL expressions to transform, filter, or enrich telemetry
- Redacting sensitive data from spans, metrics, or logs
- Configuring transform or filter processors in the Collector
- Debugging OTTL syntax or runtime errors
- Optimizing Collector pipeline performance

**Capabilities:**
- Transform (modify attributes and values)
- Filter (drop unwanted telemetry)
- Redact (hide sensitive information)
- Enrich (add contextual metadata)
- Convert (change data types and formats)

**Contexts:** resource, scope, span, spanevent, metric, datapoint, log

## Skill structure

Each skill contains:
- `SKILL.md` - Instructions for the agent
- `rules/` - Focused guidance documents
- `README.md` - Human-readable documentation

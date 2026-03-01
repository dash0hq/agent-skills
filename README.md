# Dash0 Agent Skills

A collection of skills for AI coding agents. Skills are packaged instructions and scripts that extend agent capabilities.

Skills follow the [Agent Skills](https://agentskills.io/) format.

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

### otel-semantic-conventions

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

### otel-ottl

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

## Skill Structure

Each skill contains:
- `SKILL.md` - Instructions for the agent
- `rules/` - Focused guidance documents
- `README.md` - Human-readable documentation

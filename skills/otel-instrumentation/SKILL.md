---
name: 'otel-instrumentation'
description: Expert guidance for emitting high-quality, cost-efficient OpenTelemetry telemetry. Use when instrumenting applications with traces, metrics, or logs. Triggers on requests for observability, telemetry, tracing, metrics collection, logging integration, or OTel setup.
license: MIT
metadata:
  author: dash0
  version: '2.0.0'
  workflow_type: 'advisory'
  signals:
    - traces
    - metrics
    - logs
---

# OpenTelemetry Instrumentation Guide

Expert guidance for implementing high-quality, cost-efficient OpenTelemetry telemetry.

## Rules

| Rule | Description |
|------|-------------|
| [telemetry](./rules/telemetry.md) | Spans, metrics, logs - how to write good telemetry |
| [nodejs](./rules/nodejs.md) | Node.js instrumentation setup |
| [browser](./rules/browser.md) | Browser instrumentation setup |
| [nextjs](./rules/nextjs.md) | Next.js full-stack instrumentation (App Router) |

## Official Documentation

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
- [Dash0 Integration Hub](https://www.dash0.com/hub/integrations)

## Key Principles

### Signal Density Over Volume

Every telemetry item should serve one of three purposes:
- **Detect** - Help identify that something is wrong
- **Localize** - Help pinpoint where the problem is
- **Explain** - Help understand why it happened

If it doesn't serve one of these purposes, don't emit it.

### Push Reduction Early

```
SDK (sampling)  →  Collector (filtering)  →  Backend (retention)
     ↓                    ↓                       ↓
  Cheapest           Centralized              Most flexible
```

### SLO-Aware Policies

- **Never sample** metrics used in SLO calculations
- **Always capture** error traces for SLO violations
- **Prioritize** data needed for incident response

## Quick Reference

| Use Case | Rule |
|----------|------|
| Node.js backend | [nodejs](./rules/nodejs.md) |
| Browser frontend | [browser](./rules/browser.md) |
| Next.js (App Router) | [nextjs](./rules/nextjs.md) |
| Writing spans/metrics/logs | [telemetry](./rules/telemetry.md) |
| Cardinality management | [telemetry](./rules/telemetry.md) |

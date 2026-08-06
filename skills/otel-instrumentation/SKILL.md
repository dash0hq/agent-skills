---
name: 'otel-instrumentation'
description: Configures trace spans, defines custom metrics, sets up log exporters, and optimizes sampling strategies for OpenTelemetry instrumentation. Use when instrumenting applications with traces, metrics, or logs. Triggers on requests for observability, telemetry, tracing, metrics collection, logging integration, or OTel setup.
license: Apache-2.0
metadata:
  author: dash0
  version: '2.0.0'
  workflow_type: 'advisory'
  supports-traces: "true"
  supports-metrics: "true"
  supports-logs: "true"
---

# OpenTelemetry Instrumentation Guide

Expert guidance for implementing high-quality, cost-efficient OpenTelemetry telemetry.

## Rules & Quick Reference

| Use Case / Rule | Description |
|-----------------|-------------|
| [telemetry](./rules/telemetry.md) | **Entrypoint** — signal types, correlation, and navigation |
| [resolve-values](./rules/resolve-values.md) | Resolving configuration values from the codebase |
| [verify-dependencies](./rules/verify-dependencies.md) | Verifying instrumentation packages and versions exist before adding them |
| [resources](./rules/resources.md) | Resource attributes — service identity and environment |
| [k8s](./rules/platforms/k8s.md) | Kubernetes deployment — downward API, pod spec |
| [spans](./rules/spans.md) | Spans — naming, kind, status, and hygiene |
| [logs](./rules/logs.md) | Logs — structured logging, severity, trace correlation |
| [metrics](./rules/metrics.md) | Metrics — instrument types, naming, units, cardinality |
| [sensitive-data](./rules/sensitive-data.md) | Sensitive data — PII prevention, sanitization, redaction |
| [capture-database-query-parameters](./rules/capture-database-query-parameters.md) | Prepared-statement parameter capture per language (Java, .NET, Python, Node.js, Go) |
| [validation](./rules/validation.md) | Telemetry validation — post-deployment verification checklist |
| [nodejs](./rules/sdks/nodejs.md) | Node.js instrumentation setup |
| [go](./rules/sdks/go.md) | Go instrumentation setup |
| [python](./rules/sdks/python.md) | Python instrumentation setup |
| [java](./rules/sdks/java.md) | Java instrumentation setup |
| [scala](./rules/sdks/scala.md) | Scala instrumentation setup |
| [dotnet](./rules/sdks/dotnet.md) | .NET instrumentation setup |
| [ruby](./rules/sdks/ruby.md) | Ruby instrumentation setup |
| [php](./rules/sdks/php.md) | PHP instrumentation setup |
| [browser](./rules/sdks/browser.md) | Browser instrumentation setup |
| [nextjs](./rules/sdks/nextjs.md) | Next.js full-stack instrumentation (App Router) |

## Official documentation

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
- [Dash0 Integration Hub](https://www.dash0.com/hub/integrations)

## Getting started

Follow these steps when instrumenting an application from scratch:

1. **Pick your SDK rule** — choose the language-specific rule from the table above (e.g., [nodejs](./rules/sdks/nodejs.md), [python](./rules/sdks/python.md)).
2. **Set up resource attributes** — define service identity and environment per [resources](./rules/resources.md).
3. **Add spans, metrics, and logs** — instrument your code following [spans](./rules/spans.md), [metrics](./rules/metrics.md), and [logs](./rules/logs.md).
4. **Guard sensitive data** — scrub PII before export per [sensitive-data](./rules/sensitive-data.md).
5. **Validate** — confirm telemetry reaches the backend using the checklist in [validation](./rules/validation.md).

The snippet below shows a complete span with attributes and status for Node.js — see [nodejs](./rules/sdks/nodejs.md) for full setup including SDK initialisation, exporter configuration, and auto-instrumentation:

```js
import { trace, SpanStatusCode } from '@opentelemetry/api';
const tracer = trace.getTracer('my-service', '1.0.0');

tracer.startActiveSpan('operation-name', async (span) => {
  try {
    span.setAttribute('user.id', userId);
    span.setAttribute('order.id', orderId);

    const result = await processOrder(orderId);

    span.setAttribute('order.status', result.status);
    span.setStatus({ code: SpanStatusCode.OK });
    return result;
  } catch (err) {
    // Record the exception as a structured log record, not span.recordException — see rules/spans.md
    span.setStatus({ code: SpanStatusCode.ERROR, message: `${err.name}: ${err.message}` });
    const spanContext = span.spanContext();
    logger.error('operation-name.failed', {
      'trace_id': spanContext.traceId,
      'span_id': spanContext.spanId,
      'exception.type': err.name,
      'exception.message': err.message,
      'exception.stacktrace': err.stack,
    });
    throw err;
  } finally {
    span.end();
  }
});
```

## Key principles

### Signal density over volume

Every telemetry item should serve one of three purposes:
- **Detect** - Help identify that something is wrong
- **Localize** - Help pinpoint where the problem is
- **Explain** - Help understand why it happened

If it doesn't serve one of these purposes, don't emit it.

### Sample in the pipeline, not the SDK

Use the `AlwaysOn` sampler (the default) in every SDK.
Do not configure SDK-side samplers — they make irreversible decisions before the outcome of a request is known.
Defer all sampling to the [Collector](../otel-collector/rules/sampling.md), where policies can be changed centrally without redeploying applications.

<!-- eval:skip -->
```
SDK (AlwaysOn)  →  Collector (sampling)  →  Backend (retention)
     ↓                    ↓                       ↓
  All spans         Head or tail            Storage policies
  exported          sampling applied
```

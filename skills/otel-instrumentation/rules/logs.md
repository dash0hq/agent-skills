---
title: 'Logs'
impact: HIGH
tags:
  - logs
  - structured-logging
  - severity
  - correlation
---

# Logs

Logs (technically: "log records" in OpenTelemetry) are textual records of discrete events with local context.
They are the primary signal for audit trails, debugging, and explaining causation after metrics and traces have surfaced and localized a problem.

## Structured logging

Always use structured key-value pairs, never string interpolation.
Structured logs are queryable, parseable, and compatible with log analysis tooling.

```javascript
// BAD: unstructured
logger.info(`User ${userId} placed order ${orderId}`);

// GOOD: structured
logger.info('order.placed', {
  user_id: userId,
  order_id: orderId,
  amount: amount,
});
```

When selecting fields to include in structured log records, never spread entire objects (request bodies, headers, form data) — explicitly pick safe fields to avoid accidentally logging sensitive data.
See [structured logging safeguards](./sensitive-data.md#structured-logging-safeguards) for rules and examples.

```javascript
// BAD: spreads the entire request body — may contain passwords, tokens, PII
logger.info('user.signup', { ...req.body, ...getTraceContext() });

// GOOD: explicitly select safe fields
logger.info('user.signup', {
  ...getTraceContext(),
  user_id: req.body.userId,
  plan: req.body.plan,
});
```

## Severity

Always set `severityNumber` on log records.
Log records with severity left as `UNSET` lose filtering and alerting capability.
If you are dealing with access or audit logs, use `9` for `severityNumber`.
When using the OpenTelemetry Collector\u2019s filelog receiver, configure severity parsing explicitly — it does not parse severity by default.

## Trace correlation

Every log record emitted inside an active span should carry `trace_id` and `span_id`.
Without these fields, logs are isolated events that cannot be connected to the request that produced them.

Extract the trace context from the active span and include it as structured fields in every log call:

```javascript
import { trace, context } from '@opentelemetry/api';

function getTraceContext() {
  const span = trace.getSpan(context.active());
  if (!span) return {};
  const ctx = span.spanContext();
  return { trace_id: ctx.traceId, span_id: ctx.spanId };
}

logger.info('order.placed', { ...getTraceContext(), order_id: orderId });
```

Wrap this in a logger helper to avoid repeating the extraction at every call site.
The output must be single-line structured JSON so that log collectors can parse it without multiline aggregation.

## Log events

A log record with a non-empty `event_name` OTLP field or `otel.event.name` log record attribute is an *event* — a named occurrence that tooling can recognize and process as a distinct category.
The `event_name` must uniquely identify the event structure (both its attributes and body).

Use log events for occurrences that meet **both** of these criteria:
- The occurrence has a **stable schema** — the same set of attributes is emitted every time.
- The occurrence represents a **business or operational milestone** that users will want to count, alert on, or filter by — for example, deployments, payment completions, or user sign-ups.

Do **not** use log events for general diagnostic logging, debugging output, or messages whose attributes vary from call to call.
Use regular log records without an `event_name` instead.

```javascript
// GOOD: log event — stable schema, business milestone
logger.emit({
  severityNumber: 9,
  body: 'Deployment succeeded',
  attributes: {
    'otel.event.name': 'deployment.success',
    'deployment.id': deploymentId,
    'service.version': newVersion,
    ...getTraceContext(),
  },
});

// GOOD: regular log record — diagnostic, no fixed schema
logger.info('order.validation', {
  ...getTraceContext(),
  order_id: orderId,
  validation_errors: errors,
});
```

## Exception stack traces

Exception stack traces are multi-line by default in every language.
When logs are written to stdout as structured JSON, a multi-line stack trace breaks the one-line-per-record contract and corrupts log parsing.
Log collectors (filelog receiver, Fluentd, Fluent Bit) treat each line as a separate record, splitting a single exception into dozens of unrelated log entries.

Always serialize exception stack traces as a single string value inside a structured field.
The newlines within the stack trace become escaped characters (`\n`) inside the JSON string, keeping the entire log record on one line.

```json
{"level":"error","msg":"order.failed","error.type":"TypeError","error.message":"Cannot read properties of undefined","error.stack_trace":"TypeError: Cannot read properties of undefined\n    at processOrder (/app/src/orders.js:42:15)\n    at handle (/app/src/routes.js:18:3)","trace_id":"abc123","span_id":"def456"}
```

Configure your logging framework to serialize exceptions into the `exception.stacktrace` field rather than printing them to the console directly.
(While the semantic conventions for logs foresee `exception.stacktrace` as an OTLP log attribute fields, using them also in structured logs makes things easier down the line.)
See the language-specific SDK guides for framework-level configuration.

### Flushing providers on shutdown or crash

OpenTelemetry SDKs batch telemetry before exporting.
If the process exits before the batch is flushed, buffered log records are lost — including data from the request that caused the crash.
Every application must ensure providers are shut down or flushed before process exit.

Abrupt termination (`SIGKILL`, OOM kill, segfault) bypasses all shutdown hooks — no in-process mitigation exists.

See the `Graceful shutdown` in the language-specific SDK rules for the idiomatic shutdown pattern in each runtime.

## Container runtimes

In containerized environments, always write logs to stdout/stderr.
Container runtimes capture stdout/stderr automatically, making logs available through `kubectl logs` and cluster-level log collectors without any additional configuration.
Since the stdout/stderr output of containerized applications can be interleaved, it is imperative to output structured logs as single lines.

### Stdout vs OTLP for log delivery

| Delivery method | Pros | Cons |
|-----------------|------|------|
| Stdout/stderr only | All logs captured (including library, bootstrap, and crash logs); `kubectl logs` always works; no SDK dependency for log delivery | Requires a log collector (filelog receiver or DaemonSet agent) to forward to a backend |
| OTLP only (Logs SDK) | Native OTLP format; no parsing needed at the collector | Bypasses container runtime log pipeline; `kubectl logs` shows nothing; bootstrap/crash logs and library logs are lost; OTLP endpoint outage causes silent log loss |
| Both stdout and OTLP | Belt-and-suspenders coverage | **Duplicate logs** — the backend may receive two copies of every log record (one via OTLP, one via the filelog receiver from the collector deployed on the same Kubernetes node or host), doubling storage costs and cluttering query results |

Use **stdout/stderr only** as the default strategy.
Write structured JSON to stdout and let a log collector (such as the OpenTelemetry Collector's [filelog receiver](../../otel-collector/rules/receivers.md) or a DaemonSet-based agent) pick them up.
This ensures that all logs are always visible through `kubectl logs`, while still being forwarded to your observability backend for querying and correlation.

Do not enable both the OpenTelemetry Logs SDK exporter and a file-based log collector on the same application unless you have explicitly configured deduplication.
Without deduplication, every log record arrives at the backend twice — once via OTLP and once via the filelog receiver — producing duplicate entries that inflate costs and confuse queries.

Sending logs exclusively via OTLP (through the OpenTelemetry Logs SDK) bypasses the container runtime log pipeline.
If the OTLP endpoint is unreachable or the Collector is misconfigured, those logs are lost entirely — they will not appear in `kubectl logs` or any file-based log collector.
It is also very difficult to have *all* the logs of an application over OTLP, like logs from libraries and application bootstrapping or crash logs, which leads to situations where the most valuable logs are not available over OTLP.

## Anti-patterns

### Unstructured logs

```javascript
// BAD
logger.error(`Failed: ${error.message}`);

// GOOD
logger.error('order.failed', {
  error_type: error.name,
  error_message: error.message,
  order_id: orderId,
});
```

### Multi-line log records

Log collectors (filelog receiver, Fluentd, Fluent Bit) treat each line as a separate record.
A multi-line log entry — pretty-printed JSON, raw stack traces, or multi-line messages — is split into multiple unrelated records, corrupting parsing and losing context.

```javascript
// BAD: pretty-printed JSON spans multiple lines
logger.info(JSON.stringify({ event: 'order.placed', order_id: orderId }, null, 2));
// Output:
// {
//   "event": "order.placed",
//   "order_id": "abc-123"
// }
// ↑ collector sees 4 separate log records

// GOOD: single-line structured JSON
logger.info('order.placed', { order_id: orderId });
// Output:
// {"level":"info","msg":"order.placed","order_id":"abc-123"}
```

Disable pretty-printing in all environments where logs are collected from stdout/stderr.
See [Exception stack traces](#exception-stack-traces) for serializing multi-line values within a single-line record.

### Missing trace correlation

```javascript
// BAD: logs without context
logger.info('Payment processed');

// GOOD: logs with trace context
logger.info('payment.processed', {
  ...getTraceContext(),
  payment_id: paymentId,
});
```

## References

- [Logs](https://opentelemetry.io/docs/concepts/signals/logs/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
- [Trace Context in non-OTLP Log Formats](https://opentelemetry.io/docs/specs/otel/compatibility/logging_trace_context/)

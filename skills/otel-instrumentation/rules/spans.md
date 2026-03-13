---
title: "Spans: Naming, Kind, Status, and Hygiene"
impact: HIGH
tags:
  - spans
  - naming
  - span-kind
  - status-code
  - hygiene
  - http
  - messaging
  - sampling
  - instrumentation-score
---

# Spans

For every span, make three decisions: what to **name** it, which **kind** to assign, and when to set its **status** to error.
Use the tables and rules below to make each decision.
Wrong span names break operation dashboards, wrong span kinds break service maps, and wrong status codes break error tracking.

The span hygiene rules in this file are aligned with the [Instrumentation Score](https://github.com/instrumentation-score/spec) specification — a vendor-neutral scoring system that quantifies how well a service follows OpenTelemetry best practices.
Each hygiene rule references the corresponding Instrumentation Score rule ID.

## Span naming
<!-- Instrumentation Score: SPA-003 (Important) -->

Span names MUST be low-cardinality.
The number of unique span names in a system must be bounded and small.

### General pattern: `{verb} {object}`

| Anti-Pattern (high cardinality) | Correct (low cardinality) | Fix |
|---|---|---|
| `GET /api/users/12345` | `GET /api/users/:id` | Use route template, not actual path |
| `SELECT * FROM orders WHERE id=99` | `SELECT orders` | Use table name, not full query |
| `process_payment_for_user_jane` | `process payment` | User identity is an attribute |
| `send_invoice_#98765` | `send invoice` | Invoice number is an attribute |
| `validation_failed` | `validate user_input` | Name the operation, not the outcome |

### Per-signal naming

| Signal | Format | Example |
|---|---|---|
| **HTTP server** | `{method} {http.route}` | `GET /api/users/:id` |
| **HTTP client** | `{method} {url.template}` or `{method}` | `POST /checkout` |
| **Database** | `{db.operation.name} {db.collection.name}` | `SELECT orders` |
| **RPC** | `{rpc.service}/{rpc.method}` | `UserService/GetUser` |
| **Messaging** | `{operation} {destination}` | `publish shop.orders` |

- HTTP: Never use the raw URI path as the span name.
  Use `http.route` (server) or `url.template` (client).
  If unavailable, use just the method, but to go great lengths to make sure `http.route` on the server and `url.template` on the client.
- Database: Fall back through `db.query.summary` > `{operation} {collection}` > `{collection}` > `{db.system.name}`.
- If the method is unknown and normalized to `_OTHER`, use the protocol name alone (e.g., `HTTP`).

## Span kind

Each span has exactly one kind. Choose based on the communication pattern, not the technology.

| Kind | Use When | Examples |
|---|---|---|
| `SERVER` | Handling an inbound synchronous request | Incoming HTTP request, incoming gRPC call |
| `CLIENT` | Making an outbound synchronous request | HTTP call, database query, outbound RPC |
| `PRODUCER` | Initiating an asynchronous operation | Publishing a message to a queue or topic |
| `CONSUMER` | Processing an asynchronous operation | Processing a message from a queue |
| `INTERNAL` | Internal operation with no remote parent/child | In-memory computation, internal function call |

### Common mistakes

- **Using `INTERNAL` for everything.**
  Calls towards databases are `CLIENT`.
  HTTP handlers are `SERVER`.
  Only use `INTERNAL` for operations that genuinely have no remote counterpart.
- **Using `CLIENT` for message publishing.**
  Publishing to a queue is asynchronous — use `PRODUCER`.
  `CLIENT` implies the caller waits for a response.
- **Using `SERVER` for message processing.** Processing a queued message is `CONSUMER`, not `SERVER`, because the producer isn't waiting.

### Messaging kind mapping

| Operation | Span Kind |
|---|---|
| `create` | `PRODUCER` |
| `send` | `PRODUCER` (or `CLIENT` if waiting for ack) |
| `receive` | `CLIENT` |
| `process` | `CONSUMER` |
| `settle` | `CLIENT` |

## Span status code

Leave span status `UNSET` by default.
Only set it to `ERROR` when the operation genuinely failed.
You can set the span status code to `OK` only on confirmed success and, if unsure, leave it to `UNSET`.

### HTTP status code mapping

The rules differ by span kind — this is the most commonly misunderstood convention:

#### Client spans (`SpanKind.CLIENT`)

| HTTP Status | Span Status | Rationale |
|---|---|---|
| 1xx, 2xx, 3xx | `UNSET` | Request succeeded |
| **4xx** | **`ERROR`** | Client's request failed |
| **5xx** | **`ERROR`** | Server error = client failure |
| No response | **`ERROR`** | Connection/timeout failure |

#### Server spans (`SpanKind.SERVER`)

| HTTP Status | Span Status | Rationale |
|---|---|---|
| 1xx, 2xx, 3xx | `UNSET` | Request handled successfully |
| **4xx** | **`UNSET`** | Server responded correctly to a bad request |
| **5xx** | **`ERROR`** | Server failed to handle the request |
| No response | **`ERROR`** | Server-side failure |

The critical distinction: **a 400 Bad Request on a server span is NOT an error** — the server did its job.
The same 400 on the corresponding client span IS an error — the client's request failed.

### General rules

- Set span status to `OK` when the application logic has confirmed that the operation succeeded — for example, after validating a response, completing a transaction, or receiving an explicit acknowledgement.
  `OK` signals to the backend that the operation was verified as successful, not merely that no error was caught.
  Do not set `OK` speculatively; leave the status `UNSET` if the code does not explicitly confirm success.
- Do not set span status to `ERROR` for errors that were retried and ultimately succeeded, or for errors that were intentionally handled.
- When setting span status to `ERROR`, include a status message that describes the failure.
  The message should contain the error class and a short explanation — enough to understand the failure without opening the full trace.
  Do not include stack traces in the status message; record those in a log record with `exception.stacktrace` instead.

```javascript
// BAD: error status without a message
span.setStatus({ code: SpanStatusCode.ERROR });

// BAD: generic message with no diagnostic value
span.setStatus({ code: SpanStatusCode.ERROR, message: 'something went wrong' });

// GOOD: specific message with error class and context
span.setStatus({
  code: SpanStatusCode.ERROR,
  message: `TimeoutError: upstream payment service did not respond within 5s`,
});
```

Verify this rule in integration tests — see [testing trace data](#error-status-has-a-message).

### Do not set `ERROR` for handled or retried errors

Only set span status to `ERROR` when the failure is final.
Do not set it for errors that were retried and ultimately succeeded, or for errors that were intentionally handled (e.g., a fallback path that produces a valid result).

When retrying inside a span, record each failed attempt as a span event and set `ERROR` only after all retries are exhausted.

```javascript
async function fetchWithRetry(url, maxRetries) {
  return tracer.startActiveSpan('http.fetch_with_retry', async (span) => {
    let lastError;
    for (let attempt = 1; attempt <= maxRetries; attempt++) {
      try {
        const response = await fetch(url);
        return response;
      } catch (error) {
        lastError = error;
        span.addEvent('retry', { attempt, error: error.message });
      }
    }

    // All retries exhausted — now set ERROR.
    span.setStatus({ code: SpanStatusCode.ERROR, message: lastError.message });
    span.end();
    throw lastError;
  });
}
```

See the language-specific SDK rules for idiomatic examples in each language.

### Recording exceptions

Record exception details as a structured log record, not as a span event.
The [Span Event API is being deprecated](https://github.com/open-telemetry/opentelemetry-specification/blob/main/oteps/4430-span-event-api-deprecation-plan.md) in favour of log-based events.
Emit the exception as a log record within the active span context so that it carries `trace_id` and `span_id` automatically according to the [logs guidance](./logs.md).

Use a log message that describes the failed operation — not a generic label like `"exception"` or `"error"`.
The `exception.*` attributes carry the exception details; the message provides the operational context that makes the log record useful when scanning a log stream.

```typescript
import { trace, context } from '@opentelemetry/api';

// BAD: uses the deprecated Span Event API
span.recordException(error);

// BAD: generic message with no operational context
logger.error('exception', { 'exception.type': error.name, ... });

// GOOD: descriptive message with exception attributes and trace correlation
const spanContext = trace.getSpan(context.active())?.spanContext();
logger.error('order.charge.failed', {
  'trace_id': spanContext?.traceId,
  'span_id': spanContext?.spanId,
  'exception.type': error.name,
  'exception.message': error.message,
  'exception.stacktrace': error.stack,
});
```

Include `trace_id` and `span_id` so the exception log record can be correlated with the span that produced it; see [trace correlation](./logs.md#trace-correlation) for the `getTraceContext()` helper pattern.
Set `exception.type`, `exception.message`, and `exception.stacktrace` as log record attributes.
Serialize the stack trace as a single string — see [exception stack traces](./logs.md#exception-stack-traces) for formatting rules.

## Span attributes

Auto-instrumentation libraries set protocol-level attributes (`http.request.method`, `db.operation.name`, `url.path`, etc.) automatically.
These are necessary but not sufficient — they describe *how* the system communicates, not *what business operation* is being performed.
Add domain-specific attributes to make traces actionable for debugging and business analysis.

### What to add

Add attributes that answer the question: "when investigating this span during an incident, what context would I need?"

| Domain | Attribute examples | Why |
|--------|-------------------|-----|
| E-commerce | `order.id`, `cart.item_count`, `payment.method` | Identify the affected order without searching logs |
| Auth | `user.id`, `user.role`, `auth.method` | Narrow down which users are impacted |
| Messaging | `message.type`, `queue.depth` (at publish time) | Understand the workload shape |
| Multi-tenant | `tenant.id`, `tenant.plan` | Isolate tenant-specific issues |
| Feature flags | `feature_flag.key`, `feature_flag.variant` | Correlate regressions with flag changes |

### Adding attributes to manually created spans

Set attributes at span creation time or as soon as the values are available.
Prefer `setAttribute` calls over constructor options when the value is computed after the span starts.

```typescript
await tracer.startActiveSpan('process order', async (span) => {
  span.setAttribute('order.id', order.id);
  span.setAttribute('order.total', order.total);
  span.setAttribute('payment.method', order.paymentMethod);
  try {
    await chargePayment(order);
    span.setStatus({ code: SpanStatusCode.OK });
  } catch (error) {
    span.setStatus({
      code: SpanStatusCode.ERROR,
      message: `${error.name}: ${error.message}`,
    });
    throw error;
  } finally {
    span.end();
  }
});
```

### Adding attributes to auto-instrumented spans

Auto-instrumentation creates spans you do not control directly (e.g., the `SERVER` span for an HTTP request).
To enrich these spans with business context, retrieve the active span and add attributes to it.

```typescript
import { trace } from '@opentelemetry/api';

app.post('/api/orders', async (req, res) => {
  const span = trace.getActiveSpan();
  span?.setAttribute('order.id', req.body.orderId);
  span?.setAttribute('tenant.id', req.headers['x-tenant-id']);
  // ... handler logic
});
```

Use this pattern at the earliest point in the request handler where the business context is known.
Do not wrap auto-instrumented spans in a second manual span just to add attributes — that creates unnecessary nesting.

### Attribute naming

Follow the [Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/) conventions:
- Use a dot-separated namespace: `order.id`, not `orderId` or `order_id`.
- Check the registry before inventing a name — use the standard attribute if one exists (e.g., `enduser.id` instead of `user.id`).
- Use a project-specific namespace prefix for truly custom attributes to avoid collisions (e.g., `com.acme.order.priority`).

### Cardinality constraints

Span attributes tolerate higher cardinality than metric attributes, but unbounded values may still cause problems at the backend (indexing costs, slow queries, harder to create metrics in the observability pipeline).

| Safe on spans | Avoid on spans |
|--------------|---------------|
| `order.id`, `user.id`, `tenant.id` | `request.body` (arbitrary size) |
| `feature_flag.key` (bounded set) | `url.full` with query params (unbounded) |
| `payment.method` (bounded set) | Serialized objects or arrays |

See [attribute placement](../metrics.md#attribute-placement) for how cardinality tolerance differs across signals.

## Span hygiene

### Root spans must not be CLIENT
<!-- Instrumentation Score: SPA-004 (Important) -->

A `CLIENT` or `PRODUCER` root span indicates missing instrumentation or lost trace context.
Root spans must describe what the service is *doing* (the unit of work), not what it is *calling* (outgoing requests).
Root spans should have kind `SERVER`, `CONSUMER`, or `INTERNAL`.

If you see `CLIENT` or `PRODUCER` root spans, the likely causes are:
- Missing HTTP server instrumentation (the framework handler span is absent)
- Broken context propagation (the parent context was not extracted from incoming headers)
- A headless operation (cron job, scheduled task, background worker, queue consumer) that lacks a wrapping root span

#### Headless operations

Cron jobs, scheduled tasks, CLI commands, and background workers have no inbound HTTP request to trigger a `SERVER` span via auto-instrumentation.
Without a manual root span, the first auto-instrumented outbound call (e.g., a database query or HTTP request) becomes a `CLIENT` root span, which misrepresents what the service is doing.

Create a manual `SERVER` root span that wraps the entire operation.
This ensures the outbound calls appear as children of a meaningful root span rather than as disconnected `CLIENT` or `PRODUCER` root spans.

```javascript
// BAD: no root span — the database query becomes a CLIENT root span
async function processDaily() {
  await db.query('SELECT * FROM orders WHERE status = $1', ['pending']);
}

// GOOD: manual root span wraps the headless operation
async function processDaily() {
  await tracer.startActiveSpan('process daily orders', { kind: SpanKind.SERVER }, async (span) => {
    try {
      await db.query('SELECT * FROM orders WHERE status = $1', ['pending']);
    } finally {
      span.end();
    }
  });
}
```

### CLIENT and PRODUCER spans must have a parent

Every span of kind `CLIENT` (database query, HTTP call, RPC request) or `PRODUCER` (messaging, job queues) must be a child of a `SERVER`, `CONSUMER`, or `INTERNAL` span.
A `CLIENT` or `PRODUCER` span without such a parent means the trace has no record of *why* the outgoing call was made — it captures the call but not the work that triggered it.

#### When to add a parent span

Auto-instrumentation for web frameworks and messaging libraries creates `SERVER` or `CONSUMER` spans automatically.
`CLIENT` or `PRODUCER` spans for outbound calls made inside those handlers are already children — no action needed.

The problem arises in code that runs outside a request handler: cron jobs, background workers, startup tasks, and CLI commands.
These have no auto-instrumented parent, so the first outbound call becomes a parentless `CLIENT` or `PRODUCER` root span.
Wrap such operations in a manual `SERVER` or `INTERNAL` span — see [headless operations](#headless-operations) for the pattern.

Verify this rule in integration tests — see [testing trace data](#no-parentless-client-or-producer-spans).

### No orphan spans
<!-- Instrumentation Score: SPA-002 (Normal) -->

Every span with a `parent_span_id` must have a corresponding parent span in the trace.
Orphan spans indicate broken context propagation or instrumentation gaps, and result in fragmented, misleading trace views.

Common causes:
- Parent span was dropped by sampling while child span was retained
- Context propagation headers were not forwarded between services
- Parent span ended before child span was created (timing issue)

Verify this rule in integration tests — see [testing trace data](#no-orphan-spans-1).

### Limit INTERNAL spans
<!-- Instrumentation Score: SPA-001 (Normal) -->

Keep `INTERNAL` spans under 10 per service within a single trace.
Excessive internal spans signal over-instrumentation and clutter trace views, making it harder to identify actual bottlenecks.

If you exceed this limit, consider:
- Replacing loop-per-item spans with a single batch span and a `batch.size` attribute
- Using log records instead of child spans for lightweight annotations
- Removing spans that don't add diagnostic value

Verify this rule in integration tests — see [testing trace data](#internal-span-count).

### Avoid excessive short-duration spans
<!-- Instrumentation Score: SPA-005 (Important) -->

A trace must not contain more than 20 spans with a duration under 5 milliseconds.
Exceeding this threshold indicates spans created in tight loops or over-instrumented internal code.
These inflate trace storage without adding observability value.

Instead of creating a span per iteration, create a single span for the batch operation:

```javascript
// BAD: 1,000 spans under 1ms each
items.forEach(item => {
  tracer.startActiveSpan('process.item', span => {
    process(item);
    span.end();
  });
});

// GOOD: single span with context
tracer.startActiveSpan('process.batch', span => {
  span.setAttribute('batch.size', items.length);
  items.forEach(process);
  span.end();
});
```

Verify this rule in integration tests — see [testing trace data](#short-duration-span-count).

## Sampling

Use the `AlwaysOn` sampler in application SDKs.
This is the default in every OpenTelemetry SDK — do not change it.

Do not configure `TraceIdRatioBased`, `ParentBased`, or any other sampler that drops spans in the application.
SDK-side sampling makes irreversible decisions at the head of the trace, before the outcome of the request is known.
A trace that looked unremarkable at the start may turn out to contain an error, a latency spike, or a rarely exercised code path — all of which are lost if the SDK decided not to sample.

Defer all sampling decisions to the [Collector or observability pipeline](../../otel-collector/rules/sampling.md), where they can be changed centrally without redeploying applications.
The Collector can apply head sampling with the [`probabilisticsamplerprocessor`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/probabilisticsamplerprocessor), or tail sampling with the [`tailsamplingprocessor`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor), using criteria that are impossible to evaluate at request start (error status, latency, attribute values).

```bash
# BAD: head sampling in the SDK — loses traces before their outcome is known
export OTEL_TRACES_SAMPLER=traceidratio
export OTEL_TRACES_SAMPLER_ARG=0.1

# GOOD: default AlwaysOn sampler — sampling happens in the Collector
# (no OTEL_TRACES_SAMPLER needed; AlwaysOn is the default)
```

Accurate RED metrics (request rate, error rate, duration) cannot be computed from sampled traces.
If you sample in the Collector, materialize metrics from spans *before* the sampling step — see [connectors](../../otel-collector/rules/pipelines.md#connectors) and [sampling](../../otel-collector/rules/sampling.md#materialize-red-metrics-before-sampling).

## Testing trace data

Treat trace shape — spans, their parent-child relations, kinds, status codes, and attributes — as a functional requirement.
Use an in-memory span exporter in integration tests to capture spans produced by the code under test and assert the rules from this file.

### Test setup

```typescript
import { SpanKind } from '@opentelemetry/api';
import { InMemorySpanExporter, SimpleSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { NodeTracerProvider } from '@opentelemetry/sdk-trace-node';

const exporter = new InMemorySpanExporter();
const provider = new NodeTracerProvider();
provider.addSpanProcessor(new SimpleSpanProcessor(exporter));
provider.register();

// Call between tests to reset captured spans
function resetSpans() {
  exporter.reset();
}

// Helper: get all finished spans
function getSpans() {
  return exporter.getFinishedSpans();
}
```

### No parentless CLIENT or PRODUCER spans

Assert that every `CLIENT` or `PRODUCER` span has a parent.
A failure means code is making outbound calls without a wrapping `SERVER` or `INTERNAL` span — see [headless operations](#headless-operations).

```typescript
function assertNoParentlessOutboundSpans() {
  const parentless = getSpans().filter(
    (s) => (s.kind === SpanKind.CLIENT || s.kind === SpanKind.PRODUCER) && !s.parentSpanId,
  );
  if (parentless.length > 0) {
    const names = parentless.map((s) => s.name).join(', ');
    throw new Error(`CLIENT/PRODUCER root spans detected: ${names}`);
  }
}
```

### No orphan spans

Assert that every span with a `parentSpanId` has a corresponding parent in the trace.
A failure indicates broken context propagation or a parent span that ended or was sampled out.

```typescript
function assertNoOrphanSpans() {
  const spans = getSpans();
  const spanIds = new Set(spans.map((s) => s.spanContext().spanId));
  const orphans = spans.filter((s) => s.parentSpanId && !spanIds.has(s.parentSpanId));
  if (orphans.length > 0) {
    const names = orphans.map((s) => s.name).join(', ');
    throw new Error(`Orphan spans detected (parent not found): ${names}`);
  }
}
```

### Internal span count

Assert that no trace contains more than 10 `INTERNAL` spans from a single service.
A failure signals over-instrumentation — replace fine-grained spans with batch spans or log records.

```typescript
function assertInternalSpanLimit(maxPerTrace = 10) {
  const spans = getSpans();
  const counts = new Map<string, number>();
  for (const s of spans) {
    if (s.kind === SpanKind.INTERNAL) {
      const traceId = s.spanContext().traceId;
      counts.set(traceId, (counts.get(traceId) ?? 0) + 1);
    }
  }
  for (const [traceId, count] of counts) {
    if (count > maxPerTrace) {
      throw new Error(
        `Trace ${traceId} has ${count} INTERNAL spans (limit: ${maxPerTrace})`,
      );
    }
  }
}
```

### Short-duration span count

Assert that no trace contains more than 20 spans with a duration under 5 milliseconds.
A failure indicates spans created in tight loops — replace with a single batch span.

```typescript
function assertShortDurationSpanLimit(maxPerTrace = 20, thresholdMs = 5) {
  const spans = getSpans();
  const counts = new Map<string, number>();
  for (const s of spans) {
    const durationMs = (s.endTime[0] - s.startTime[0]) * 1000 + (s.endTime[1] - s.startTime[1]) / 1e6;
    if (durationMs < thresholdMs) {
      const traceId = s.spanContext().traceId;
      counts.set(traceId, (counts.get(traceId) ?? 0) + 1);
    }
  }
  for (const [traceId, count] of counts) {
    if (count > maxPerTrace) {
      throw new Error(
        `Trace ${traceId} has ${count} spans under ${thresholdMs} ms (limit: ${maxPerTrace})`,
      );
    }
  }
}
```

### ERROR status has a message

Assert that every span with status `ERROR` includes a non-empty status message.
A failure means an error was recorded without enough context to diagnose the failure from the trace alone.

```typescript
function assertErrorSpansHaveMessages() {
  const missing = getSpans().filter(
    (s) => s.status.code === 2 /* ERROR */ && !s.status.message?.trim(),
  );
  if (missing.length > 0) {
    const names = missing.map((s) => s.name).join(', ');
    throw new Error(`ERROR spans without a status message: ${names}`);
  }
}
```

### Running the assertions

Call these assertions after each integration test that exercises a code path producing traces.
If any assertion fails, investigate the code structure to find the most appropriate place to fix the instrumentation.

```typescript
afterEach(() => {
  assertNoParentlessOutboundSpans();
  assertNoOrphanSpans();
  assertInternalSpanLimit();
  assertShortDurationSpanLimit();
  assertErrorSpansHaveMessages();
  resetSpans();
});
```

## References

- [Instrumentation Score Specification](https://github.com/instrumentation-score/spec) — vendor-neutral scoring for OpenTelemetry instrumentation quality
- [Semantic Conventions: Traces](https://opentelemetry.io/docs/specs/semconv/general/trace/) — general span conventions
- [HTTP Spans](https://opentelemetry.io/docs/specs/semconv/http/http-spans/) — HTTP-specific span and status rules
- [Dash0 Semantic Conventions Explainer](https://www.dash0.com/knowledge/otel-semantic-conventions-explainer) — comprehensive guide

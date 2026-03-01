---
title: "Spans: Naming, Kind, and Status"
impact: HIGH
tags:
  - spans
  - naming
  - span-kind
  - status-code
  - http
  - messaging
---

# Spans

Every span requires three decisions: what to **name** it, which **kind** to assign, and when to set its **status** to error. These decisions are tightly coupled — you cannot name a span correctly without knowing its kind, and the status code rules depend on the kind. Getting these right is essential for service maps, operation dashboards, and error tracking.

## Span Naming

Span names MUST be low-cardinality. The number of unique span names in a system must be bounded and small.

### General Pattern: `{verb} {object}`

| Anti-Pattern (high cardinality) | Correct (low cardinality) | Fix |
|---|---|---|
| `GET /api/users/12345` | `GET /api/users/:id` | Use route template, not actual path |
| `SELECT * FROM orders WHERE id=99` | `SELECT orders` | Use table name, not full query |
| `process_payment_for_user_jane` | `process payment` | User identity is an attribute |
| `send_invoice_#98765` | `send invoice` | Invoice number is an attribute |
| `validation_failed` | `validate user_input` | Name the operation, not the outcome |

### Per-Signal Naming

| Signal | Format | Example |
|---|---|---|
| **HTTP server** | `{method} {http.route}` | `GET /api/users/:id` |
| **HTTP client** | `{method} {url.template}` or `{method}` | `POST /checkout` |
| **Database** | `{db.operation.name} {db.collection.name}` | `SELECT orders` |
| **RPC** | `{rpc.service}/{rpc.method}` | `UserService/GetUser` |
| **Messaging** | `{operation} {destination}` | `publish shop.orders` |

- HTTP: Never use the raw URI path as the span name. Use `http.route` (server) or `url.template` (client). If unavailable, use just the method.
- Database: Fall back through `db.query.summary` > `{operation} {collection}` > `{collection}` > `{db.system.name}`.
- If the method is unknown and normalized to `_OTHER`, use the protocol name alone (e.g., `HTTP`).

## Span Kind

Each span has exactly one kind. Choose based on the communication pattern, not the technology.

| Kind | Use When | Examples |
|---|---|---|
| `SERVER` | Handling an inbound synchronous request | Incoming HTTP request, incoming gRPC call |
| `CLIENT` | Making an outbound synchronous request | HTTP call, database query, outbound RPC |
| `PRODUCER` | Initiating an asynchronous operation | Publishing a message to a queue or topic |
| `CONSUMER` | Processing an asynchronous operation | Processing a message from a queue |
| `INTERNAL` | Internal operation with no remote parent/child | In-memory computation, internal function call |

### Common Mistakes

- **Using `INTERNAL` for everything.** Database calls are `CLIENT`. HTTP handlers are `SERVER`. Only use `INTERNAL` for operations that genuinely have no remote counterpart.
- **Using `CLIENT` for message publishing.** Publishing to a queue is asynchronous — use `PRODUCER`. `CLIENT` implies the caller waits for a response.
- **Using `SERVER` for message processing.** Processing a queued message is `CONSUMER`, not `SERVER`, because the producer isn't waiting.

### Messaging Kind Mapping

| Operation | Span Kind |
|---|---|
| `create` | `PRODUCER` |
| `send` | `PRODUCER` (or `CLIENT` if waiting for ack) |
| `receive` | `CLIENT` |
| `process` | `CONSUMER` |
| `settle` | `CLIENT` |

## Span Status Code

Leave span status `Unset` by default. Only set it to `Error` when the operation genuinely failed.

### HTTP Status Code Mapping

The rules differ by span kind — this is the most commonly misunderstood convention:

#### Client Spans (`SpanKind.CLIENT`)

| HTTP Status | Span Status | Rationale |
|---|---|---|
| 1xx, 2xx, 3xx | `Unset` | Request succeeded |
| **4xx** | **`Error`** | Client's request failed |
| **5xx** | **`Error`** | Server error = client failure |
| No response | **`Error`** | Connection/timeout failure |

#### Server Spans (`SpanKind.SERVER`)

| HTTP Status | Span Status | Rationale |
|---|---|---|
| 1xx, 2xx, 3xx | `Unset` | Request handled successfully |
| **4xx** | **`Unset`** | Server responded correctly to a bad request |
| **5xx** | **`Error`** | Server failed to handle the request |
| No response | **`Error`** | Server-side failure |

The critical distinction: **a 400 Bad Request on a server span is NOT an error** — the server did its job. The same 400 on the corresponding client span IS an error — the client's request failed.

### Rules

- Do NOT record errors that were retried and ultimately succeeded, or errors that were intentionally handled.

## References

- [Semantic Conventions: Traces](https://opentelemetry.io/docs/specs/semconv/general/trace/) — general span conventions
- [HTTP Spans](https://opentelemetry.io/docs/specs/semconv/http/http-spans/) — HTTP-specific span and status rules
- [Dash0 Semantic Conventions Explainer](https://www.dash0.com/knowledge/otel-semantic-conventions-explainer) — comprehensive guide

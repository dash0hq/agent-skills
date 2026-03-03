---
title: "Dash0 Derived Attributes and Features"
impact: HIGH
tags:
  - dash0
  - derived-attributes
  - service-mapping
  - operations
---

# Dash0

To get full value from Dash0, ensure your telemetry sets the attributes listed in the table below.
Dash0 derives enriched attributes from incoming telemetry — if the source attributes are missing or incorrect, the features degrade silently without errors.

## Derived capabilities

| Capability | Depends On | What Happens |
|---|---|---|
| **Service mapping and topology** | `service.name`, correct span kinds, `peer.service` or `server.address` on client spans | Builds the service dependency graph and call flows |
| **Operation classification** | `http.request.method`, `http.route`, `db.operation.name`, `rpc.method` | Builds `dash0.operation.name` for grouping requests by operation |
| **Span type detection** | Presence of namespace attributes (`http.*`, `db.*`, `messaging.*`, `rpc.*`) | Classifies spans as HTTP, database, messaging, or RPC for type-specific views |
| **Adaptive sampling** | Consistent attribute naming across services | Makes intelligent sampling decisions based on attribute patterns |
| **AI-powered log analysis** | Structured severity, log body conventions | Generates log templates and groups similar log entries |

## Minimum attributes to set

At a minimum, ensure every service sets the following.
Without them, Dash0 features degrade silently — no errors, just missing data.

| What to set | Why | Consequence if missing |
|---|---|---|
| `service.name` | Service attribution | Telemetry appears as `unknown_service` |
| Correct span kind (`SERVER`, `CLIENT`, etc.) | Service map edges | Database calls with `INTERNAL` kind do not appear as dependencies |
| Protocol attributes (`http.*`, `db.*`, `messaging.*`, `rpc.*`) | Span type classification | Spans are classified as generic, losing type-specific dashboards |
| Consistent attribute names across all services | Cross-service queries | Topology views show fragmented services; queries return partial results |

## Dash0 derived attributes

These attributes are automatically derived by Dash0 from incoming telemetry. They are not set by instrumentation — Dash0 computes them at ingestion time.

The `otel.*` attributes below are Dash0's attribute representations of OpenTelemetry protocol fields that are not natively attributes (e.g., trace ID, span duration, severity number). Dash0 surfaces them as queryable attributes for filtering and analysis.

### Resource attributes

| Attribute | Type | Description |
|---|---|---|
| `dash0.resource.id` | string | Unique identifier for the resource in Dash0 |
| `dash0.resource.type` | string | Type classification (e.g., `k8s.pod`, `host`, `vercel.project`) |
| `dash0.resource.name` | string | Human-readable name for the resource |

### Span attributes

| Attribute | Type | Description |
|---|---|---|
| `dash0.span.name` | string | Dash0-normalized span name (e.g., `SELECT … FROM my-table WHERE …`) |
| `dash0.span.type` | string | Span type classification: `http`, `rpc`, `database`, `messaging` |
| `dash0.operation.name` | string | Logical operation name (e.g., `GET /articles/<article-id>`) |
| `dash0.operation.type` | string | Operation type: `http`, `rpc`, `database`, `messaging` |
| `otel.trace.id` | string | Trace identifier (from OTel protocol trace ID field) |
| `otel.span.id` | string | Span identifier (from OTel protocol span ID field) |
| `otel.parent.id` | string | Parent span identifier (from OTel protocol parent span ID field) |
| `otel.span.name` | string | Original OpenTelemetry span name (from OTel protocol span name field) |
| `otel.span.kind` | string | Span kind: `SERVER`, `CLIENT`, `PRODUCER`, `CONSUMER`, `INTERNAL` (from OTel protocol span kind field) |
| `otel.span.duration` | double | Span duration in seconds (computed from OTel protocol start/end time fields) |
| `otel.span.start_time` | int | Span start time in Unix nanoseconds (from OTel protocol start time field) |
| `otel.span.end_time` | int | Span end time in Unix nanoseconds (from OTel protocol end time field) |
| `otel.span.status.code` | string | Span status code: `OK`, `ERROR` (from OTel protocol status field) |
| `otel.span.status.message` | string | Span status description (from OTel protocol status field) |

### Log attributes

| Attribute | Type | Description |
|---|---|---|
| `dash0.log.processor.type` | string | Log processor type (e.g., `json`) |
| `dash0.log.template` | string | AI-inferred log template with variables replaced by placeholders |
| `dash0.log.attribute.<key>` | varies | Log attributes automatically extracted through log AI |
| `otel.log.body` | string | Log record body (from OTel protocol log body field) |
| `otel.log.time` | string | Log record timestamp (from OTel protocol time field) |
| `otel.log.severity.number` | int | Severity as number, e.g., `17` for ERROR (from OTel protocol severity number field) |
| `otel.log.severity.text` | string | Severity as text, e.g., `ERROR` (from OTel protocol severity text field) |
| `otel.log.severity.range` | string | Categorical severity range: `ERROR`, `WARN`, `INFO`, `DEBUG`, `TRACE` |

### Metric attributes

| Attribute | Type | Description |
|---|---|---|
| `otel.metric.name` | string | Metric name (from OTel protocol metric name field) |
| `otel.metric.description` | string | Metric description (from OTel protocol metric description field) |
| `otel.metric.type` | string | Metric type: `GAUGE`, `HISTOGRAM`, `SUM`, etc. (from OTel protocol metric type) |
| `otel.metric.unit` | string | Metric unit, e.g., `s`, `bytes`, `1` (from OTel protocol metric unit field) |

## References

- [Dash0 Semantic Conventions](https://www.dash0.com/documentation/dash0/semantic-conventions) — full list of derived attributes
- [Dash0 Semantic Conventions Explainer](https://www.dash0.com/knowledge/otel-semantic-conventions-explainer) — comprehensive guide to conventions

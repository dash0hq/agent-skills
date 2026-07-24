---
name: otel-semantic-conventions
description: OpenTelemetry Semantic Conventions expert. Use when selecting, applying, or reviewing telemetry attributes. Triggers on tasks involving attribute selection, semantic convention compliance, attribute migration, or custom attribute decisions. Covers the attribute registry, naming patterns, attribute placement, and versioning. For span names, span kinds, and span status codes, see the otel-instrumentation skill.
metadata:
  author: dash0
  version: '1.0.0'
---

# OpenTelemetry Semantic Conventions

This skill governs correct selection, placement, and validation of telemetry attributes and metric instruments according to the OpenTelemetry Semantic Conventions specification.
For span naming, span kinds, and span status codes, see the [otel-instrumentation](../otel-instrumentation/) skill.

The [Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/) is the single source of truth for all defined attributes.

## Rules

| Rule                                | Description                                                           | Use Case                                                        |
|-------------------------------------|-----------------------------------------------------------------------|-----------------------------------------------------------------|
| [attributes](./rules/attributes.md) | Attribute registry, selection, placement, common attributes by domain | Choosing or reviewing attributes; HTTP/DB/messaging/RPC attributes; attribute placement (resource vs span) |
| [versioning](./rules/versioning.md) | Semconv versioning, stability, migration                              | Semconv version migration                                       |
| [dash0](./rules/dash0.md)           | Dash0 derived attributes and feature dependencies                     | Dash0 derived attributes                                        |

## Official documentation

- [Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/)
- [Semantic Conventions Specification](https://opentelemetry.io/docs/specs/semconv/)
- [Semantic Conventions Repository](https://github.com/open-telemetry/semantic-conventions)
- [Dash0 Semantic Conventions](https://www.dash0.com/documentation/dash0/semantic-conventions)
- [Dash0 Semantic Conventions Explainer](https://www.dash0.com/knowledge/otel-semantic-conventions-explainer)

## How to select the right attribute

1. **Search the registry first** — Look up the concept in the [Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/). Use the standard name if it exists (e.g., prefer `http.request.method` over a custom `custom.http.verb`). Custom names fragment querying and break tooling — only create a custom attribute when no registry entry covers the concept.
2. **Check stability** — Prefer `stable` attributes; note any `experimental` attributes that may change. See [versioning](./rules/versioning.md).
3. **Place at the correct level** — Resource attributes describe the entity producing telemetry; span/log attributes describe the individual operation. Do not duplicate across levels. Once an attribute is at a given level, keep it there consistently across all services.
4. **Verify cardinality** — Metric attribute values must be low-cardinality (bounded set). Variable data (user IDs, request paths with parameters) belongs in span attributes, not metric attributes.
5. **Custom attribute as last resort** — Only create a custom attribute if no registry entry covers the concept. Document the decision and follow the `org.namespace.attribute_name` naming pattern.

### Example: correct vs incorrect attribute selection

<!-- eval:skip -->
```
# Correct — uses registry attribute for HTTP method
span.set_attribute("http.request.method", "GET")

# Incorrect — invents a custom attribute for a concept already in the registry
span.set_attribute("custom.http.verb", "GET")
```

### Example: resource vs span attribute placement

<!-- eval:skip -->
```
# Correct — service identity is a resource attribute
resource = Resource({"service.name": "checkout-service", "service.version": "2.1.0"})

# Correct — operation-specific data is a span attribute
span.set_attribute("http.request.method", "POST")
span.set_attribute("http.response.status_code", 201)

# Incorrect — placing a resource-level attribute on every span
span.set_attribute("service.name", "checkout-service")  # belongs on the resource
```

### Example: cardinality violation in metric attributes

<!-- eval:skip -->
```
# Correct — metric attribute uses a bounded, low-cardinality value
histogram.record(duration_ms, {"http.request.method": "GET", "http.response.status_code": 200})

# Incorrect — unbounded values as metric attributes explode storage and query cost
histogram.record(duration_ms, {"user.id": "u-839201", "url.path": "/orders/839201"})
# Fix: move high-cardinality values to span attributes instead
span.set_attribute("user.id", "u-839201")
span.set_attribute("url.path", "/orders/839201")
```

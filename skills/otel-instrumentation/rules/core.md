---
title: "Core Setup and Quality"
impact: CRITICAL
tags:
  - setup
  - configuration
  - quality
  - cardinality
---

# Core Setup and Quality

## Essential Environment Variables

```bash
export OTEL_SERVICE_NAME="my-service"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer ${DASH0_AUTH_TOKEN}"
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.0.0,deployment.environment.name=production"
```

---

## Resource Attributes

| Attribute | Required | Example |
|-----------|----------|---------|
| `service.name` | **Yes** | `payment-service` |
| `service.version` | Recommended | `2.1.0` |
| `deployment.environment.name` | Recommended | `production` |
| `service.namespace` | Optional | `checkout-team` |

---

## Sampling

```bash
export OTEL_TRACES_SAMPLER="parentbased_traceidratio"
export OTEL_TRACES_SAMPLER_ARG="0.1"  # 10%
```

| Sampler | Use Case |
|---------|----------|
| `always_on` | Development |
| `parentbased_traceidratio` | Production (recommended) |

---

## Protocol Selection

| Protocol | Port | When |
|----------|------|------|
| gRPC | 4317 | Default, best performance |
| HTTP/protobuf | 4318 | Firewalls blocking gRPC |

---

## Quality Hierarchy

| Priority | Focus | Question |
|----------|-------|----------|
| 1 | Correctness | Is the data accurate? |
| 2 | Actionability | Will this help diagnose issues? |
| 3 | Efficiency | Is cardinality bounded? |
| 4 | Cost | Can we afford this at scale? |

**Golden rule**: If you never look at it, remove it.

---

## Cardinality Check

Before deploying new instrumentation:

```
method:      5 values
route:       50 values (normalized)
status:      5 values (bucketed)
instances:   10

Total: 5 × 50 × 5 × 10 = 12,500 series ✓
```

### Budget Zones

| Series | Zone | Action |
|--------|------|--------|
| < 10K | Ideal | Proceed |
| 10K-100K | Caution | Review attributes |
| > 100K | Danger | Remove unbounded attributes |

### Unbounded = Dangerous

Never use on metrics:
- `user.id`, `request.id`, `order.id`
- `url.full` (has query params)
- `timestamp`, `ip.address`

---

## Attribute Placement

| Signal | Cardinality | OK | Avoid |
|--------|-------------|----|----|
| Metrics | Very low | method, route, status_bucket | user.id |
| Spans | Medium | + user.id, order.id | request.body |
| Logs | Higher | + request.id | secrets |

---

## Normalization Patterns

### URLs

```javascript
path.replace(/\/\d+/g, "/{id}")  // /users/123 → /users/{id}
```

### Status Codes

```javascript
if (code >= 200 && code < 300) return "2xx";
if (code >= 400 && code < 500) return "4xx";
if (code >= 500) return "5xx";
```

---

## Anti-Patterns

```javascript
// BAD: metric per entity
meter.createCounter(`orders.${orderId}.processed`);

// GOOD: use attributes
ordersProcessed.add(1, { order_type: order.type });
```

```javascript
// BAD: span per iteration
for (const item of items) {
  tracer.startActiveSpan("process.item", span => { ... });
}

// GOOD: single span
tracer.startActiveSpan("process.batch", span => {
  span.setAttribute("batch.size", items.length);
});
```

---

## Troubleshooting

| Issue | Fix |
|-------|-----|
| No data | Check `OTEL_SERVICE_NAME` and endpoint |
| 401 errors | Verify `Authorization=Bearer token` format |
| High costs | Review cardinality, add sampling |

## Resources

- [OTel SDK Configuration](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

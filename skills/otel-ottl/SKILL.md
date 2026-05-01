---
name: otel-ottl
description: OpenTelemetry Transformation Language (OTTL) expert. Use when writing or debugging OTTL expressions for any OpenTelemetry Collector component that supports OTTL (processors, connectors, receivers, exporters). Triggers on tasks involving telemetry transformation, filtering, attribute manipulation, data redaction, sampling policies, routing, or Collector configuration. Covers syntax, contexts, functions, error handling, and performance.
metadata:
  author: dash0
  version: '1.0.0'
---

# OpenTelemetry Transformation Language (OTTL)

Use OTTL to transform, filter, and manipulate telemetry data inside the OpenTelemetry Collector — without changing application code.

## Components that use OTTL

OTTL is not limited to the transform and filter processors.
Processors (transform, filter, attributes, span, tailsampling, cumulativetodelta, logdedup, lookup), connectors (routing, count, sum, signaltometrics), and the hostmetrics receiver all accept OTTL expressions.
See [components](./rules/components.md) for the full list with use cases.

## OTTL syntax

### Path expressions

Navigate telemetry data using dot notation:

```
span.name
span.attributes["http.method"]
resource.attributes["service.name"]
```

**Contexts** (first path segment) map to OpenTelemetry signal structures:
- `resource` - Resource-level attributes
- `scope` - Instrumentation scope
- `span` - Span data (traces)
- `spanevent` - Span events
- `metric` - Metric metadata
- `datapoint` - Metric data points
- `log` - Log records

### Enumerations

Use int64 constants for enumeration fields:

```
span.status.code == STATUS_CODE_ERROR
span.kind == SPAN_KIND_SERVER
```

### Operators

| Category | Operators |
|----------|-----------|
| Assignment | `=` |
| Comparison | `==`, `!=`, `>`, `<`, `>=`, `<=` |
| Logical | `and`, `or`, `not` |

### Functions

**Converters** (uppercase, return values):

```
ToUpperCase(span.attributes["http.request.method"])
Substring(log.body.string, 0, 1024)
Concat(["prefix", span.attributes["request.id"]], "-")
IsMatch(metric.name, "^k8s\\..*$")
```

**Editors** (lowercase, modify data in-place):

```
set(span.attributes["region"], "us-east-1")
delete_key(resource.attributes, "internal.key")
limit(log.attributes, 10, [])
```

### Conditional statements

Use `where` to apply transformations conditionally:

```
span.attributes["db.statement"] = "REDACTED" where resource.attributes["service.name"] == "accounting"
```

### Nil checks

Use `nil` for absence checking (not `null`):

```
resource.attributes["service.name"] != nil
```

## Validation workflow

1. **Validate config syntax** — run `otelcol validate --config=config.yaml` to catch compilation errors before starting the Collector.
2. **Test with the debug exporter** — route transformed telemetry to a `debug` exporter and inspect the output:

```yaml
exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [transform, batch]
      exporters: [debug]   # swap in production exporter once validated
```

3. **Set `error_mode: ignore` in production** — see [Error handling](#error-handling).
4. **Promote to production exporters** — replace `debug` with the production exporter.

## Common patterns

### Set attributes

```
set(resource.attributes["k8s.cluster.name"], "prod-aws-us-west-2")
```

### Redact sensitive data

Guard with a `nil` check to avoid creating the attribute when it does not exist.

| Strategy | Function | When to use |
|----------|----------|-------------|
| Replace with placeholder | `set(target, "REDACTED")` | Known sensitive attributes (auth headers, cookies) |
| Mask partial value | `replace_pattern(target, regex, replacement)` | Preserve structure while hiding detail (credit card numbers, IPs) |
| Hash | `SHA256(target)` | Remove raw value but keep a correlatable identifier (emails, user IDs) |
| Delete | `delete_key(map, key)` | Attribute should never leave the Collector |
| Drop record | Filter processor | Entire record is sensitive (e.g., contains private keys) |

```yaml
processors:
  transform/redact:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          # Replace — auth and session headers
          - set(span.attributes["http.request.header.authorization"], "REDACTED") where span.attributes["http.request.header.authorization"] != nil
          - set(span.attributes["http.request.header.cookie"], "REDACTED") where span.attributes["http.request.header.cookie"] != nil
          # Hash — emails (preserves correlation)
          - set(span.attributes["user.email"], SHA256(span.attributes["user.email"])) where span.attributes["user.email"] != nil
          # Delete — attributes that must never be exported
          - delete_key(span.attributes, "credit-card.number")
    log_statements:
      - context: log
        statements:
          # Mask — credit card numbers (keep first/last 4 digits)
          - replace_pattern(log.body["string"], "\\b(\\d{4})\\d{5,11}(\\d{4})\\b", "$$1****$$2")
  filter/drop-sensitive-logs:
    error_mode: ignore
    logs:
      log_record:
        - 'IsMatch(log.body["string"], "(?i)-----BEGIN (RSA |EC )?PRIVATE KEY-----")'
```

Place redaction processors **after** enrichment processors (`resourcedetection`, `k8sattributes`, `resource`) and **before** exporters.
See [processor ordering](../otel-collector/rules/processors.md#processor-ordering) for the full ordering guidance.

See the [sensitive data](../otel-instrumentation/rules/sensitive-data.md) rule for application-level sanitization.

### Drop telemetry by pattern

```
IsMatch(metric.name, "^k8s\\.replicaset\\..*$")
```

### Drop stale data

```
time_unix_nano < UnixNano(Now()) - 21600000000000
```

### Backfill missing timestamps

```yaml
processors:
  transform:
    log_statements:
      - context: log
        statements:
          - set(log.observed_time, Now()) where log.observed_time_unix_nano == 0
          - set(log.time, log.observed_time) where log.time_unix_nano == 0
```

### Filter processor example

```yaml
processors:
  filter:
    metrics:
      datapoint:
        - 'IsMatch(ConvertCase(String(metric.name), "lower"), "^k8s\\.replicaset\\.")'

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [filter, batch]
      exporters: [debug]
```

### Transform processor example

```yaml
processors:
  transform:
    trace_statements:
      - context: span
        statements:
          - set(span.status.code, STATUS_CODE_ERROR) where span.attributes["http.response.status_code"] >= 500
          - set(span.attributes["env"], "production") where resource.attributes["deployment.environment"] == "prod"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [transform, batch]
      exporters: [debug]
```

### Defensive nil checks

```
resource.attributes["service.namespace"] != nil
and
IsMatch(ConvertCase(String(resource.attributes["service.namespace"]), "lower"), "^platform.*$")
```

### Normalize high-cardinality attributes

#### Replace dynamic path segments

Replace numeric IDs and UUIDs in `url.path` and `http.route` with fixed placeholders.

```yaml
processors:
  transform/normalize-paths:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - replace_pattern(span.attributes["url.path"], "/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", "/{uuid}") where span.attributes["url.path"] != nil
          - replace_pattern(span.attributes["url.path"], "/\\d+", "/{id}") where span.attributes["url.path"] != nil
          - replace_pattern(span.attributes["http.route"], "/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", "/{uuid}") where span.attributes["http.route"] != nil
          - replace_pattern(span.attributes["http.route"], "/\\d+", "/{id}") where span.attributes["http.route"] != nil
```

#### Mask IP addresses to subnet

```yaml
processors:
  transform/mask-ips:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - replace_pattern(span.attributes["client.address"], "(\\d{1,3}\\.\\d{1,3}\\.\\d{1,3})\\.\\d{1,3}", "$$1.0") where span.attributes["client.address"] != nil
    log_statements:
      - context: log
        statements:
          - replace_pattern(log.attributes["client.address"], "(\\d{1,3}\\.\\d{1,3}\\.\\d{1,3})\\.\\d{1,3}", "$$1.0") where log.attributes["client.address"] != nil
```

#### Limit attribute count and value length

```yaml
processors:
  transform/limit-attributes:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - limit(span.attributes, 64, [])
          - truncate_all(span.attributes, 256)
    log_statements:
      - context: log
        statements:
          - limit(log.attributes, 64, [])
          - truncate_all(log.attributes, 256)
```

### Enrich telemetry with static attributes

Use the `resource` processor (not `transform`) for static resource attributes.

```yaml
processors:
  resource/static-env:
    attributes:
      - key: deployment.environment.name
        value: production
        action: upsert
      - key: k8s.cluster.name
        value: prod-us-west-2
        action: upsert
```

To copy a resource attribute down to the span or log level, use the transform processor:

```yaml
processors:
  transform/copy-resource:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - set(span.attributes["deployment.environment.name"], resource.attributes["deployment.environment.name"]) where resource.attributes["deployment.environment.name"] != nil
```

## Error handling

### Compilation errors

Occur during processor initialization and prevent Collector startup:
- Invalid syntax (missing quotes)
- Unknown functions
- Invalid path expressions
- Type mismatches

### Runtime errors

Occur during telemetry processing:
- Accessing non-existent attributes
- Type conversion failures
- Function execution errors

### Error mode configuration

Always set `error_mode` explicitly.

| Mode | Behavior | When to use |
|------|----------|-------------|
| `propagate` (default) | Stops processing current item | Development and strict environments where you want to catch every error |
| `ignore` | Logs error, continues processing | **Production** — set this unless you have a specific reason not to |
| `silent` | Ignores errors without logging | High-volume pipelines with known-safe transforms where error logs are noise |

```yaml
processors:
  transform:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - set(span.attributes["parsed"], ParseJSON(span.attributes["json_body"]))
```

## Performance

Use `where` clauses to skip items early rather than applying unconditional transforms.

## Function reference

See [function-reference](./rules/function-reference.md) for the full list of editors and converters.

**Editors** (lowercase, modify data in-place): `set`, `delete_key`, `delete_matching_keys`, `keep_keys`, `replace_pattern`, `replace_match`, `merge_maps`, `limit`, `truncate_all`, `flatten`, `append`.

**Converters** (uppercase, return values): `IsMatch`, `Concat`, `Substring`, `ConvertCase`, `SHA256`, `ParseJSON`, `ExtractPatterns`, `Now`, `UnixNano`, `Len`, `String`, `Int`, `Double`, `Bool`.

## References

- [OTTL Guide](https://www.dash0.com/guides/opentelemetry-transformation-language-ottl)
- [OTTL Specification](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl)
- [OTTL Functions Reference](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/ottlfuncs)
- [OTTL Playground](https://ottl.run)

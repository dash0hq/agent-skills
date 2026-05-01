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
The following Collector components accept OTTL expressions in their configuration.

### Processors

| Component | Use case |
|-----------|----------|
| [transform](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor) | Modify, enrich, or redact telemetry (set attributes, rename fields, truncate values) |
| [filter](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor) | Drop telemetry entirely (discard metrics by name, drop spans by status, remove noisy logs) |
| [attributes](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/attributesprocessor) | Insert, update, delete, or hash resource and record attributes |
| [span](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/spanprocessor) | Rename spans and set span status based on attribute values |
| [tailsampling](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/tailsamplingprocessor) | Sample traces based on OTTL conditions (e.g., keep error traces, drop health checks) |
| [cumulativetodelta](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/cumulativetodeltaprocessor) | Convert cumulative metrics to delta temporality with OTTL-based metric selection |
| [logdedup](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/logdedupprocessor) | Deduplicate log records using OTTL conditions |
| [lookup](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/lookupprocessor) | Enrich telemetry by looking up values from external tables using OTTL expressions |

### Connectors

| Component | Use case |
|-----------|----------|
| [routing](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/routingconnector) | Route telemetry to different pipelines based on OTTL conditions |
| [count](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/countconnector) | Count spans, metrics, or logs matching OTTL conditions and emit as metrics |
| [sum](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/sumconnector) | Sum numeric values from telemetry matching OTTL conditions and emit as metrics |
| [signaltometrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/signaltometricsconnector) | Generate metrics from spans or logs using OTTL expressions for attribute extraction |
### Receivers

| Component | Use case |
|-----------|----------|
| [hostmetrics](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/hostmetricsreceiver) | Filter host metrics at collection time using OTTL conditions |

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

Several fields accept int64 values exposed as global constants:

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

**Converters** (uppercase, pure functions that return values):

```
ToUpperCase(span.attributes["http.request.method"])
Substring(log.body.string, 0, 1024)
Concat(["prefix", span.attributes["request.id"]], "-")
IsMatch(metric.name, "^k8s\\..*$")
```

**Editors** (lowercase, functions with side-effects that modify data):

```
set(span.attributes["region"], "us-east-1")
delete_key(resource.attributes, "internal.key")
limit(log.attributes, 10, [])
```

### Conditional statements

The `where` clause applies transformations conditionally:

```
span.attributes["db.statement"] = "REDACTED" where resource.attributes["service.name"] == "accounting"
```

### Nil checks

OTTL uses `nil` for absence checking (not `null`):

```
resource.attributes["service.name"] != nil
```

## Common patterns

### Set attributes

```
set(resource.attributes["k8s.cluster.name"], "prod-aws-us-west-2")
```

### Redact sensitive data

Always guard with a `nil` check to avoid creating the attribute when it does not exist.
Choose the redaction strategy based on the use case:

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

Application-level sanitization is the first line of defence; use Collector-side redaction as a safety net.
See the [sensitive data](../otel-instrumentation/rules/sensitive-data.md) rule in the `otel-instrumentation` skill for application-level guidance.

### Drop telemetry by pattern

In a filter processor, matching expressions cause data to be dropped:

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

Always check for `nil` before operating on optional attributes:

```
resource.attributes["service.namespace"] != nil
and
IsMatch(ConvertCase(String(resource.attributes["service.namespace"]), "lower"), "^platform.*$")
```

### Normalize high-cardinality attributes

High-cardinality attributes — URL paths with embedded IDs, long freeform strings, or unbounded attribute maps — inflate storage costs and degrade query performance.
Use OTTL in a transform processor to normalize these attributes before export.

#### Replace dynamic path segments

Replace numeric IDs and UUIDs in `url.path` and `http.route` with fixed placeholders to collapse cardinality.

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

Replace the last octet of IPv4 addresses with `0` to reduce cardinality while preserving subnet-level information.

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

Use `limit` and `truncate_all` to enforce bounds on attribute maps that may grow unboundedly.

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

When `resourcedetection` or `k8sattributes` processors are not available — for example, in non-Kubernetes deployments or when the Collector runs outside the cluster — set resource attributes explicitly.

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

Use the `resource` processor (not the `transform` processor) for static resource attributes.
The `resource` processor operates at the resource level directly, while `transform` requires a `resource` context and explicit path expressions.

To copy a resource attribute down to the span or log level — for example, when the backend does not propagate resource context — use the transform processor:

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
The default (`propagate`) stops processing the current item on any error, which can silently drop telemetry in production.

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

OTTL statements compile once at startup and execute as optimized function chains at runtime.
There is no need to optimize for compilation speed — focus on reducing the number of statements that evaluate per telemetry item.
Use `where` clauses to skip items early rather than applying unconditional transforms.

## Validation

Validate OTTL statements **before** deploying them to the Collector.
A syntax error in a statement causes the Collector to reject the entire configuration at startup.

### Validation workflow

1. **Write statements** — draft OTTL expressions following the patterns in this skill.
2. **Test in the playground** — paste the full processor YAML into [ottl.run](https://ottl.run), supply sample telemetry as JSON, and verify the output matches expectations.
3. **Validate the full config** — run `otelcol validate --config=config.yaml` to catch wiring errors (undeclared components, missing pipelines).
4. **Smoke-test with the debug exporter** — deploy with a `debug` exporter in the pipeline, send representative telemetry, and inspect stdout to confirm transforms apply correctly.

Use [ottl.run](https://ottl.run) for every non-trivial statement.
It catches errors that `otelcol validate` does not — such as runtime type mismatches, incorrect `where` clauses, and regex issues — because it executes the statements against real telemetry data rather than only checking structural validity.

### What to test in the playground

| Check | How |
|-------|-----|
| Statement compiles | Paste the YAML; the playground reports syntax errors inline |
| Correct field is modified | Compare input and output JSON side by side |
| `where` clause filters correctly | Provide one matching and one non-matching telemetry item |
| `nil` safety | Provide telemetry that is missing the target attribute |
| Regex patterns | Provide input strings that should and should not match |

## Function reference

See [function-reference](./rules/function-reference.md) for the full list of editors and converters.

**Editors** (lowercase, modify data in-place): `set`, `delete_key`, `delete_matching_keys`, `keep_keys`, `replace_pattern`, `replace_match`, `merge_maps`, `limit`, `truncate_all`, `flatten`, `append`.

**Converters** (uppercase, return values): `IsMatch`, `Concat`, `Substring`, `ConvertCase`, `SHA256`, `ParseJSON`, `ExtractPatterns`, `Now`, `UnixNano`, `Len`, `String`, `Int`, `Double`, `Bool`.

## References

- [OTTL Guide](https://www.dash0.com/guides/opentelemetry-transformation-language-ottl)
- [OTTL Specification](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl)
- [OTTL Functions Reference](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl/ottlfuncs)
- [OTTL Playground](https://ottl.run)
---
title: "OTTL recipes"
impact: HIGH
tags:
  - ottl
  - recipes
  - pci
  - multi-tenant
  - performance
---

# OTTL recipes

End-to-end Collector configurations that solve common production problems using OTTL.
Each recipe is a self-contained pipeline fragment you can add to an existing Collector configuration.

For OTTL syntax and function reference, see the [main OTTL guide](../SKILL.md).

## PCI compliance pipeline

Payment-processing services must not store cardholder data in telemetry.
This 4-stage pipeline strips PAN, CVV, and related attributes from both traces and logs.

```yaml
processors:
  # Stage 1: Delete known sensitive attribute keys
  transform/pci-delete:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - delete_matching_keys(span.attributes, "(?i).*(card_number|pan|cvv|pin|card\\.number).*")
    log_statements:
      - context: log
        statements:
          - delete_matching_keys(log.attributes, "(?i).*(card_number|pan|cvv|pin|card\\.number).*")

  # Stage 2: Mask card numbers embedded in log bodies (keep first 6 and last 4)
  transform/pci-mask-bodies:
    error_mode: ignore
    log_statements:
      - context: log
        statements:
          - replace_pattern(log.body["string"], "\\b(\\d{4})[- ]?\\d{4}[- ]?\\d{4}[- ]?(\\d{4})\\b", "\\1-****-****-\\2")

  # Stage 3: Mask card numbers in span attributes (e.g., db.query.text)
  transform/pci-mask-spans:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - replace_pattern(span.attributes["db.query.text"], "\\b\\d{4}[- ]?\\d{4}[- ]?\\d{4}[- ]?\\d{4}\\b", "****-****-****-****") where span.attributes["db.query.text"] != nil

  # Stage 4: Drop logs that contain private key material
  filter/pci-drop-keys:
    error_mode: ignore
    logs:
      log_record:
        - 'IsMatch(log.body["string"], "-----BEGIN (RSA |EC )?PRIVATE KEY-----")'

service:
  pipelines:
    traces:
      processors: [memory_limiter, transform/pci-delete, transform/pci-mask-spans, batch]
    logs:
      processors: [memory_limiter, transform/pci-delete, transform/pci-mask-bodies, filter/pci-drop-keys, batch]
```

Place the PCI processors after `memory_limiter` and before `batch`.
The `error_mode: ignore` setting ensures that a malformed field does not halt the pipeline — the statement is skipped and processing continues.

## Multi-tenant telemetry isolation

Route telemetry from different tenants to separate backend pipelines.
This is useful when tenants have different retention policies, export endpoints, or data-residency requirements.

```yaml
connectors:
  routing/tenant:
    default_pipelines: [traces/default]
    error_mode: ignore
    table:
      - statement: route() where resource.attributes["tenant.id"] == "acme"
        pipelines: [traces/acme]
      - statement: route() where resource.attributes["tenant.id"] == "globex"
        pipelines: [traces/globex]

exporters:
  otlp/default:
    endpoint: https://default-backend.example.com:4317
  otlp/acme:
    endpoint: https://acme-backend.example.com:4317
  otlp/globex:
    endpoint: https://globex-backend.example.com:4317

service:
  pipelines:
    traces/ingress:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [routing/tenant]
    traces/default:
      receivers: [routing/tenant]
      processors: [batch]
      exporters: [otlp/default]
    traces/acme:
      receivers: [routing/tenant]
      processors: [batch]
      exporters: [otlp/acme]
    traces/globex:
      receivers: [routing/tenant]
      processors: [batch]
      exporters: [otlp/globex]
```

The routing connector evaluates OTTL conditions on each trace and forwards it to the matching pipeline.
Telemetry that matches no condition goes to `default_pipelines`.

To add a new tenant, add a row to the `table` and a corresponding pipeline and exporter.
The application must set `tenant.id` as a resource attribute — see the [resources rule](../../otel-instrumentation/rules/resources.md) for how to configure resource attributes in the SDK.

## Multi-step transformation with cache

Some transformations require intermediate values that do not map to a single OTTL statement.
Use the `cache` map (a per-record scratch pad) to store intermediate results across statements within the same context block.

```yaml
processors:
  transform/parse-user-agent:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          # Step 1: Parse user-agent header into a map
          - set(cache["ua"], UserAgent(span.attributes["http.request.header.user-agent"])) where span.attributes["http.request.header.user-agent"] != nil
          # Step 2: Extract fields from the parsed map
          - set(span.attributes["browser.name"], cache["ua"]["name"]) where cache["ua"] != nil
          - set(span.attributes["browser.version"], cache["ua"]["version"]) where cache["ua"] != nil
          - set(span.attributes["os.name"], cache["ua"]["os"]) where cache["ua"] != nil
          # Step 3: Remove the raw header (high cardinality)
          - delete_key(span.attributes, "http.request.header.user-agent") where cache["ua"] != nil
```

The `cache` map exists only for the duration of a single telemetry record.
It is not shared between records and does not persist across batches.

Another common use — parse a URL and set attributes from its components:

```yaml
processors:
  transform/parse-url:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - set(cache["url"], URL(span.attributes["url.full"])) where span.attributes["url.full"] != nil
          - set(span.attributes["url.scheme"], cache["url"]["scheme"]) where cache["url"] != nil
          - set(span.attributes["url.path"], cache["url"]["path"]) where cache["url"] != nil
          - set(span.attributes["server.address"], cache["url"]["host"]) where cache["url"] != nil
```

## Service-specific redaction

Apply stricter redaction rules to services that handle sensitive data (e.g., payment services) while keeping standard redaction for other services.

```yaml
processors:
  # Strict: remove all user/customer/payment attributes from payment services
  transform/redact-payment:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - delete_matching_keys(span.attributes, "(?i).*(user|customer|payment|card).*") where resource.attributes["service.name"] == "payment-service"

  # Standard: redact auth headers everywhere
  transform/redact-standard:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - set(span.attributes["http.request.header.authorization"], "REDACTED") where span.attributes["http.request.header.authorization"] != nil
          - set(span.attributes["http.request.header.cookie"], "REDACTED") where span.attributes["http.request.header.cookie"] != nil
          - set(span.attributes["http.response.header.set-cookie"], "REDACTED") where span.attributes["http.response.header.set-cookie"] != nil

service:
  pipelines:
    traces:
      processors: [memory_limiter, transform/redact-payment, transform/redact-standard, batch]
```

Order matters: place the stricter processor before the standard one so the payment service attributes are deleted before the standard processor tries to redact them individually.

## Performance anti-patterns

### Missing `where` guards

Every OTTL statement is evaluated for every telemetry record in the pipeline.
Statements that call expensive functions (regex, parsing, hashing) without a `where` clause waste CPU on records that do not match.

```yaml
# BAD: SHA256 runs on every span, even those without user.email
- set(span.attributes["user.email"], SHA256(span.attributes["user.email"]))

# GOOD: only hash when the attribute exists
- set(span.attributes["user.email"], SHA256(span.attributes["user.email"])) where span.attributes["user.email"] != nil
```

### Broad regex on large fields

Avoid running complex regex patterns against unbounded fields like `log.body` or `db.query.text` without first checking that the field is present and reasonably sized.

```yaml
# BAD: regex on every log body regardless of content
- replace_pattern(log.body["string"], "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Za-z]{2,}\\b", "****@****.***")

# GOOD: only apply to logs from services known to emit user-facing data
- replace_pattern(log.body["string"], "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Za-z]{2,}\\b", "****@****.***") where resource.attributes["service.name"] == "web-api"
```

### Ordering cheap checks before expensive ones

When multiple conditions apply, place cheap attribute-existence checks in the `where` clause before expensive function calls.
OTTL evaluates `where` conditions left-to-right and short-circuits on the first `false`.

```yaml
# GOOD: nil check (cheap) runs before IsMatch (regex, expensive)
- set(span.attributes["http.route"], "/{id}") where span.attributes["http.route"] != nil and IsMatch(span.attributes["http.route"], "/users/\\d+")
```

## References

- [OTTL main guide](../SKILL.md)
- [Transform processor documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor)
- [Routing connector documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/routingconnector)
- [Filter processor documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor)
- [Sensitive data — application-level redaction](../../otel-instrumentation/rules/sensitive-data.md)

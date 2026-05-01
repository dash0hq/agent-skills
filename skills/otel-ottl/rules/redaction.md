# Redact sensitive data with OTTL

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
See [processor ordering](../../otel-collector/rules/processors.md#processor-ordering) for the full ordering guidance.

See the [sensitive data](../../otel-instrumentation/rules/sensitive-data.md) rule for application-level sanitization.

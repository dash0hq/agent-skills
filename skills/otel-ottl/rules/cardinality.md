# Normalize high-cardinality attributes

## Replace dynamic path segments

Replace numeric IDs and UUIDs in `url.path` and `http.route` with fixed placeholders.

```yaml
processors:
  transform/normalize-paths:
    error_mode: ignore
    trace_statements:
      - replace_pattern(span.attributes["url.path"], "/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", "/{uuid}") where span.attributes["url.path"] != nil
      - replace_pattern(span.attributes["url.path"], "/\\d+", "/{id}") where span.attributes["url.path"] != nil
      - replace_pattern(span.attributes["http.route"], "/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", "/{uuid}") where span.attributes["http.route"] != nil
      - replace_pattern(span.attributes["http.route"], "/\\d+", "/{id}") where span.attributes["http.route"] != nil
```

## Mask IP addresses to subnet

```yaml
processors:
  transform/mask-ips:
    error_mode: ignore
    trace_statements:
      - replace_pattern(span.attributes["client.address"], "(\\d{1,3}\\.\\d{1,3}\\.\\d{1,3})\\.\\d{1,3}", "$$1.0") where span.attributes["client.address"] != nil
    # Add log_statements with the same pattern using log.attributes to apply to logs.
```

## Limit attribute count and value length

```yaml
processors:
  transform/limit-attributes:
    error_mode: ignore
    trace_statements:
      - limit(span.attributes, 64, [])
      - truncate_all(span.attributes, 256)
    # Add log_statements with the same pattern using log.attributes to apply to logs.
```

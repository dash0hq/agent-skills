# Enrich telemetry with static attributes

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
      - set(span.attributes["deployment.environment.name"], resource.attributes["deployment.environment.name"]) where resource.attributes["deployment.environment.name"] != nil
```

# Common OTTL patterns

## Set attributes

```
set(resource.attributes["k8s.cluster.name"], "prod-aws-us-west-2")
```

## Drop telemetry by pattern

```
IsMatch(metric.name, "^k8s\\.replicaset\\..*$")
```

## Drop stale data

```
time_unix_nano < UnixNano(Now()) - 21600000000000
```

## Backfill missing timestamps

```yaml
processors:
  transform:
    log_statements:
      - context: log
        statements:
          - set(log.observed_time, Now()) where log.observed_time_unix_nano == 0
          - set(log.time, log.observed_time) where log.time_unix_nano == 0
```

## Filter processor example

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

## Transform processor example

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

## Defensive nil checks

```
resource.attributes["service.namespace"] != nil
and
IsMatch(ConvertCase(String(resource.attributes["service.namespace"]), "lower"), "^platform.*$")
```

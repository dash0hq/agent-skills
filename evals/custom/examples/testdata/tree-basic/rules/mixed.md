# Mixed fixture

A valid Collector fragment.

```yaml
processors:
  batch:
    send_batch_size: 8192
```

A skipped diagram.

<!-- eval:skip -->
```
A → B → C
```

A deliberately wrong example.

```yaml
# BAD — the debug exporter must not stay in production pipelines
exporters:
  debug:
    verbosity: not-a-real-verbosity
```

SDK code pending a toolchain.

```go
package main

func main() {}
```

A bash block, exempt by default.

```bash
echo hello
```

Bare OTTL statements.

```
set(span.attributes["env"], "production")
```

A Kubernetes manifest with an embedded Collector configuration.

```yaml
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: otel
spec:
  mode: deployment
  config:
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
    exporters:
      otlp:
        endpoint: <OTLP_ENDPOINT>
        headers:
          Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
    service:
      pipelines:
        traces:
          receivers: [otlp]
          exporters: [otlp]
```

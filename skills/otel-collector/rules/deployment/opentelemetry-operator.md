---
title: "OpenTelemetry Operator"
impact: HIGH
tags:
  - deployment
  - kubernetes
  - operator
  - auto-instrumentation
  - sidecar
---

# OpenTelemetry Operator

The [OpenTelemetry Operator](https://github.com/open-telemetry/opentelemetry-operator) is a Kubernetes operator that manages Collector instances and auto-instrumentation injection via custom resources.
Use it when you need declarative Collector lifecycle management or automatic SDK injection without modifying application images.

## Installation

The operator requires [cert-manager](https://cert-manager.io/) for webhook TLS certificates.

```bash
# Install cert-manager (if not already present)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml

# Install the operator
kubectl apply -f https://github.com/open-telemetry/opentelemetry-operator/releases/latest/download/opentelemetry-operator.yaml
```

## Custom resources

The operator provides two CRDs:

| CRD | API version | Purpose |
|-----|-------------|---------|
| `OpenTelemetryCollector` | `opentelemetry.io/v1beta1` | Manages Collector instances (DaemonSet, Deployment, StatefulSet, sidecar) |
| `Instrumentation` | `opentelemetry.io/v1alpha1` | Configures auto-instrumentation injection into application pods |

## OpenTelemetryCollector

### Deployment modes

| Mode | Value | Use when |
|------|-------|----------|
| Deployment | `deployment` | Gateway — centralized processing and export |
| DaemonSet | `daemonset` | Agent — node-level collection |
| StatefulSet | `statefulset` | Gateway with persistent volumes for sending queues |
| Sidecar | `sidecar` | Per-pod collector injected via annotation |

### Agent example (DaemonSet)

```yaml
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: otel-agent
  namespace: otel
spec:
  mode: daemonset
  image: ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-k8s:0.120.0
  resources:
    limits:
      cpu: 500m
      memory: 512Mi
  env:
    - name: K8S_NODE_NAME
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
    - name: DASH0_AUTH_TOKEN
      valueFrom:
        secretKeyRef:
          name: dash0-credentials
          key: auth-token
  config:
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318
    processors:
      memory_limiter:
        check_interval: 1s
        limit_mib: 410
        spike_limit_mib: 100
    exporters:
      otlp:
        endpoint: <OTLP_ENDPOINT>
        headers:
          Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
        sending_queue:
          enabled: true
          queue_size: 5000
          storage: file_storage
    extensions:
      health_check:
        endpoint: 0.0.0.0:13133
      file_storage:
        directory: /var/lib/otelcol/queue
    service:
      extensions: [health_check, file_storage]
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter]
          exporters: [otlp]
        metrics:
          receivers: [otlp]
          processors: [memory_limiter]
          exporters: [otlp]
        logs:
          receivers: [otlp]
          processors: [memory_limiter]
          exporters: [otlp]
```

### Gateway example (Deployment)

```yaml
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: otel-gateway
  namespace: otel
spec:
  mode: deployment
  replicas: 2
  image: ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-k8s:0.120.0
  resources:
    limits:
      cpu: "2"
      memory: 2Gi
  autoscaler:
    minReplicas: 2
    maxReplicas: 10
    targetCPUUtilization: 70
  env:
    - name: DASH0_AUTH_TOKEN
      valueFrom:
        secretKeyRef:
          name: dash0-credentials
          key: auth-token
  config:
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318
    processors:
      memory_limiter:
        check_interval: 1s
        limit_mib: 1638
        spike_limit_mib: 400
      k8sattributes:
        auth_type: serviceAccount
        passthrough: false
        extract:
          metadata:
            - k8s.namespace.name
            - k8s.deployment.name
            - k8s.pod.name
            - k8s.pod.uid
            - k8s.node.name
        pod_association:
          - sources:
              - from: resource_attribute
                name: k8s.pod.uid
          - sources:
              - from: connection
      resource:
        attributes:
          - key: k8s.cluster.name
            value: "<CLUSTER_NAME>"
            action: upsert
    exporters:
      otlp:
        endpoint: <OTLP_ENDPOINT>
        headers:
          Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
        compression: gzip
        sending_queue:
          enabled: true
          num_consumers: 10
          queue_size: 5000
          storage: file_storage
    extensions:
      health_check:
        endpoint: 0.0.0.0:13133
      file_storage:
        directory: /var/lib/otelcol/queue
    service:
      extensions: [health_check, file_storage]
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter, k8sattributes, resource]
          exporters: [otlp]
        metrics:
          receivers: [otlp]
          processors: [memory_limiter, k8sattributes, resource]
          exporters: [otlp]
        logs:
          receivers: [otlp]
          processors: [memory_limiter, k8sattributes, resource]
          exporters: [otlp]
```

### Sidecar mode

In sidecar mode, the operator injects a Collector container into application pods annotated with `sidecar.opentelemetry.io/inject`.

```yaml
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: otel-sidecar
  namespace: otel
spec:
  mode: sidecar
  config:
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
    exporters:
      otlp:
        endpoint: otel-gateway.otel.svc.cluster.local:4317
        tls:
          insecure: true
    service:
      pipelines:
        traces:
          receivers: [otlp]
          exporters: [otlp]
```

Annotate the application pod template to trigger injection:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    metadata:
      annotations:
        sidecar.opentelemetry.io/inject: "otel-sidecar"
    spec:
      containers:
        - name: my-app
          image: my-app:latest
```

The annotation value can be `"true"` (uses any `OpenTelemetryCollector` CR with `mode: sidecar` in the same namespace), a specific CR name (`"otel-sidecar"`), or a namespaced reference (`"other-namespace/otel-sidecar"`).

Place the annotation on the pod template, not on the Deployment metadata.

## Instrumentation CR (auto-instrumentation)

The `Instrumentation` CR configures automatic SDK injection into application pods.
The operator uses a mutating webhook to inject an init container that copies the instrumentation agent into a shared volume and sets the required environment variables (`JAVA_TOOL_OPTIONS`, `NODE_OPTIONS`, `PYTHONPATH`, etc.).

```yaml
apiVersion: opentelemetry.io/v1alpha1
kind: Instrumentation
metadata:
  name: otel-instrumentation
  namespace: otel
spec:
  exporter:
    endpoint: http://otel-agent.otel.svc.cluster.local:4317
  propagators:
    - tracecontext
    - baggage
  sampler:
    type: always_on
  resource:
    attributes:
      - name: deployment.environment.name
        value: production
  java:
    image: ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-java:latest
  python:
    image: ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-python:latest
  nodejs:
    image: ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-nodejs:latest
  dotnet:
    image: ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-dotnet:latest
  go:
    image: ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-go:latest
```

### Injecting instrumentation

Annotate the application pod template with the language-specific annotation:

| Language | Annotation |
|----------|-----------|
| Java | `instrumentation.opentelemetry.io/inject-java: "true"` |
| Python | `instrumentation.opentelemetry.io/inject-python: "true"` |
| Node.js | `instrumentation.opentelemetry.io/inject-nodejs: "true"` |
| .NET | `instrumentation.opentelemetry.io/inject-dotnet: "true"` |
| Go | `instrumentation.opentelemetry.io/inject-go: "true"` |

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-java-app
spec:
  template:
    metadata:
      annotations:
        instrumentation.opentelemetry.io/inject-java: "true"
    spec:
      containers:
        - name: my-java-app
          image: my-java-app:latest
```

The annotation value follows the same pattern as the sidecar annotation: `"true"` (same namespace), a specific CR name, or a namespaced reference.

For multi-container pods, use the `instrumentation.opentelemetry.io/container-names` annotation to target specific containers:

```yaml
annotations:
  instrumentation.opentelemetry.io/inject-java: "true"
  instrumentation.opentelemetry.io/container-names: "my-java-app"
```

### Environment variable precedence

When the operator injects environment variables, the following precedence applies (highest first):

1. Original container environment variables (always preserved).
2. Language-specific block variables (`spec.java.env`, `spec.python.env`, etc.).
3. Common variables (`spec.env`).

Set `OTEL_SERVICE_NAME` in the application's own environment to override any operator-injected value.

## Validating the setup with the debug exporter

Add the `debug` exporter to the `OpenTelemetryCollector` CR to verify that the pipeline processes telemetry correctly before sending it to a production backend.

```yaml
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: otel-gateway
  namespace: otel
spec:
  config:
    exporters:
      debug:
        verbosity: detailed
      otlp:
        endpoint: <OTLP_ENDPOINT>
        headers:
          Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter, k8sattributes, resource]
          exporters: [otlp, debug]
        metrics:
          receivers: [otlp]
          processors: [memory_limiter, k8sattributes, resource]
          exporters: [otlp, debug]
        logs:
          receivers: [otlp]
          processors: [memory_limiter, k8sattributes, resource]
          exporters: [otlp, debug]
```

Apply the updated CR and inspect the output:

```bash
kubectl apply -f otel-gateway.yaml
kubectl logs -n otel -l app.kubernetes.io/managed-by=opentelemetry-operator --tail=100 -f
```

### What to check

Run through this checklist after adding the `debug` exporter:

1. **Resource attributes are present.**
   Verify that `resourcedetection` and `k8sattributes` added the expected attributes (e.g., `host.name`, `k8s.namespace.name`, `k8s.deployment.name`).
   If attributes are missing, check detector configuration and RBAC permissions.
2. **Resource attributes are consistent across signals.**
   Compare the resource attributes on a trace, a metric, and a log record from the same service.
   All three must carry the same set of resource attributes.
   A mismatch means a processor is missing from one of the pipelines.
3. **Filters drop the right data.**
   Confirm that outdated or unwanted metrics no longer appear in the output.
   If a metric that should be dropped still shows up, check the `filter` processor's `instrumentation_scope.name` condition.
4. **Metric names and units match stable semantic conventions.**
   Verify that the metrics reaching the exporter use the expected names (e.g., `http.server.request.duration`, not `http.server.duration`) and units (e.g., `s`, not `ms`).
5. **Spans have expected attributes and parent-child relationships.**
   Check that business attributes set in application code (e.g., `order.id`) appear on spans, and that `CLIENT` spans are children of `SERVER` spans (not root spans).
6. **Dash0 dataset header is set.**
   If the organization uses multiple [datasets](https://www.dash0.com/documentation/dash0/key-concepts/datasets), verify the `Dash0-Dataset` header in the `OpenTelemetryCollector` CR.
   The debug exporter shows telemetry data, not outgoing headers — check the CR directly.
   See [Dash0 dataset routing](#dash0-dataset-routing).

### Remove the debug exporter before deploying to production

The `debug` exporter serializes every telemetry item to stdout.
In production this wastes CPU and I/O, and risks logging sensitive attribute values.
Remove `debug` from all pipeline `exporters` lists and the `exporters` section in the CR once validation is complete.

See [debug exporter](../exporters.md#debug-exporter) for verbosity levels and configuration.

## Dash0 dataset routing

If the Dash0 organization uses multiple [datasets](https://www.dash0.com/documentation/dash0/key-concepts/datasets), add the `Dash0-Dataset` header to the OTLP exporter in the `OpenTelemetryCollector` CR.

```yaml
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: otel-gateway
spec:
  config:
    exporters:
      otlp:
        endpoint: <OTLP_ENDPOINT>
        headers:
          Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
          Dash0-Dataset: "my-dataset"
```

A missing or incorrect `Dash0-Dataset` header causes telemetry to land in the default dataset.
The debug exporter shows telemetry data, not outgoing headers — verify the header in the `OpenTelemetryCollector` CR directly.

## Anti-patterns

- **Missing cert-manager.**
  The operator's webhook requires TLS certificates from cert-manager.
  Install cert-manager before the operator, or the webhook pods fail to start.
- **Sidecar annotation on Deployment metadata instead of pod template.**
  The operator watches pod creation events.
  Annotations on the Deployment metadata are not propagated to pods.
  Place annotations on `spec.template.metadata.annotations`.
- **Using `latest` image tags for auto-instrumentation.**
  Pin auto-instrumentation images to specific versions to avoid unexpected SDK upgrades that may change telemetry behaviour.
- **Go auto-instrumentation without `OTEL_GO_AUTO_TARGET_EXE`.**
  The Go auto-instrumentation agent requires `OTEL_GO_AUTO_TARGET_EXE` to identify the target binary.
  Omitting it causes the injection to abort silently.

## References

- [OpenTelemetry Operator](https://github.com/open-telemetry/opentelemetry-operator)
- [Operator API reference](https://github.com/open-telemetry/opentelemetry-operator/blob/main/docs/api.md)
- [Auto-instrumentation](https://opentelemetry.io/docs/kubernetes/operator/automatic/)
- [Deployment patterns](../deployment.md)

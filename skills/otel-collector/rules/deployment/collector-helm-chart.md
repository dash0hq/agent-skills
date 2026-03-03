---
title: "Collector Helm chart"
impact: HIGH
tags:
  - deployment
  - kubernetes
  - helm
  - agent
  - gateway
---

# Collector Helm chart

The [OpenTelemetry Collector Helm chart](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-collector) deploys the Collector as a DaemonSet, Deployment, or StatefulSet with a single `helm install` command.
Use it instead of raw manifests for production Kubernetes deployments.

## Installation

```bash
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm install otel-collector open-telemetry/opentelemetry-collector \
  --namespace otel --create-namespace \
  --set mode=daemonset
```

## Mode

The `mode` value is required and determines the Kubernetes workload type.

| Mode | Workload | Use when |
|------|----------|----------|
| `daemonset` | DaemonSet | Agent — collect host metrics, pod logs, receive OTLP from local pods |
| `deployment` | Deployment | Gateway — centralized processing, enrichment, export |
| `statefulset` | StatefulSet | Gateway with persistent sending queues across restarts |

## Agent configuration (DaemonSet)

Deploy as an agent to collect node-level telemetry and receive OTLP from local applications.

```yaml
# values-agent.yaml
mode: daemonset

image:
  repository: ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-k8s

command:
  name: otelcol-k8s

presets:
  logsCollection:
    enabled: true
  kubernetesAttributes:
    enabled: true
  hostMetrics:
    enabled: true

resources:
  limits:
    cpu: 500m
    memory: 512Mi

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

extraEnvsFrom:
  - secretRef:
      name: dash0-credentials
```

Install with:

```bash
helm install otel-agent open-telemetry/opentelemetry-collector \
  --namespace otel --create-namespace \
  -f values-agent.yaml
```

### Presets

Presets inject both the collector configuration and the required Kubernetes plumbing (volumes, RBAC rules, volume mounts) automatically.
You cannot remove preset-injected config via `.Values.config` — if you need to override preset behaviour, disable the preset and configure the component manually.

| Preset | Key | Best mode | What it adds |
|--------|-----|-----------|--------------|
| Log collection | `presets.logsCollection.enabled` | daemonset | Filelog receiver, mounts `/var/log/pods` |
| Kubernetes attributes | `presets.kubernetesAttributes.enabled` | daemonset | `k8sattributes` processor and RBAC |
| Host metrics | `presets.hostMetrics.enabled` | daemonset | Hostmetrics receiver, mounts host filesystem |
| Kubelet metrics | `presets.kubeletMetrics.enabled` | daemonset | Kubeletstats receiver |
| Cluster metrics | `presets.clusterMetrics.enabled` | deployment | `k8s_cluster` receiver |
| Kubernetes events | `presets.kubernetesEvents.enabled` | deployment | `k8sobjects` receiver for event collection |

## Gateway configuration (Deployment)

Deploy as a gateway for centralized processing, enrichment, and export.

```yaml
# values-gateway.yaml
mode: deployment

image:
  repository: ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-k8s

command:
  name: otelcol-k8s

replicaCount: 2

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

presets:
  kubernetesAttributes:
    enabled: true

resources:
  limits:
    cpu: "2"
    memory: 2Gi

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
    resourcedetection:
      detectors: [env, system]
      timeout: 5s
      override: false
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
        processors: [memory_limiter, resourcedetection, resource]
        exporters: [otlp]
      metrics:
        receivers: [otlp]
        processors: [memory_limiter, resourcedetection, resource]
        exporters: [otlp]
      logs:
        receivers: [otlp]
        processors: [memory_limiter, resourcedetection, resource]
        exporters: [otlp]

extraEnvsFrom:
  - secretRef:
      name: dash0-credentials
```

### Image selection

Use the `otelcol-k8s` distribution for Kubernetes deployments.
It includes the Kubernetes-specific components (`k8sattributes`, `kubeletstats`, `k8s_cluster`) that are not in the core distribution.

| Distribution | Image | Use when |
|-------------|-------|----------|
| Kubernetes | `ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-k8s` | Kubernetes deployments (recommended) |
| Contrib | `otel/opentelemetry-collector-contrib` | You need components not in the K8s distribution |
| Core | `otel/opentelemetry-collector` | Minimal footprint, no contrib components needed |

Pin the image tag to a specific version in production (e.g., `tag: "0.120.0"`).
Do not use `latest`.

### Memory management

The chart sets `GOMEMLIMIT` to 80 percent of the memory limit by default (`useGOMEMLIMIT: true`).
Set `memory_limiter.limit_mib` to 80 percent of the container memory limit in the collector config.

## Validating the setup with the debug exporter

Add the `debug` exporter to the Helm values to verify that the pipeline processes telemetry correctly before sending it to a production backend.

```yaml
# values-debug.yaml (merge with your agent or gateway values)
config:
  exporters:
    debug:
      verbosity: detailed

  service:
    pipelines:
      traces:
        exporters: [otlp, debug]
      metrics:
        exporters: [otlp, debug]
      logs:
        exporters: [otlp, debug]
```

Upgrade the release with the debug values:

```bash
helm upgrade otel-collector open-telemetry/opentelemetry-collector \
  --namespace otel \
  -f values-agent.yaml \
  -f values-debug.yaml
```

Inspect the output:

```bash
kubectl logs -n otel -l app.kubernetes.io/name=opentelemetry-collector --tail=100 -f
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
   If the organization uses multiple [datasets](https://www.dash0.com/documentation/dash0/key-concepts/datasets), verify the `Dash0-Dataset` header in the Helm values.
   The debug exporter shows telemetry data, not outgoing headers — check the values file directly.
   See [Dash0 dataset routing](#dash0-dataset-routing).

### Remove the debug exporter before deploying to production

The `debug` exporter serializes every telemetry item to stdout.
In production this wastes CPU and I/O, and risks logging sensitive attribute values.
Remove the debug values overlay and upgrade the release once validation is complete.

See [debug exporter](../exporters.md#debug-exporter) for verbosity levels and configuration.

## Dash0 dataset routing

If the Dash0 organization uses multiple [datasets](https://www.dash0.com/documentation/dash0/key-concepts/datasets), add the `Dash0-Dataset` header to the OTLP exporter to route telemetry to the correct dataset.

```yaml
config:
  exporters:
    otlp:
      endpoint: <OTLP_ENDPOINT>
      headers:
        Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
        Dash0-Dataset: "my-dataset"
```

A missing or incorrect `Dash0-Dataset` header causes telemetry to land in the default dataset.
The debug exporter shows telemetry data, not outgoing headers — verify the header in the Helm values directly.

## Anti-patterns

- **Missing `mode` value.**
  The chart requires `mode` to be set explicitly.
  Omitting it causes a deployment failure.
- **Overriding preset config in `.Values.config`.**
  Preset-injected components cannot be removed via the config overlay.
  Disable the preset and configure the component manually instead.
- **Using the core image with Kubernetes presets.**
  The core distribution does not include `k8sattributes`, `kubeletstats`, or `k8s_cluster`.
  Use the `otelcol-k8s` or contrib image.

## References

- [OpenTelemetry Collector Helm chart](https://github.com/open-telemetry/opentelemetry-helm-charts/tree/main/charts/opentelemetry-collector)
- [Helm chart values](https://github.com/open-telemetry/opentelemetry-helm-charts/blob/main/charts/opentelemetry-collector/values.yaml)
- [Deployment patterns](../deployment.md)

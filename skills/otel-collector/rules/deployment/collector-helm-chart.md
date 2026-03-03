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

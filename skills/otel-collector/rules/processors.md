---
title: "Processors"
impact: HIGH
tags:
  - processors
  - memory-limiter
  - resource-detection
  - k8sattributes
  - sending-queue
---

# Processors

Processors transform telemetry between receivers and exporters.
Processor ordering within a pipeline determines the order of execution — incorrect ordering causes memory exhaustion or data loss.

## Required processor

### Memory limiter

The `memory_limiter` processor prevents the Collector from running out of memory.
Place it **first** in every pipeline so it can apply backpressure before other processors allocate memory.

```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
```

| Setting | Description | Recommendation |
|---------|-------------|----------------|
| `check_interval` | How often to check memory usage | `1s` for production |
| `limit_mib` | Hard memory limit in MiB | Set to 80 percent of the container's memory limit |
| `spike_limit_mib` | Soft limit margin for spike absorption | Set to 25 percent of `limit_mib` |

When memory usage exceeds `limit_mib - spike_limit_mib`, the processor starts refusing data.
When it exceeds `limit_mib`, the processor drops data to protect the process.

```yaml
# GOOD — memory_limiter first
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, resourcedetection]
      exporters: [otlp]

# BAD — memory_limiter after other processors
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [resourcedetection, memory_limiter]
      exporters: [otlp]
```

## Batching and the sending queue

Do not use the `batch` processor.
Configure batching at the exporter level using the `sending_queue` with persistent storage instead.

The `batch` processor buffers telemetry in memory before export.
When the Collector crashes or restarts, all buffered data is lost — there is no recovery mechanism.
The exporter's `sending_queue` with `storage: file_storage` persists batches to disk, surviving Collector restarts without data loss.

See [exporters](./exporters.md#sending-queue) for `sending_queue` configuration with `file_storage`.

```yaml
# GOOD — exporter-level batching with persistent storage
exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
      storage: file_storage

extensions:
  file_storage:
    directory: /var/lib/otelcol/queue

service:
  extensions: [file_storage]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, resourcedetection]
      exporters: [otlp]

# BAD — batch processor loses all buffered data on crash
processors:
  batch:
    send_batch_size: 8192
    timeout: 200ms

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, resourcedetection, batch]
      exporters: [otlp]
```

## Recommended processors

### Resource detection

The `resourcedetection` processor auto-detects environment attributes (host, cloud provider, container runtime).
Use it to enrich telemetry with infrastructure metadata without manual configuration.

```yaml
processors:
  resourcedetection:
    detectors:
      - env
      - system
      - docker
      - ec2
      - gcp
      - azure
    system:
      hostname_sources:
        - os
      resource_attributes:
        host.id:
          enabled: true
        host.name:
          enabled: true
        os.type:
          enabled: true
    timeout: 5s
    override: false
```

Set `override: false` to preserve attributes already set by the application SDK.
Only include detectors for your actual environment — unnecessary detectors add startup latency.

| Environment | Detectors to use |
|-------------|-----------------|
| AWS EC2 / ECS / EKS | `env`, `ec2`, `ecs` |
| Google Cloud / GKE | `env`, `gcp` |
| Azure / AKS | `env`, `azure` |
| Docker (local) | `env`, `system`, `docker` |
| Bare metal | `env`, `system` |

### Kubernetes attributes

The `k8sattributes` processor enriches telemetry with Kubernetes metadata by querying the Kubernetes API.
It matches incoming telemetry to a pod, then resolves namespace, deployment, node, labels, and other metadata automatically.

For guidance on setting the `k8s.pod.uid` resource attribute in application SDKs, see [resource attributes](../../otel-instrumentation/rules/resources.md).

```yaml
processors:
  k8sattributes:
    auth_type: serviceAccount
    passthrough: false
    extract:
      metadata:
        - k8s.namespace.name
        - k8s.deployment.name
        - k8s.deployment.uid
        - k8s.daemonset.name
        - k8s.daemonset.uid
        - k8s.statefulset.name
        - k8s.statefulset.uid
        - k8s.job.name
        - k8s.job.uid
        - k8s.cronjob.name
        - k8s.node.name
        - k8s.pod.name
        - k8s.pod.uid
        - k8s.pod.start_time
      labels:
        - tag_name: app
          key: app.kubernetes.io/name
        - tag_name: version
          key: app.kubernetes.io/version
    pod_association:
      - sources:
          - from: resource_attribute
            name: k8s.pod.uid
      - sources:
          - from: connection
```

Extract both name and UID for workload attributes (`k8s.deployment.name` and `k8s.deployment.uid`).
Kubernetes names are unique only among resources of the same type within the same namespace.
In multi-cluster environments, UIDs are required for unambiguous identification across clusters.

### Pod association

The `pod_association` list controls how the processor identifies which pod sent the telemetry.
Entries are evaluated in order — the first match wins.

| Strategy | Source | Reliability |
|----------|--------|-------------|
| Resource attribute `k8s.pod.uid` | `from: resource_attribute` | High — uniquely identifies a pod regardless of network topology |
| Connection IP | `from: connection` | Low — unreliable with service meshes (Istio, Linkerd) and shared-IP configurations |

Always list `k8s.pod.uid` as the first association source.
The connection IP fallback matches the IP of the TCP connection that delivered the telemetry, but service meshes route traffic through sidecar proxies, causing the processor to see the proxy's IP instead of the application pod's IP.

```yaml
# GOOD — k8s.pod.uid first, connection IP as fallback
pod_association:
  - sources:
      - from: resource_attribute
        name: k8s.pod.uid
  - sources:
      - from: connection

# BAD — relying solely on connection IP
pod_association:
  - sources:
      - from: connection
```

For this association to work, application pods must set `k8s.pod.uid` as a resource attribute via the Kubernetes downward API.
See [raw manifests](./deployment/raw-manifests.md) for the required pod spec configuration.

### Passthrough mode

| Mode | Setting | Use when |
|------|---------|----------|
| Full processing | `passthrough: false` | The Collector instance performs the Kubernetes API lookup itself |
| Passthrough | `passthrough: true` | A downstream Collector (e.g., a gateway) performs the lookup |

Use `passthrough: false` for gateway deployments and for standalone agent deployments that export directly to a backend.
Use `passthrough: true` only for agent (DaemonSet) deployments that forward to a downstream gateway where `k8sattributes` runs with `passthrough: false`.

In passthrough mode, the processor adds the pod's IP to the telemetry resource without performing any API lookup.
The downstream gateway then uses that IP (or `k8s.pod.uid`) to resolve the full Kubernetes metadata.

### RBAC requirements

The `k8sattributes` processor requires a `ServiceAccount` with read access to pods, namespaces, replicasets, and other workload resources.
Without the correct RBAC, the processor fails silently and telemetry is not enriched.
See [raw manifests](./deployment/raw-manifests.md) for the required ClusterRole and ClusterRoleBinding.

### Resource processor

Use the `resource` processor to set static resource attributes that apply to all telemetry flowing through the Collector.
This is the correct place for cluster-wide attributes like `k8s.cluster.name`, `k8s.cluster.uid`, or `deployment.environment.name`.

```yaml
processors:
  resource:
    attributes:
      - key: k8s.cluster.name
        value: "prod-eu-1"
        action: upsert
      - key: k8s.cluster.uid
        value: "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
        action: upsert
      - key: deployment.environment.name
        value: "production"
        action: upsert
```

A Kubernetes cluster has no built-in notion of its own name or identity — cluster names exist only in management tooling, not within the cluster itself.
Set `k8s.cluster.name` in the Collector to tag all telemetry with the cluster it originated from.

Set `k8s.cluster.uid` to the UID of the `kube-system` namespace, which exists in every cluster and has a unique UID.
Retrieve it with:

```bash
kubectl get namespace kube-system -o jsonpath='{.metadata.uid}'
```

In multi-cluster environments, `k8s.cluster.uid` is the only attribute that guarantees globally unique cluster identification.
Cluster names are human-chosen and can collide across teams or regions.

Use `action: upsert` to set the value regardless of whether the attribute already exists.
Use `action: insert` to set the value only when the attribute is not already present (preserves SDK-set values).

### Filter processor

The `filter` processor drops telemetry that matches a condition.
Use it to remove outdated or unwanted metrics, spans, or logs before they reach the backend.

A common use case is dropping metrics that auto-instrumentation libraries emit under outdated semantic convention names.
When an instrumentation library lags behind the stable specification, the application creates the correct metric manually and the Collector drops the outdated one.
See [auto-instrumented metrics must be tested](../../otel-instrumentation/rules/metrics.md#auto-instrumented-metrics-must-be-tested) for the full remediation workflow.

```yaml
processors:
  # Drop outdated HTTP metrics from instrumentation-http 0.53.x.
  # The application creates the correct stable semconv metrics manually.
  # Remove this filter when the library migrates to stable HTTP semantic conventions.
  filter/drop-outdated-http-metrics:
    error_mode: ignore
    metrics:
      metric:
        - 'name == "http.server.duration" and instrumentation_scope.name == "@opentelemetry/instrumentation-http"'
        - 'name == "http.client.duration" and instrumentation_scope.name == "@opentelemetry/instrumentation-http"'
```

Always scope the filter to the specific instrumentation scope (`instrumentation_scope.name`) that produces the outdated metric.
A name-only filter risks dropping a legitimate metric with the same name from a different source.

Add a comment with the library version that caused the mismatch and a note to remove the filter once the library upgrades.

## Processor ordering

Follow this numbered decision process to determine processor order in a pipeline:

1. **First**: `memory_limiter` — always first to apply backpressure before memory is allocated.
2. **Second**: `resourcedetection` and `k8sattributes` — enrich telemetry with metadata early so downstream processors can use it.
3. **Third**: `resource` — set static attributes after auto-detection to override or supplement detected values.
4. **Fourth**: Transform and filter processors — modify or drop telemetry based on enriched attributes.

Batching is handled by the exporter's `sending_queue`, not by a processor.
See [batching and the sending queue](#batching-and-the-sending-queue).

```yaml
# GOOD — correct ordering
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors:
        - memory_limiter
        - resourcedetection
        - k8sattributes
        - resource
        - filter/drop-outdated-http-metrics
        - transform
      exporters: [otlp]
```

## Anti-patterns

- **Missing `memory_limiter` in production.**
  A burst of telemetry causes the Collector to consume all available memory and crash.
  Always include `memory_limiter` as the first processor.
- **Using the `batch` processor in production.**
  The batch processor buffers data in memory with no persistence.
  A Collector crash or restart loses all buffered telemetry.
  Use the exporter's `sending_queue` with `storage: file_storage` instead.
- **`k8sattributes` without RBAC.**
  The processor fails silently and telemetry is not enriched.
  Ensure the Collector's ServiceAccount has the required ClusterRole.
- **Relying solely on connection IP for pod association.**
  Service meshes (Istio, Linkerd) route traffic through sidecar proxies, causing the processor to see the proxy's IP instead of the application pod's IP.
  Always set `k8s.pod.uid` as the primary association source via the downward API.
- **Extracting only names without UIDs in multi-cluster environments.**
  Kubernetes names are unique only within a namespace.
  Extract both `k8s.deployment.name` and `k8s.deployment.uid` (and similarly for other workload types) for unambiguous cross-cluster identification.
- **Missing `k8s.cluster.name` in multi-cluster deployments.**
  Identical workloads across clusters produce indistinguishable telemetry.
  Set `k8s.cluster.name` (and `k8s.cluster.uid`) in the `resource` processor.
- **`resourcedetection` with `override: true` overwriting SDK attributes.**
  Application-set attributes like `service.name` get replaced with auto-detected values (often `unknown_service`).
  Set `override: false` unless you intentionally want to replace SDK-set attributes.

## References

- [Memory limiter processor](https://github.com/open-telemetry/opentelemetry-collector/tree/main/processor/memorylimiterprocessor)
- [File storage extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/storage/filestorage)
- [Why the batch processor is going away](https://www.dash0.com/blog/why-the-opentelemetry-batch-processor-is-going-away-eventually)
- [Filter processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor)
- [Resource detection processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourcedetectionprocessor)
- [Kubernetes attributes processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/k8sattributesprocessor)
- [Resource processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/resourceprocessor)
- [Kubernetes attributes best practices](https://www.dash0.com/guides/opentelemetry-kubernetes-attributes-best-practices)

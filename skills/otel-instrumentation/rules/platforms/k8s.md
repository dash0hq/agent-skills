---
title: 'Kubernetes deployment'
impact: HIGH
tags:
  - kubernetes
  - deployment
  - resource-attributes
  - downward-api
---

# Kubernetes deployment

OpenTelemetry-instrumented applications running in Kubernetes need pod metadata injected as resource attributes for proper contextualization of traces, metrics, and logs.
This file covers the pod-spec changes needed for application containers and the Dash0 Kubernetes Operator approach.

For the full list of required and recommended resource attributes (including `service.name`, `service.version`, and `deployment.environment.name`), see [resource attributes](../resources.md).

## Pod metadata via downward API

Use the Kubernetes downward API to expose pod metadata as environment variables.
The SDK reads `OTEL_RESOURCE_ATTRIBUTES` at startup and attaches these values to every signal.

### `k8s.pod.uid` (critical)

The most important Kubernetes resource attribute.
The `k8sattributes` processor uses it to resolve most other Kubernetes metadata (namespace, deployment, node) automatically.

Always set `k8s.pod.uid` explicitly rather than relying on IP-based pod detection.
IP-based association is unreliable with service meshes (Istio, Linkerd) or non-standard network configurations where multiple pods share the same IP.

### `k8s.container.name` (critical)

Identifies the container within a pod.
Set it for every multi-container pod (e.g., pods with sidecar proxies or log collectors).
The `k8sattributes` processor cannot distinguish between containers that share the same pod UID and IP.

There is no downward API field for the container name.
Set it as a literal value that matches the container name in the pod spec.

### `k8s.pod.name`

Human-readable pod identifier.
Pod names are easier to search than UIDs but are unique only within a namespace on a cluster.

### `k8s.node.name`

Identifies the node on which the pod runs.
Required when investigating performance issues caused by resource contention or node-pressure evictions.

Not needed for AWS EKS on Fargate, where each pod runs on a dedicated virtual node.

### Complete pod spec example

```yaml
spec:
  containers:
    - name: <container-name>
      env:
        - name: K8S_POD_UID
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.uid
        - name: K8S_POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: K8S_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: OTEL_SERVICE_NAME
          value: <service-name>
        - name: OTEL_EXPORTER_OTLP_HEADERS
          valueFrom:
            secretKeyRef:
              name: otel-auth
              key: token
        - name: OTEL_RESOURCE_ATTRIBUTES
          value: "service.version=<service-version or commit sha>,deployment.environment.name=<dev-env>,k8s.pod.uid=$(K8S_POD_UID),k8s.pod.name=$(K8S_POD_NAME),k8s.node.name=$(K8S_NODE_NAME),k8s.container.name=<container-name>"
```

The `$(K8S_POD_UID)` syntax is a Kubernetes dependent environment variable reference — Kubernetes substitutes it with the value of the `K8S_POD_UID` variable defined earlier in the same `env` block.

The placeholders (`<service-name>`, `<service-version or commit sha>`, `<dev-env>`, `<container-name>`) require project-specific values.
See [resolving configuration values](../resolve-values.md) for ordered lookup strategies to derive each value from the codebase.

## Collector-level enrichment

Attributes that cannot be set from inside the pod — workload names and UIDs (`k8s.deployment.name`, `k8s.deployment.uid`, `k8s.namespace.name`, etc.) and cluster identity (`k8s.cluster.name`, `k8s.cluster.uid`) — are handled by OpenTelemetry Collector processors.

For `k8sattributes` processor configuration (metadata extraction, pod association, passthrough mode, RBAC), see [processors](../../../otel-collector/rules/processors.md).
For `resource` processor configuration (`k8s.cluster.name`, `k8s.cluster.uid`), see [processors](../../../otel-collector/rules/processors.md).
For Collector deployment manifests (DaemonSet, Deployment, RBAC), see [raw manifests](../../../otel-collector/rules/deployment/raw-manifests.md).

## Dash0 Kubernetes Operator

Use the [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator) to automate instrumentation and resource attribute injection for all workloads in a cluster.
The operator handles SDK injection, collector configuration, and Kubernetes metadata enrichment without manual pod spec changes.

When the Dash0 Kubernetes Operator manages the workload, skip the manual downward API and Collector processor setup above.

## Anti-patterns

- **Using `k8s.pod.ip` instead of `k8s.pod.uid`.**
  Pod IPs are reused and can be shared across pods in service mesh configurations.
  Always set `k8s.pod.uid` via the downward API.
- **Omitting `k8s.container.name` in multi-container pods.**
  The `k8sattributes` processor cannot distinguish between containers sharing the same pod UID and IP.
  There is no downward API field for the container name — set it as a literal value matching the container name in the pod spec.
- **Missing downward API environment variables.**
  Without `k8s.pod.uid` in `OTEL_RESOURCE_ATTRIBUTES`, the Collector's `k8sattributes` processor falls back to unreliable connection-IP matching.
  Always expose pod metadata via the downward API and pass it to the SDK.

## References

- [Kubernetes attributes best practices](https://www.dash0.com/guides/opentelemetry-kubernetes-attributes-best-practices)
- [Kubernetes semantic conventions](https://opentelemetry.io/docs/specs/semconv/resource/k8s/)
- [Kubernetes downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/)
- [Resource attributes](../resources.md)

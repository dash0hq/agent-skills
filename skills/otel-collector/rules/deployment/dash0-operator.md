---
title: "Dash0 Kubernetes Operator"
impact: HIGH
tags:
  - deployment
  - kubernetes
  - operator
  - dash0
  - auto-instrumentation
---

# Dash0 Kubernetes Operator

The [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator) automates OpenTelemetry instrumentation, Collector deployment, and telemetry export to Dash0 or any OTLP-compatible backend.
Use it when Dash0 is the observability backend and you want minimal configuration overhead.

## When to use the Dash0 Operator

| Condition | Use the Dash0 Operator |
|-----------|------------------------|
| Dash0 is the observability backend | Yes |
| You need auto-instrumentation without per-workload annotations | Yes |
| You want automatic Collector deployment and lifecycle management | Yes |
| You need full control over Collector pipelines and processors | No — use the [Collector Helm chart](./collector-helm-chart.md) or [OpenTelemetry Operator](./opentelemetry-operator.md) |
| You export to a non-Dash0 backend exclusively | Possible (via generic OTLP export), but the [OpenTelemetry Operator](./opentelemetry-operator.md) is a better fit |

## Installation

Install via Helm.
Requires Kubernetes 1.25.16+ and Helm 3.x+.

```bash
helm repo add dash0-operator https://dash0hq.github.io/dash0-operator
helm repo update dash0-operator

helm install \
  --wait \
  --namespace dash0-system \
  --create-namespace \
  --set operator.dash0Export.enabled=true \
  --set operator.dash0Export.endpoint=<OTLP_ENDPOINT> \
  --set operator.dash0Export.apiEndpoint=<API_ENDPOINT> \
  --set operator.dash0Export.secretRef.name=dash0-credentials \
  --set operator.dash0Export.secretRef.key=auth-token \
  --set operator.clusterName=<CLUSTER_NAME> \
  dash0-operator \
  dash0-operator/dash0-operator
```

Replace the placeholders:
- `<OTLP_ENDPOINT>`: copy the gRPC endpoint from [Settings → Endpoints → OTLP via gRPC](https://app.dash0.com/goto/settings/endpoints?endpoint_type=otlp_grpc) (e.g., `ingress.eu-west-1.aws.dash0.com:4317`).
- `<API_ENDPOINT>`: copy the API endpoint from [Settings → Endpoints → API](https://app.dash0.com/goto/settings/endpoints?endpoint_type=api_http).
  Omit `apiEndpoint` if you do not need GitOps features (dashboard sync, check rule sync, synthetic checks, views).
- `<CLUSTER_NAME>`: a human-readable name for the cluster, used as the `k8s.cluster.name` resource attribute.

### Authentication

Always use a Secret reference in production.
Create the Secret before installing the operator.

```bash
kubectl create namespace dash0-system
kubectl create secret generic dash0-credentials \
  --namespace dash0-system \
  --from-literal=auth-token=<AUTH_TOKEN>
```

Replace `<AUTH_TOKEN>` with a token from [Settings → Auth Tokens](https://app.dash0.com/settings/auth-tokens).

#### Token permissions

The auth token requires different scopes depending on which operator features you use.

| Feature | Required scope |
|---------|---------------|
| Telemetry export (traces, metrics, logs) | **Ingesting** |
| GitOps — dashboard sync (PersesDashboard), check rule sync (PrometheusRule), synthetic checks, views | **All permissions** |

If you only need telemetry export, create a token with **ingesting** permissions.
If you also use the `apiEndpoint` to synchronise dashboards, check rules, synthetic checks, or views as Kubernetes resources, create a token with **all permissions**.

The Helm chart also accepts an inline token via `operator.dash0Export.token`, but this stores the token in a ConfigMap (not a Secret), making it readable by anyone with cluster API access.
Do not use inline tokens in production.

## Custom resources

The operator provides two primary CRDs.

| CRD | Scope | API version | Purpose |
|-----|-------|-------------|---------|
| `Dash0OperatorConfiguration` | Cluster | `operator.dash0.com/v1alpha1` | Backend connection, export targets, cluster-wide settings |
| `Dash0Monitoring` | Namespace | `operator.dash0.com/v1beta1` | Enables instrumentation and monitoring for workloads in a namespace |

### Dash0OperatorConfiguration

When you install with `operator.dash0Export.enabled=true`, the Helm chart creates a `Dash0OperatorConfiguration` automatically.
Do not edit this auto-generated resource manually — the operator overwrites it on restart.
Change Helm values instead.

To create the resource manually (e.g., when installing without `operator.dash0Export.enabled=true`):

```yaml
apiVersion: operator.dash0.com/v1alpha1
kind: Dash0OperatorConfiguration
metadata:
  name: dash0-operator-configuration
spec:
  exports:
    - dash0:
        endpoint: <OTLP_ENDPOINT>
        authorization:
          secretRef:
            name: dash0-credentials
            key: auth-token
        apiEndpoint: <API_ENDPOINT>
  clusterName: <CLUSTER_NAME>
  selfMonitoring:
    enabled: true
  kubernetesInfrastructureMetricsCollection:
    enabled: true
  collectPodLabelsAndAnnotations:
    enabled: true
```

Key fields:

| Field | Default | Description |
|-------|---------|-------------|
| `spec.exports` | — | One or more export targets (Dash0 or generic OTLP) |
| `spec.clusterName` | — | Populates `k8s.cluster.name` on all telemetry |
| `spec.selfMonitoring.enabled` | `true` | Operator self-monitoring telemetry |
| `spec.kubernetesInfrastructureMetricsCollection.enabled` | `true` | Collect Kubernetes infrastructure metrics (nodes, pods, containers) |
| `spec.collectPodLabelsAndAnnotations.enabled` | `true` | Convert pod labels and annotations to `k8s.pod.label.*` and `k8s.pod.annotation.*` resource attributes |
| `spec.telemetryCollection.enabled` | `true` | Master switch — disabling this stops all Collector deployment |
| `spec.prometheusCrdSupport.enabled` | `false` | Enable Target Allocator for ServiceMonitor, PodMonitor, and ScrapeConfig CRDs |

### Dash0Monitoring

Create one `Dash0Monitoring` resource per namespace to enable instrumentation.

```yaml
apiVersion: operator.dash0.com/v1beta1
kind: Dash0Monitoring
metadata:
  name: dash0-monitoring-resource
  namespace: my-namespace
spec:
  instrumentWorkloads:
    mode: all
```

#### Instrumentation modes

| Mode | Behaviour |
|------|-----------|
| `all` | Instruments existing workloads immediately (causes pod restarts) and all future workloads |
| `created-and-updated` | Instruments only newly deployed or updated workloads; avoids restarting existing pods |
| `none` | Disables and removes instrumentation from all workloads in the namespace |

Use `created-and-updated` in production to avoid unexpected pod restarts during initial rollout.
Switch to `all` only during a planned maintenance window.

#### Per-workload opt-out

Apply this label to any workload to prevent instrumentation:

```yaml
metadata:
  labels:
    dash0.com/enable: "false"
```

## Auto-instrumentation

The operator automatically detects the application runtime and injects instrumentation via init containers and environment variables.
No per-workload annotations are required (unlike the [OpenTelemetry Operator](./opentelemetry-operator.md)).

### Supported runtimes

| Runtime | Minimum version | Notes |
|---------|----------------|-------|
| Node.js | 16+ | Uses Dash0 custom OpenTelemetry distribution |
| Java | 8+ | Uses upstream OpenTelemetry Java agent |
| .NET | All versions supported by the OTel .NET SDK | — |
| Python | 3.9+ | Beta; requires explicit opt-in and `http/protobuf` protocol |

Enable Python auto-instrumentation in Helm values:

```yaml
operator:
  instrumentation:
    enablePythonAutoInstrumentation: true
```

Python auto-instrumentation requires `http/protobuf` as the OTLP protocol (not gRPC).
It is incompatible with existing OpenTelemetry instrumentation in the same process.

## Collector management

The operator deploys and manages OpenTelemetry Collectors automatically when `telemetryCollection.enabled` is `true` (the default).

### What gets deployed

| Component | Workload | Purpose |
|-----------|----------|---------|
| Node collector | DaemonSet | OTLP receiving, pod log collection (filelog), kubeletstats, Prometheus scraping |
| Cluster collector | Deployment | Cluster-level Kubernetes metrics |
| Target Allocator | Deployment (optional) | Distributes Prometheus scrape targets when `prometheusCrdSupport.enabled` is `true` |

You do not need to write Collector configuration.
The operator generates and manages the entire Collector pipeline, including receivers, processors, exporters, and resource attribute enrichment.

### Exporting to generic OTLP backends

The operator supports exporting to any OTLP-compatible backend alongside or instead of Dash0.
Add an OTLP export to the `Dash0OperatorConfiguration`:

```yaml
spec:
  exports:
    - dash0:
        endpoint: <OTLP_ENDPOINT>
        authorization:
          secretRef:
            name: dash0-credentials
            key: auth-token
        apiEndpoint: <API_ENDPOINT>
    - otlp:
        endpoint: my-other-backend:4317
        protocol: grpc
        headers:
          Authorization: "Bearer <TOKEN>"
```

Multiple exports are supported simultaneously.
Telemetry is sent to all configured destinations.

## Uninstallation

Remove monitoring from each namespace, then uninstall the operator:

```bash
# Remove monitoring from each namespace
kubectl delete dash0monitoring dash0-monitoring-resource --namespace my-namespace

# Uninstall the operator
helm uninstall dash0-operator --namespace dash0-system

# Clean up CRDs (optional)
kubectl delete crd dash0monitoring.operator.dash0.com
kubectl delete crd dash0operatorconfigurations.operator.dash0.com
```

## Anti-patterns

- **Using `mode: all` without a maintenance window.**
  Setting `instrumentWorkloads.mode` to `all` causes immediate pod restarts across the namespace.
  Use `created-and-updated` for gradual rollout.
- **Editing the auto-generated `Dash0OperatorConfiguration`.**
  The operator overwrites it on restart.
  Update Helm values as the source of truth.
- **Using inline tokens in production.**
  Inline tokens are stored in a ConfigMap, not a Secret.
  Always use `secretRef`.
- **Running Dash0 and OTel Operator instrumentation on the same workloads.**
  Both operators inject init containers and environment variables.
  This causes double instrumentation or conflicts.
  Use one operator per workload.
- **Disabling `telemetryCollection` while expecting infrastructure metrics.**
  Setting `telemetryCollection.enabled=false` stops all Collector deployment, including infrastructure metrics collection.
- **Enabling Python auto-instrumentation without verifying prerequisites.**
  Python requires 3.9+, `http/protobuf` protocol, and no existing OTel instrumentation.
  Missing any prerequisite causes silent deactivation.

## References

- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)
- [Dash0 Integration Hub](https://www.dash0.com/hub/integrations)
- [Deployment patterns](../deployment.md)
- [OpenTelemetry Operator](./opentelemetry-operator.md)

---
title: "Dash0 Kubernetes Operator"
impact: HIGH
tags:
  - deployment
  - kubernetes
  - operator
  - dash0
  - auto-instrumentation
  - gitops
  - argocd
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

## Validating the setup

The Dash0 Operator manages the Collector configuration automatically — you cannot add a `debug` exporter to the operator-managed pipeline.
Validate the setup through the Dash0 UI and the operator-managed Collector logs instead.

### Check the operator-managed Collector logs

Inspect the logs of the Collectors for startup errors, export failures, and dropped telemetry:

```bash
kubectl logs -n dash0-system -l app.kubernetes.io/part-of=dash0-operator --tail=100 -f
```

Look for error-level log entries indicating authentication failures, endpoint unreachability, or configuration problems.

### What to check in Dash0

Send a few requests to the application and verify the following in the Dash0 UI:

1. **Resource attributes are present.**
   Open a trace or metric in Dash0 and verify that Kubernetes metadata attributes are present (e.g., `k8s.namespace.name`, `k8s.deployment.name`, `k8s.pod.name`).
   If attributes are missing, check that the `Dash0Monitoring` resource exists in the application's namespace.
2. **Resource attributes are consistent across signals.**
   Compare the resource attributes on a trace, a metric, and a log record from the same service.
   All three must carry the same set of resource attributes.
3. **Metric names and units match stable semantic conventions.**
   Verify that the metrics in Dash0 use the expected names (e.g., `http.server.request.duration`, not `http.server.duration`) and units (e.g., `s`, not `ms`).
4. **Spans have expected attributes and parent-child relationships.**
   Check that business attributes set in application code (e.g., `order.id`) appear on spans, and that `CLIENT` spans are children of `SERVER` spans (not root spans).
5. **Telemetry lands in the correct dataset.**
   If the organization uses multiple [datasets](https://www.dash0.com/documentation/dash0/key-concepts/datasets), verify that telemetry appears in the expected dataset.
   See [Dash0 dataset routing](#dash0-dataset-routing) for configuration.

## Dash0 dataset routing

If the Dash0 organization uses multiple [datasets](https://www.dash0.com/documentation/dash0/key-concepts/datasets), set the `dataset` field on the Dash0 export in `Dash0OperatorConfiguration`.

```yaml
apiVersion: operator.dash0.com/v1alpha1
kind: Dash0OperatorConfiguration
metadata:
  name: dash0-operator-configuration
spec:
  exports:
    - dash0:
        endpoint: <OTLP_ENDPOINT>
        dataset: "my-dataset"
        authorization:
          secretRef:
            name: dash0-credentials
            key: auth-token
```

When installing via Helm, set the dataset with:

```bash
--set operator.dash0Export.dataset=my-dataset
```

A missing or incorrect dataset value causes telemetry to land in the default dataset.

Per-namespace dataset routing is also possible by overriding the export configuration in a `Dash0Monitoring` resource:

```yaml
apiVersion: operator.dash0.com/v1beta1
kind: Dash0Monitoring
metadata:
  name: dash0-monitoring-resource
  namespace: my-namespace
spec:
  instrumentWorkloads:
    mode: all
  export:
    dash0:
      endpoint: <OTLP_ENDPOINT>
      dataset: "namespace-specific-dataset"
      authorization:
        secretRef:
          name: dash0-credentials
          key: auth-token
```

Use per-namespace datasets to separate telemetry by team or environment within a single cluster.

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

## GitOps compatibility

When deploying workloads via GitOps tools (ArgoCD, Flux) in a cluster where the Dash0 Operator is installed, the operator's auto-instrumentation modifies pod specs by adding environment variables, labels, and init containers.
These modifications conflict with GitOps tools that enforce declarative state, causing settings to flip-flop between the GitOps-desired state and the operator's instrumentation.

### Avoiding environment variable conflicts

Do not define the following environment variables in GitOps-managed workload manifests.
The Dash0 Operator manages these automatically, and redefining them in Git causes reconciliation loops.

| Environment variable | Purpose |
|----------------------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP export endpoint |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | OTLP export protocol |
| `OTEL_PROPAGATORS` | Context propagation format |
| `LD_PRELOAD` | Injector shared library preload |
| `DASH0_NODE_IP` | Node IP for collector routing |
| `DASH0_OTEL_COLLECTOR_BASE_URL` | Collector base URL |
| `OTEL_INJECTOR_K8S_NAMESPACE_NAME` | Kubernetes namespace |
| `OTEL_INJECTOR_K8S_POD_NAME` | Pod name |
| `OTEL_INJECTOR_K8S_POD_UID` | Pod UID |
| `OTEL_INJECTOR_K8S_CONTAINER_NAME` | Container name |
| `OTEL_INJECTOR_SERVICE_NAME` | OTel service name |
| `OTEL_INJECTOR_SERVICE_NAMESPACE` | OTel service namespace |
| `OTEL_INJECTOR_SERVICE_VERSION` | OTel service version |
| `OTEL_INJECTOR_RESOURCE_ATTRIBUTES` | OTel resource attributes |

This restriction applies only to workloads in namespaces with a `Dash0Monitoring` resource whose `instrumentWorkloads.mode` is not `none`.
Workloads excluded via the `dash0.com/enable: "false"` label are also unaffected.

### ArgoCD: ignoring operator-generated TLS diffs

The Dash0 Operator Helm chart regenerates TLS certificates for in-cluster communication on every Helm template render.
When deploying the operator itself via ArgoCD, the regenerated certificates and `caBundle` values show up as permanent diffs in the ArgoCD UI, even when nothing has changed in Git.
ArgoCD's hard refresh also triggers this, since it re-renders the Helm templates.

Add `ignoreDifferences` to the ArgoCD `Application` resource that manages the Dash0 Operator:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: dash0-operator
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  source:
    chart: dash0-operator
    repoURL: https://dash0hq.github.io/dash0-operator
    targetRevision: "..."
  # ... remaining spec
  ignoreDifferences:
    - kind: Secret
      name: dash0-operator-certificates
      jsonPointers:
        - /data/ca.crt
        - /data/tls.crt
        - /data/tls.key
    - group: admissionregistration.k8s.io
      kind: MutatingWebhookConfiguration
      name: dash0-operator-injector
      jsonPointers:
        - /webhooks/0/clientConfig/caBundle
    - group: admissionregistration.k8s.io
      kind: MutatingWebhookConfiguration
      name: dash0-operator-monitoring-mutating
      jsonPointers:
        - /webhooks/0/clientConfig/caBundle
    - group: admissionregistration.k8s.io
      kind: MutatingWebhookConfiguration
      name: dash0-operator-operator-configuration-mutating
      jsonPointers:
        - /webhooks/0/clientConfig/caBundle
    - group: admissionregistration.k8s.io
      kind: ValidatingWebhookConfiguration
      name: dash0-operator-monitoring-validator
      jsonPointers:
        - /webhooks/0/clientConfig/caBundle
    - group: admissionregistration.k8s.io
      kind: ValidatingWebhookConfiguration
      name: dash0-operator-operator-configuration-validator
      jsonPointers:
        - /webhooks/0/clientConfig/caBundle
```

### Recommended GitOps strategy

Follow this decision process when using the Dash0 Operator with GitOps:

1. **Set `instrumentWorkloads.mode` to `created-and-updated`** in the `Dash0Monitoring` resource.
   This avoids immediate pod restarts and lets GitOps-driven deployments pick up instrumentation naturally on the next rollout.
2. **Remove operator-managed environment variables from workload manifests in Git.**
   Refer to the table above.
   If a workload already defines any of these variables, delete them from the Git manifest and let the operator inject them.
3. **If a workload must not be instrumented**, add `dash0.com/enable: "false"` to the workload's `metadata.labels` in Git.
   This is safe to commit — the operator will skip that workload entirely, so no conflict arises.
4. **If deploying the Dash0 Operator itself via ArgoCD**, add the `ignoreDifferences` block from the section above to the ArgoCD `Application` resource.

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
- **Defining operator-managed environment variables in GitOps manifests.**
  Variables like `OTEL_EXPORTER_OTLP_ENDPOINT` or `LD_PRELOAD` in Git-managed workload specs conflict with the operator's auto-instrumentation.
  This causes reconciliation loops where the GitOps tool and operator overwrite each other.
  See [GitOps compatibility](#gitops-compatibility).

## References

- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)
- [Dash0 Integration Hub](https://www.dash0.com/hub/integrations)
- [Deployment patterns](../deployment.md)
- [OpenTelemetry Operator](./opentelemetry-operator.md)

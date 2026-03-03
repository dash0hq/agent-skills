---
title: "Raw Kubernetes manifests"
impact: HIGH
tags:
  - deployment
  - kubernetes
  - manifests
  - agent
  - gateway
  - docker
---

# Raw Kubernetes manifests

Use raw manifests when you need full control over every resource, or in environments where Helm and operators are not available.
For most Kubernetes deployments, prefer the [Collector Helm chart](./collector-helm-chart.md), the [OpenTelemetry Operator](./opentelemetry-operator.md), or the [Dash0 Kubernetes Operator](./dash0-operator.md).

## Agent vs gateway

| Requirement | Pattern | Kubernetes workload |
|-------------|---------|---------------------|
| Collect host metrics, node-level logs | Agent | DaemonSet |
| Collect pod logs from node filesystem | Agent | DaemonSet |
| Receive OTLP from application SDKs | Agent or Gateway | DaemonSet or Deployment + HPA |
| Centralized processing and enrichment | Gateway | Deployment + HPA |
| Cross-signal derivation (span → metrics) | Gateway | Deployment + HPA |
| Reduce egress connections to backend | Gateway | Deployment + HPA |

### When to use both

Deploy an agent DaemonSet for local collection (host metrics, pod logs) and a gateway Deployment for centralized processing and export.
The agent forwards to the gateway via OTLP.

```
Applications → Agent (DaemonSet) → Gateway (Deployment) → Dash0
                 ↑                       ↑
          host metrics,            enrichment, sampling,
          pod logs                 queuing, export
```

This is the same topology required for [tail sampling](../sampling.md#architecture).
The agent tier uses a `loadbalancingexporter` to consistently hash the trace ID and route all spans of a trace to the same gateway instance, where the `tailsamplingprocessor` can make informed decisions on complete traces.
If you also need [RED metrics from traces](../red-metrics.md), place the `signaltometricsconnector` in the gateway before the sampling step to materialise metrics from all spans.

## Configuring application pods for Collector enrichment

For the `k8sattributes` processor to reliably identify which pod sent the telemetry, application pods must set `k8s.pod.uid` as a resource attribute.
Use the Kubernetes downward API to expose pod metadata as environment variables, then pass them to the OpenTelemetry SDK via `OTEL_RESOURCE_ATTRIBUTES`.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      containers:
        - name: my-app
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
              value: "my-app"
            - name: OTEL_RESOURCE_ATTRIBUTES
              value: "k8s.pod.uid=$(K8S_POD_UID),k8s.pod.name=$(K8S_POD_NAME),k8s.node.name=$(K8S_NODE_NAME),k8s.container.name=my-app"
```

`k8s.pod.uid` is the most critical attribute.
The `k8sattributes` processor uses it to resolve all other Kubernetes metadata (namespace, deployment, labels) from the Kubernetes API.
Without it, the processor falls back to connection IP matching, which is unreliable with service meshes.

Set `k8s.container.name` manually — there is no downward API field for the container name.
This attribute is essential for multi-container pods (e.g., pods with sidecar proxies), because the processor cannot distinguish between containers sharing the same pod UID and IP.

For detailed resource attribute guidance, see [resource attributes](../../../otel-instrumentation/rules/resources.md).

## Agent deployment (DaemonSet)

Use a DaemonSet when the Collector needs access to node-level resources (host metrics, pod log files, container runtime).
Each node runs one Collector instance.

### Kubernetes DaemonSet manifest

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: otel-collector-agent
  namespace: otel
  labels:
    app.kubernetes.io/name: otel-collector
    app.kubernetes.io/component: agent
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: otel-collector
      app.kubernetes.io/component: agent
  template:
    metadata:
      labels:
        app.kubernetes.io/name: otel-collector
        app.kubernetes.io/component: agent
    spec:
      serviceAccountName: otel-collector
      containers:
        - name: collector
          image: otel/opentelemetry-collector-contrib:latest
          args:
            - --config=/etc/otelcol/config.yaml
          ports:
            - name: otlp-grpc
              containerPort: 4317
              hostPort: 4317
              protocol: TCP
            - name: otlp-http
              containerPort: 4318
              hostPort: 4318
              protocol: TCP
            - name: health
              containerPort: 13133
              protocol: TCP
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
          resources:
            requests:
              cpu: 200m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
          livenessProbe:
            httpGet:
              path: /
              port: health
            initialDelaySeconds: 15
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /
              port: health
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: config
              mountPath: /etc/otelcol
            - name: varlogpods
              mountPath: /var/log/pods
              readOnly: true
            - name: varlibdockercontainers
              mountPath: /var/lib/docker/containers
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: otel-collector-agent-config
        - name: varlogpods
          hostPath:
            path: /var/log/pods
        - name: varlibdockercontainers
          hostPath:
            path: /var/lib/docker/containers
```

Set `hostPort` on the OTLP ports so applications on the same node can send to `localhost:4317` without a Service.
Mount `/var/log/pods` for the filelog receiver and `/var/lib/docker/containers` for Docker container log symlinks.

### Agent resource limits

Set `memory_limiter.limit_mib` to 80 percent of the container memory limit.
For a 512 MiB limit, set `limit_mib: 410` and `spike_limit_mib: 100`.

## Gateway deployment (Deployment)

Use a Deployment for centralized processing: Kubernetes metadata enrichment, cross-signal derivation, and export.
Scale horizontally with a HorizontalPodAutoscaler.

### Kubernetes Deployment manifest

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector-gateway
  namespace: otel
  labels:
    app.kubernetes.io/name: otel-collector
    app.kubernetes.io/component: gateway
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: otel-collector
      app.kubernetes.io/component: gateway
  template:
    metadata:
      labels:
        app.kubernetes.io/name: otel-collector
        app.kubernetes.io/component: gateway
    spec:
      serviceAccountName: otel-collector
      containers:
        - name: collector
          image: otel/opentelemetry-collector-contrib:latest
          args:
            - --config=/etc/otelcol/config.yaml
          ports:
            - name: otlp-grpc
              containerPort: 4317
              protocol: TCP
            - name: otlp-http
              containerPort: 4318
              protocol: TCP
            - name: health
              containerPort: 13133
              protocol: TCP
            - name: metrics
              containerPort: 8888
              protocol: TCP
          env:
            - name: DASH0_AUTH_TOKEN
              valueFrom:
                secretKeyRef:
                  name: dash0-credentials
                  key: auth-token
          resources:
            requests:
              cpu: 500m
              memory: 1Gi
            limits:
              cpu: "2"
              memory: 2Gi
          livenessProbe:
            httpGet:
              path: /
              port: health
            initialDelaySeconds: 15
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /
              port: health
            initialDelaySeconds: 5
            periodSeconds: 5
          volumeMounts:
            - name: config
              mountPath: /etc/otelcol
      volumes:
        - name: config
          configMap:
            name: otel-collector-gateway-config
---
apiVersion: v1
kind: Service
metadata:
  name: otel-collector-gateway
  namespace: otel
  labels:
    app.kubernetes.io/name: otel-collector
    app.kubernetes.io/component: gateway
spec:
  type: ClusterIP
  ports:
    - name: otlp-grpc
      port: 4317
      targetPort: otlp-grpc
      protocol: TCP
    - name: otlp-http
      port: 4318
      targetPort: otlp-http
      protocol: TCP
  selector:
    app.kubernetes.io/name: otel-collector
    app.kubernetes.io/component: gateway
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: otel-collector-gateway
  namespace: otel
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: otel-collector-gateway
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

Start with at least 2 replicas for redundancy.
Scale based on CPU utilization — the Collector is CPU-bound during serialization and compression.

### RBAC for Kubernetes attributes processor

The `k8sattributes` processor needs read access to the Kubernetes API.
Apply this RBAC configuration for both agent and gateway deployments.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: otel-collector
  namespace: otel
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: otel-collector
rules:
  - apiGroups: [""]
    resources: ["pods", "namespaces", "nodes"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["replicasets", "deployments", "daemonsets", "statefulsets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: otel-collector
subjects:
  - kind: ServiceAccount
    name: otel-collector
    namespace: otel
roleRef:
  kind: ClusterRole
  name: otel-collector
  apiGroup: rbac.authorization.k8s.io
```

## Docker Compose for local development

Use Docker Compose to run the Collector locally for development and testing.

```yaml
services:
  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    command:
      - --config=/etc/otelcol/config.yaml
    ports:
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
      - "13133:13133" # Health check
      - "8888:8888"   # Internal metrics
    volumes:
      - ./otel-collector-config.yaml:/etc/otelcol/config.yaml:ro
    environment:
      - DASH0_AUTH_TOKEN=${DASH0_AUTH_TOKEN}
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:13133/"]
      interval: 10s
      timeout: 5s
      retries: 3
```

Create a `otel-collector-config.yaml` file alongside `docker-compose.yaml` with your Collector configuration.
See [pipelines](../pipelines.md) for a complete working configuration.

## Health checks and readiness probes

Enable the `health_check` extension to expose a health endpoint on port 13133.
Kubernetes liveness and readiness probes target this endpoint.

```yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133

service:
  extensions: [health_check]
```

| Probe | Path | Purpose |
|-------|------|---------|
| Liveness | `GET /` on port 13133 | Restarts the pod if the Collector is unresponsive |
| Readiness | `GET /` on port 13133 | Removes the pod from the Service if not ready to receive traffic |

The health check extension responds with HTTP 200 when the Collector is running and all pipelines are healthy.
Set `initialDelaySeconds` to at least 15 seconds to allow the Collector to start up.

## Validating the setup with the debug exporter

Add the `debug` exporter to the Collector ConfigMap to verify that the pipeline processes telemetry correctly before sending it to a production backend.

```yaml
# In the ConfigMap referenced by --config
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
      processors: [memory_limiter, resourcedetection, k8sattributes, resource]
      exporters: [otlp, debug]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, resourcedetection, k8sattributes, resource]
      exporters: [otlp, debug]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, resourcedetection, k8sattributes, resource]
      exporters: [otlp, debug]
```

Apply the updated ConfigMap, restart the Collector pods, and inspect the output:

```bash
kubectl apply -f otel-collector-config.yaml
kubectl rollout restart -n otel daemonset/otel-collector-agent   # or deployment/otel-collector-gateway
kubectl logs -n otel -l app.kubernetes.io/name=otel-collector --tail=100 -f
```

For Docker Compose, inspect the output with:

```bash
docker compose logs -f otel-collector
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
   If the organization uses multiple [datasets](https://www.dash0.com/documentation/dash0/key-concepts/datasets), verify the `Dash0-Dataset` header in the ConfigMap.
   The debug exporter shows telemetry data, not outgoing headers — check the ConfigMap directly.
   See [Dash0 dataset routing](#dash0-dataset-routing).

### Remove the debug exporter before deploying to production

The `debug` exporter serializes every telemetry item to stdout.
In production this wastes CPU and I/O, and risks logging sensitive attribute values.
Remove `debug` from all pipeline `exporters` lists and the `exporters` section in the ConfigMap once validation is complete.

See [debug exporter](../exporters.md#debug-exporter) for verbosity levels and configuration.

## Dash0 dataset routing

If the Dash0 organization uses multiple [datasets](https://www.dash0.com/documentation/dash0/key-concepts/datasets), add the `Dash0-Dataset` header to the OTLP exporter in the Collector configuration.

```yaml
exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
      Dash0-Dataset: "my-dataset"
```

A missing or incorrect `Dash0-Dataset` header causes telemetry to land in the default dataset.
The debug exporter shows telemetry data, not outgoing headers — verify the header in the ConfigMap directly.

## Anti-patterns

- **Gateway without horizontal scaling.**
  A single gateway instance is a single point of failure.
  Always deploy gateways with at least 2 replicas and a HorizontalPodAutoscaler.
- **DaemonSet with heavy processing (transforms, k8sattributes lookups).**
  Heavy processing on every node wastes resources.
  Offload enrichment and transformation to a gateway Deployment.
- **Missing RBAC for `k8sattributes`.**
  The processor fails silently and does not enrich telemetry.
  Apply the ClusterRole and ClusterRoleBinding before deploying.
- **No resource limits on Collector containers.**
  Without limits, a misbehaving Collector consumes all node resources.
  Always set CPU and memory requests and limits.
- **Using `latest` image tag in production.**
  Pin the Collector image to a specific version (e.g., `otel/opentelemetry-collector-contrib:0.120.0`) for reproducible deployments.

## References

- [Deployment patterns](../deployment.md)
- [Collector Helm chart](./collector-helm-chart.md)
- [OpenTelemetry Operator](./opentelemetry-operator.md)
- [Dash0 Kubernetes Operator](./dash0-operator.md)
- [Collector deployment](https://opentelemetry.io/docs/collector/deployment/)
- [Collector on Kubernetes](https://opentelemetry.io/docs/collector/deployment/kubernetes/)
- [Kubernetes attributes best practices](https://www.dash0.com/guides/opentelemetry-kubernetes-attributes-best-practices)

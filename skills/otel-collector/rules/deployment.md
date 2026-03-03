---
title: "Deployment"
impact: HIGH
tags:
  - deployment
  - kubernetes
  - docker
  - agent
  - gateway
---

# Deployment

The OpenTelemetry Collector can be deployed in a variety of ways.
Choose the pattern based on what telemetry you collect and how you process it.

## Decision process

Follow these steps in order.
Stop at the first match.

### 1. Is the target environment Kubernetes?

If **no** — use [Docker Compose or a binary](./deployment/raw-manifests.md#docker-compose-for-local-development) for local development, or a system service for bare-metal hosts.
The remaining steps apply only to Kubernetes.

### 2. Is Dash0 the observability backend?

If **yes** and you do not need custom Collector pipelines (custom processors, connectors, or non-standard receivers) — use the **[Dash0 Kubernetes Operator](./deployment/dash0-operator.md)**.
It deploys and manages Collectors automatically, injects instrumentation without per-workload annotations, and synchronises dashboards and alert rules.

If **yes** but you need full control over the Collector pipeline — continue to step 3.
Configure the Collector to export to Dash0 via OTLP (see [exporters](./exporters.md)).

### 3. Do you need automatic SDK instrumentation injected into application pods?

If **yes** — use the **[OpenTelemetry Operator](./deployment/opentelemetry-operator.md)**.
It manages Collector instances via the `OpenTelemetryCollector` CRD and injects language-specific auto-instrumentation via the `Instrumentation` CRD.

If **no** — continue to step 4.

### 4. Is Helm available in the cluster?

If **yes** — use the **[Collector Helm chart](./deployment/collector-helm-chart.md)**.
Presets handle RBAC, volumes, and common receivers automatically.
Set `mode` to `daemonset` for an agent or `deployment` for a gateway.

If **no** — use **[raw Kubernetes manifests](./deployment/raw-manifests.md)**.

### Summary

```
Is the target Kubernetes?
├─ No  → Docker Compose / binary
└─ Yes
   └─ Is Dash0 the backend?
      ├─ Yes, standard pipelines  → Dash0 Operator
      ├─ Yes, custom pipelines    → continue ↓
      └─ No                       → continue ↓
         └─ Need auto-instrumentation?
            ├─ Yes  → OpenTelemetry Operator
            └─ No
               └─ Helm available?
                  ├─ Yes  → Collector Helm chart
                  └─ No   → Raw manifests
```

## Deployment method comparison

| Capability | [Dash0 Operator](./deployment/dash0-operator.md) | [OTel Operator](./deployment/opentelemetry-operator.md) | [Helm chart](./deployment/collector-helm-chart.md) | [Raw manifests](./deployment/raw-manifests.md) |
|------------|:-:|:-:|:-:|:-:|
| Automatic Collector deployment | Yes | Yes (via CRD) | Yes | Manual |
| Auto-instrumentation injection | Yes (automatic) | Yes (per-workload annotation) | No | No |
| Custom Collector pipelines | No | Yes | Yes | Yes |
| Sidecar Collector mode | No | Yes | No | Manual |
| Dashboard and alert sync | Yes | No | No | No |
| Prometheus CRD support (ServiceMonitor) | Yes (opt-in) | Yes | No | No |
| Requires Helm | Yes | No | Yes | No |
| Requires cert-manager | No | Yes | No | No |

## Agent vs gateway

For agent vs gateway pattern selection (DaemonSet, Deployment, or both), see [raw manifests](./deployment/raw-manifests.md#agent-vs-gateway).
The same decision applies when deploying with the [Collector Helm chart](./deployment/collector-helm-chart.md) or the [OpenTelemetry Operator](./deployment/opentelemetry-operator.md).
The [Dash0 Kubernetes Operator](./deployment/dash0-operator.md) manages this topology automatically.

## References

- [Raw manifests](./deployment/raw-manifests.md)
- [Collector Helm chart](./deployment/collector-helm-chart.md)
- [OpenTelemetry Operator](./deployment/opentelemetry-operator.md)
- [Dash0 Kubernetes Operator](./deployment/dash0-operator.md)
- [Collector deployment](https://opentelemetry.io/docs/collector/deployment/)
- [Collector on Kubernetes](https://opentelemetry.io/docs/collector/deployment/kubernetes/)
- [Kubernetes attributes best practices](https://www.dash0.com/guides/opentelemetry-kubernetes-attributes-best-practices)

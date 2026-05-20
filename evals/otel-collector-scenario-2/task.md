# Kubernetes Telemetry Enrichment for Multi-Cluster Observability

## Problem/Feature Description

A large e-commerce company runs microservices across two Kubernetes clusters: a primary production cluster and a disaster-recovery cluster in a different region. Their platform observability team has deployed OpenTelemetry Collectors on each cluster, but telemetry arriving at their backend is difficult to analyze because it lacks Kubernetes context. Traces, metrics, and logs all arrive without information about which namespace, deployment, or pod they originated from, making it impossible to filter by workload or correlate signals from the same service.

The team has also run into an issue where infrastructure metadata (host names, cloud provider details) was overwriting the `service.name` set by their application SDKs, leaving spans labeled `unknown_service` instead of the intended service identifiers.

They want a single `collector-config.yaml` for the gateway Collector (a Deployment, not a DaemonSet) that solves these problems. The config should:

- Enrich all telemetry signals with Kubernetes metadata including namespace, deployment name and identity, pod name and identity, and node name
- Tag all telemetry with cluster-level attributes so data from each cluster can be distinguished
- Avoid overwriting resource attributes already set by application SDKs
- Work reliably with service-mesh environments (Istio/Linkerd) where connection-IP-based pod matching is unreliable

The cluster runs Istio as a service mesh. The clusters are named `prod-eu-1` and `dr-eu-2`. The unique cluster identifier should be derived from a built-in Kubernetes resource that has a unique UID per cluster.

## Output Specification

Produce a file named `collector-config.yaml` containing a complete OpenTelemetry Collector configuration for the gateway Collector. The configuration must cover all three signal types (traces, metrics, and logs).

Include a file named `notes.md` explaining:
1. What Kubernetes resource you used to derive the cluster UID and how to retrieve it
2. Why the pod association strategy is configured the way it is, given the service mesh environment


# Cleaning Up a Noisy Observability Pipeline: Stale Data, Missing Timestamps, and Error Tracing

## Problem/Feature Description

DataVault, a cloud storage provider, runs an OpenTelemetry Collector aggregating metrics, traces, and logs from their Kubernetes infrastructure and application services. Three problems have built up that are increasing their observability costs and reducing signal quality:

1. **Stale buffered data**: During incidents, some services queue up telemetry and replay it hours later. The backend rejects anything older than six hours but still bills for ingestion. The team wants the Collector to drop any data with a timestamp older than six hours before it reaches the exporter.

2. **Missing log timestamps**: A legacy logging library emits log records with no timestamps set. Downstream tooling fails to sort these logs correctly. The team wants the Collector to backfill both the observed time and the record time on any log that has neither.

3. **Noise reduction in metrics**: The Kubernetes control plane emits hundreds of `k8s.replicaset.*` metrics that DataVault's dashboards never use. The team wants these dropped at the Collector to reduce ingestion volume. Additionally, the team wants to mark any HTTP server span that completed with a 5xx status as an error span so that error-rate dashboards work correctly.

## Output Specification

Produce a complete OpenTelemetry Collector configuration file named `collector-config.yaml` with:

- Processor(s) that drop telemetry older than six hours
- Processor(s) that backfill timestamps on log records missing them  
- Processor(s) that drop the unwanted Kubernetes replicaset metrics
- Processor(s) that mark 5xx HTTP spans as error spans using the correct span status field
- A `service` section wiring separate `traces`, `metrics`, and `logs` pipelines from an `otlp` receiver through the processors to a `debug` exporter

Also produce a `pipeline-notes.md` explaining the approach taken for each of the four requirements, including which OTTL constructs were chosen and why.


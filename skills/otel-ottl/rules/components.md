# Collector components that use OTTL

OTTL is supported across a wide range of Collector components, not just the transform and filter processors.

## Processors

| Component | Use case |
|-----------|----------|
| transform | Modify, enrich, or redact telemetry (set attributes, rename fields, truncate values) |
| filter | Drop telemetry entirely (discard metrics by name, drop spans by status, remove noisy logs) |
| tailsampling | Sample traces based on OTTL conditions (e.g., keep error traces, drop health checks) |
| logdedup | Deduplicate log records using OTTL conditions |

## Connectors

| Component | Use case |
|-----------|----------|
| routing | Route telemetry to different pipelines based on OTTL conditions; `error_mode` defaults to `propagate` |
| count | Count spans, metrics, or logs matching OTTL conditions and emit as metrics; no `error_mode` key, errors always propagate |
| sum | Sum numeric values from telemetry matching OTTL conditions and emit as metrics; no `error_mode` key, errors always propagate |
| signaltometrics | Generate metrics from spans or logs using OTTL expressions for attribute extraction; `error_mode` defaults to `propagate` |

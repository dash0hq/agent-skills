---
title: "Telemetry validation"
impact: HIGH
tags:
  - validation
  - verification
  - checklist
  - deployment
---

# Telemetry validation

After instrumenting an application, verify that telemetry is arriving correctly at the backend.
Run this checklist after initial instrumentation, after SDK or Collector upgrades, after pipeline configuration changes, and during incident response when telemetry is suspected missing.

## Pre-flight checks

Verify these conditions before investigating the backend.
A failure at this stage means telemetry never left the application or never reached the Collector.

| Check | How to verify | Common fix |
|-------|---------------|------------|
| `OTEL_SERVICE_NAME` is set | Print the environment variable in the application logs at startup | Set it in the deployment manifest or application config |
| `OTEL_EXPORTER_OTLP_ENDPOINT` is set and reachable | `curl -v <endpoint>/v1/traces` returns a response (even an error response confirms network reachability) | Fix the endpoint URL, check DNS resolution, open network policies or firewall rules |
| Exporter protocol matches the receiver | If the Collector receiver listens on gRPC (port 4317), `OTEL_EXPORTER_OTLP_PROTOCOL` must be `grpc`, not `http/protobuf` | Set `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` or change the endpoint to the HTTP port (4318) |
| Collector is accepting data | Check `otelcol_receiver_accepted_spans`, `otelcol_receiver_accepted_metric_points`, and `otelcol_receiver_accepted_log_records` — all should be > 0 | See [pipelines — internal telemetry](../../otel-collector/rules/pipelines.md#internal-telemetry) |
| Collector is exporting data | Check `otelcol_exporter_sent_spans` > 0 and `otelcol_exporter_send_failed_spans` == 0 | Check exporter configuration, authentication tokens, and backend availability |

## Backend validation gate

Once pre-flight checks pass, verify that the backend contains the expected telemetry.
Work through these steps in order — each step depends on the previous one.

### 1. Service exists

Search the backend for the value of `service.name`.
If the service does not appear, check for naming mismatches: trailing whitespace, environment-specific prefixes, or case differences.

### 2. Resource attributes are present

Verify that the following resource attributes are populated on the telemetry from the service.

| Attribute | Why it matters |
|-----------|---------------|
| `service.name` | Identifies the service in the backend |
| `service.version` | Enables version-to-version comparison and deployment tracking |
| `deployment.environment.name` | Separates production from staging and development data |
| `k8s.namespace.name` (Kubernetes only) | Scopes queries to the correct namespace |

See [resources](./resources.md) for the full list of required and recommended resource attributes.

### 3. Expected span names appear

List the operations the service handles (HTTP endpoints, message consumers, cron jobs) and verify that each produces a span with the expected name.
If a span name is missing, the code path may not be instrumented, the route may not be receiving traffic, or the span may be sampled out.

### 4. Required attributes are populated

For each signal type, verify that key attributes are present and non-empty on the telemetry records.

| Signal | Attributes to check |
|--------|---------------------|
| Traces | `http.request.method`, `http.response.status_code`, `url.path` (for HTTP services); `db.system`, `db.operation.name` (for database clients) |
| Logs | `severity_number` is set (not `UNSET`), `body` is non-empty |
| Metrics | Expected metric names exist, units are correct |

### 5. Trace-to-log correlation works

Verify that log records emitted during a traced operation carry `trace_id` and `span_id` fields.
Query for a specific `trace_id` in the backend and confirm that both spans and logs appear.
If logs lack trace context, the logging bridge or SDK configuration is incomplete — see [logs](./logs.md) for setup guidance.

## Signal-specific checks

### Traces

| Check | Pass criteria |
|-------|---------------|
| Root span exists | Each inbound request produces exactly one `SERVER` span with no parent |
| Span kind is correct | `SERVER` for inbound, `CLIENT` for outbound, `PRODUCER`/`CONSUMER` for messaging |
| Error status is set | 5xx responses produce spans with status `ERROR` and a non-empty status message |
| No orphan spans | Every child span's parent exists in the trace |

See [spans](./spans.md) for detailed span hygiene rules.

### Metrics

| Check | Pass criteria |
|-------|---------------|
| Expected metric names exist | Application-defined and auto-instrumented metrics appear in the backend |
| Units are correct | `s` for seconds, `By` for bytes, `{request}` for counts — not `ms`, `kb`, or unitless |
| Cardinality is bounded | No metric has more than 1000 unique label combinations |

### Logs

| Check | Pass criteria |
|-------|---------------|
| Severity is set | Every log record has a `severity_number` above `UNSET` |
| Body is structured | Log bodies are structured (JSON or key-value), not unstructured strings, where the logging framework supports it |
| Trace correlation present | Logs emitted during a traced request carry `trace_id` and `span_id` |

## Common failures and fixes

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| No telemetry visible in the backend | Exporter protocol mismatch (HTTP/protobuf sent to gRPC receiver) | Set `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` or switch endpoint to port 4318 |
| Service name shows as `unknown_service` | `OTEL_SERVICE_NAME` not set | Set the environment variable in the deployment manifest |
| Spans appear but logs do not | Logging bridge not configured | Enable the OTel log appender for your logging framework — see [logs](./logs.md) |
| Logs lack `trace_id` | Log SDK not connected to the trace context | Ensure the logging bridge reads the active span context — see [logs](./logs.md) |
| Metrics missing after Collector restart | Cumulative metrics reset to zero; backend may treat the reset as a gap | Use the `cumulativetodelta` processor or wait for the next scrape interval |
| Resource attributes missing | `resourcedetection` or `k8sattributes` processor not in the pipeline | Add the processors — see [processors](../../otel-collector/rules/processors.md#recommended-processors) |
| High cardinality warning from the backend | Unparameterized URL paths or unbounded attribute values | Normalize paths in the SDK or Collector — see [sensitive data](./sensitive-data.md#path-parameterization) and the [otel-ottl skill](../../otel-ottl/SKILL.md#normalize-high-cardinality-attributes) |

## References

- [Pipelines — internal telemetry](../../otel-collector/rules/pipelines.md#internal-telemetry) — Collector self-monitoring metrics
- [Resources](./resources.md) — required and recommended resource attributes
- [Spans](./spans.md) — span naming, kind, status, and hygiene rules
- [Logs](./logs.md) — log bridge setup and trace correlation

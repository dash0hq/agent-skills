# Collector workspace

This directory is an OpenTelemetry Collector configuration workspace used by the eval scenarios for the `otel-collector` and `otel-ottl` skills.
The eval agent edits `config.yaml`; the harness then validates the result with `otelcol-contrib validate` and runs it verbatim as a local process, so the file must stay self-contained.

## Environment contract

The runtime environment provides the variables below.
Reference them with `${env:...}` placeholders in `config.yaml`; never replace them with hardcoded values.

| Variable | Meaning |
|----------|---------|
| `EVAL_OTLP_RECEIVER_PORT` | Port the OTLP/HTTP receiver must listen on (`127.0.0.1:${env:EVAL_OTLP_RECEIVER_PORT}`). |
| `EVAL_OTLP_ENDPOINT` | Base URL of the OTLP/HTTP endpoint the Collector must export to. |
| `EVAL_OTLP_TOKEN` | Bearer token the exporter must present in the `Authorization` header. |

## Operational constraints

The Collector runs as an unprivileged local process with this workspace as its working directory.

- Any directory the configuration references must be created inside this workspace and referenced by a relative path; there are no writable system paths.
- There is no metrics scrape target; keep the internal telemetry metrics level at `none`.
- The export endpoint does not accept compressed payloads; keep `compression: none` on the exporter.

---
title: 'Telemetry overview'
impact: CRITICAL
tags:
  - telemetry
  - overview
---

# Telemetry overview

This is the entrypoint for the OpenTelemetry instrumentation skill.
Start here, then follow the links below to the relevant rule files.

## Before you instrument

**[Resolve configuration values](./resolve-values.md) first.**
Every instrumentation setup requires project-specific values (`service.name`, `service.version`, `deployment.environment.name`, OTLP endpoint, auth token).
Use the ordered lookup strategies in that file to infer them from the codebase before writing any instrumentation code.

**[Resource attributes](./resources.md) identify what produces telemetry.**
They are attached to every signal automatically and must be set correctly before any other instrumentation work.

## Signals

Telemetry consists of three core signal types: **Metrics**, **Traces**, and **Logs**.
Each serves a distinct purpose in understanding system behavior.

| Signal  | When to use                                                         | Reference |
| ------- | ------------------------------------------------------------------- | --------- |
| Metrics | Alerting, SLOs, dashboards, trend analysis, capacity planning       | [metrics](./metrics.md) |
| Traces  | Request flow, latency breakdown, dependency mapping, error locality | [spans](./spans.md) |
| Logs    | Audit trails, event detail, causation after traces localize a fault | [logs](./logs.md) |

**Symptom-to-cause workflow:** Metrics surface problems → Traces pinpoint location → Logs explain causation.

**Correlation is essential.**
Link signals through shared context (trace IDs, span IDs) so you can navigate from an alert to the exact log line that explains the failure.

## Platforms

| Platform   | Reference |
| ---------- | --------- |
| Kubernetes | [Kubernetes deployment](./platforms/k8s.md) |

## References

- [Metrics](https://opentelemetry.io/docs/concepts/signals/metrics/)
- [Traces](https://opentelemetry.io/docs/concepts/signals/traces/)
- [Logs](https://opentelemetry.io/docs/concepts/signals/logs/)
- [Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)

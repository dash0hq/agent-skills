# Securing Telemetry Before Export: PII Redaction Pipeline

## Problem/Feature Description

FinTech startup Meridian Payments runs an OpenTelemetry Collector that forwards traces and logs from their payment processing services to a third-party observability vendor. A recent security audit revealed that spans contain raw user email addresses in `user.email`, authorization headers in `http.request.header.authorization`, and session cookies in `http.request.header.cookie`. Some log records carry raw credit card numbers embedded in their body text. Occasionally, application misconfigurations cause RSA private keys to appear in log bodies.

The engineering team wants to harden the pipeline before the next release. They want sensitive attributes scrubbed, partial values masked where possible, and records containing cryptographic key material dropped entirely before telemetry leaves the Collector. The pipeline already includes enrichment processors that add Kubernetes and environment metadata upstream; any redaction must happen after these enrichment stages so that the enriched attributes are available but no sensitive data reaches the exporter.

## Output Specification

Produce a complete OpenTelemetry Collector configuration file named `collector-config.yaml`. The config must define:

- All necessary processors to redact, mask, hash, and drop the sensitive data described above
- A service section that wires a `traces` pipeline and a `logs` pipeline, both starting from an `otlp` receiver, passing through any enrichment and redaction processors, and exiting through a `debug` exporter (use `verbosity: detailed`)
- A brief `redaction-notes.md` file explaining which strategy was applied to each type of sensitive data and why

The pipelines must be realistic enough that a reviewer can confirm correct processor ordering, correct redaction strategy per field type, and correct error handling configuration.


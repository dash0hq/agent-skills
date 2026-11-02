# Wiring a Python AWS Lambda Function to Dash0 with Upstream OpenTelemetry Layers

## Problem/Feature Description

A media company runs an image-resizing pipeline on AWS Lambda. The `ThumbnailRenderFunction` is a Python 3.12 function on the `x86_64` architecture, deployed to `eu-west-1` and packaged as a ZIP archive. It is triggered by S3 object-created events and calls DynamoDB to record each render.

The platform team wants distributed traces, metrics, and logs from this function in Dash0. They have standardised on the upstream OpenTelemetry Lambda layers across the organisation rather than any vendor-specific layer, because a second team consumes a copy of the same telemetry and they do not want to be locked into a single exporter. No changes to the handler code are acceptable.

Previous attempts failed in two ways that the team wants avoided. The first attempt added the layers but the function's p99 latency rose by roughly 400 milliseconds, because the export happened inside the invocation. The second attempt was an engineer copying the organisation's standard Kubernetes Collector configuration into the function, which never exported anything at all.

The Dash0 OTLP/gRPC endpoint is `ingress.eu-west-1.aws.dash0.com:4317`, telemetry must land in the `media-pipeline` dataset, and the auth token will be injected into the function as an environment variable named `DASH0_AUTH_TOKEN`.

## Output Specification

Produce the following:

1. `otel-collector-config.yaml` — the Collector configuration the function's Collector extension will load, covering traces, metrics, and logs, and avoiding the latency problem described above.
2. `deploy.sh` — the shell commands that package the configuration, publish it so the function can load it, and update the function so instrumentation is active. Assume the AWS CLI is configured and that the function already exists.
3. `README.md` — a short note listing every layer attached to the function, every environment variable set on it with its value, and how to verify telemetry is arriving.

Do not include any literal token or credential in any of the produced files.

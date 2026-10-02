# AWS Lambda OpenTelemetry packaging

## Problem description

Northstar runs 2 AWS Lambda functions in `eu-west-1` and wants to standardize them on OpenTelemetry without changing either function's deployment package type.

The first function, `CheckoutApi`, is a Node.js 22 function deployed as a `.zip` archive on `arm64`.
It has no existing OpenTelemetry, ADOT, or CloudWatch Application Signals configuration.
Telemetry must be sent to the company's existing vendor-neutral OTLP backend.
No current OpenTelemetry Lambda layer ARNs are provided, and you must not guess or invent release versions.

The second function, `ReconcileWorker`, is deployed from an ECR container image on `arm64`.
The team initially suggested attaching the same Lambda layers to it, but the deployment must remain a container image.

The OTLP endpoint and authorization value are managed outside source control.
Do not put a real credential or a made-up production credential into any generated file.
The platform team owns the Collector pipeline configuration separately, so wire its packaged location into the ZIP function but do not generate or duplicate `collector.yaml` in this task.

## Output specification

Produce the following files:

1. **`template.yaml`** — an AWS SAM template showing the deployment changes for both functions.
2. **`IMAGE_FUNCTION.md`** — concise implementation guidance for instrumenting the container-image function without changing its package type.
3. **`VALIDATION.md`** — a post-deployment checklist covering functional behavior, telemetry correctness, duplicate initialization, and cold-start impact.

Use deployment parameters or explicit placeholders where a current region/runtime/architecture-specific layer ARN or a secret-backed value must be resolved outside the generated files.

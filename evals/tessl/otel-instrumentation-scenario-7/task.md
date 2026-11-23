# Payment Webhook Function in Rust on AWS Lambda

## Problem Description

Lanternfish Payments receives payment-provider webhooks through a Rust AWS Lambda function, `payment-webhook`, exposed through a Lambda Function URL.
The function uses `lambda_runtime` 1.5 on Tokio, verifies the webhook, and forwards a confirmation for order `TEST-0001` to `https://ledger.example.test/confirmations` with `reqwest`.
It runs on the `provided.al2023` runtime on `arm64` and is deployed with AWS CDK in TypeScript.
It currently starts with `lambda_runtime::tracing::init_default_subscriber()` and exports no telemetry.

The team wants one trace per invocation that shows the webhook handling and the call to the ledger, with the function's AWS identity on the resource.
Invocations arrive minutes apart, so execution environments often sit idle and are frozen between invocations.
Telemetry must reach the backend through the upstream OpenTelemetry Collector Lambda layer, which the platform team already attaches with its configuration; that layer receives OTLP on `localhost:4317` (gRPC) and `localhost:4318` (HTTP).

- Required `service.name`: `payments`
- Required `deployment.environment.name`: `production`

## Output Specification

Produce the following files:

1. **`lambda/Cargo.toml`** — The function's manifest, including the binary definition and the instrumentation crates.
2. **`lambda/src/main.rs`** — The function with its OpenTelemetry setup.
3. **`lib/function.ts`** — The CDK construct for the function: runtime, architecture, code asset, and environment variables. Assume the Collector layers are passed in as a `layers` property.
4. **`NOTES.md`** — How to build the function and what to check after the first deployment.

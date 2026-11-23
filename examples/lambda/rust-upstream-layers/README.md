# Rust on AWS Lambda with the upstream OpenTelemetry Collector layer

## What this project demonstrates

A single AWS Lambda function written in Rust, instrumented with the OpenTelemetry Rust SDK in code, exporting OTLP over loopback to the upstream OpenTelemetry Collector Lambda layer, which ships the telemetry to Dash0.
Everything is provisioned with AWS CDK v2.

Rust takes this path because it has to.
The [AWS Lambda deployment](../../../skills/otel-instrumentation/rules/platforms/aws-lambda.md) rule routes Go, .NET, and Rust to manual SDK instrumentation: there is no upstream auto-instrumentation layer for any of them, so there is nothing for `AWS_LAMBDA_EXEC_WRAPPER` to point at.
The function therefore attaches only two layers — the Collector and its configuration — and owns its own provider setup, invocation span, and end-of-invocation flush.

The handler does a little CPU work, issues one outbound HTTPS request that fails DNS resolution on purpose, issues one that succeeds, and returns a small JSON body.
That produces an invocation span with a failed child span, a successful child span, and two custom spans underneath it.

## Project layout

| Path | Purpose |
|------|---------|
| `bin/app.ts` | CDK entry point; reads configuration from environment variables. |
| `lib/rust-upstream-layers-stack.ts` | The stack: layers, function, environment variables, Function URL. |
| `collector-config/otel-collector-config.yaml` | Collector configuration, packaged as its own layer. |
| `lambda/src/main.rs` | The handler and all OpenTelemetry wiring. |

## Prerequisites

- Node.js 20 or newer, for the CDK app.
- Rust 1.88 or newer, the highest `rust-version` among the pinned crates, and [`cargo lambda`](https://www.cargo-lambda.info/guide/installation.html), for the function.
  `cargo lambda` cross-compiles to `aarch64-unknown-linux-gnu` with Zig, so it needs no Docker daemon and no Linux host.
- An AWS account bootstrapped for CDK v2 (`npx cdk bootstrap`) and credentials in the environment.
- A Dash0 auth token stored in AWS Secrets Manager as a **plain string** secret, in the same region as the function.

The function and both layers are `arm64`.
Lambda layers are region- and architecture-scoped, so the Collector layer ARN built in the stack always names the function's own region, and the `arm64` build is paired with an `arm64` function.

## Configuration

Export these before synthesizing or deploying.
`bin/app.ts` fails with an explicit message when a required one is missing.

```bash
# Dash0 OTLP/gRPC ingress endpoint, host and port, no scheme.
# Settings -> Endpoints in Dash0.
export DASH0_OTLP_GRPC_ENDPOINT='ingress.eu-west-1.aws.dash0.com:4317'

# ARN of the Secrets Manager secret holding the Dash0 auth token as a plain string.
export DASH0_TOKEN_SECRET_ARN='arn:aws:secretsmanager:eu-west-1:111122223333:secret:TEST-dash0-token-AbCdEf'

# Dataset telemetry is routed to. Optional, defaults to "default".
export DASH0_DATASET='default'

# Target account and region for the stack.
export CDK_DEFAULT_ACCOUNT='111122223333'
export CDK_DEFAULT_REGION='eu-west-1'
```

## Build the function

The Rust build is a separate, explicit step rather than Docker bundling inside the CDK stack.

```bash
npm install
npm run build:function
```

That runs `cargo lambda build --release --arm64` in `lambda/`, which writes the executable to `lambda/target/lambda/bootstrap/bootstrap`.
The stack deploys that directory with `lambda.Code.fromAsset` on the `provided.al2023` runtime, which starts the executable named `bootstrap`.
The stack checks for the executable and fails synthesis with a pointer to this section if the step was skipped.

## Deploy

```bash
npx cdk deploy
```

The stack emits three outputs: `FunctionUrl`, `FunctionName`, and `LogGroupName`.

## Invoke

```bash
curl "$(aws cloudformation describe-stacks \
  --stack-name Dash0RustUpstreamLayersStack \
  --query "Stacks[0].Outputs[?OutputKey=='FunctionUrl'].OutputValue" \
  --output text)"
```

The response is a small JSON body carrying the request ID, the computed checksum, the trace ID, and the outcome of both outbound probes — one `failed`, one `ok`.

The Function URL has auth type `NONE`, so the endpoint is public for as long as the stack exists.
Tear the stack down when the exercise is over.

## Verify

### In CloudWatch

The log group is `/aws/lambda/Dash0RustUpstreamLayersStack-rust-demo`, also emitted as the `LogGroupName` output.
Both the handler and the Collector extension write there.

```bash
aws logs tail /aws/lambda/Dash0RustUpstreamLayersStack-rust-demo --since 10m --follow
```

Look for, in this order:

1. `Registering OpenTelemetry collector extension` and `Starting OpenTelemetry collector`, written by the Collector layer during the cold start.
   The absence of any Collector line means the layer is attached but not loading, which is almost always a region or architecture mismatch.
2. `Everything is ready. Begin running and processing data.`, the Collector's own startup line, which confirms the configuration YAML parsed.
   If the Collector instead reports `error decoding 'exporters'` or starts with no pipelines, the configuration layer did not land where `OPENTELEMETRY_COLLECTOR_CONFIG_URI` points.
3. `demo.invocation.start request_id=TEST-...` and `demo.invocation.end request_id=TEST-...`, written by the handler.
4. No `trace flush failed` or `metric flush failed` lines from the handler, and no `Export failed` or `context deadline exceeded` lines from the Collector.
   The handler lines mean the SDK cannot reach the Collector on loopback; the Collector lines mean the ingress endpoint, the dataset, or the auth token is wrong.

Invoke at least once after deploying: the extension only registers on a cold start.

### In Dash0

Filter on `service.name = dash0-rust-demo`.
A single invocation produces one trace containing:

- The invocation span, a SERVER span named after the function, carrying `faas.invocation_id`, `faas.coldstart`, and `faas.trigger`.
- `demo.work` and `demo.cpu`, the two custom spans from `main.rs`.
- Two `GET` CLIENT spans from `reqwest-tracing`: one to `aws.amazon.com` with an HTTP response status, and one to `example.test` with status `ERROR` and a DNS resolution failure.

The resource carries `faas.name`, `cloud.provider`, and `cloud.region` from `LambdaResourceDetector`, and `service.namespace=examples`, `service.version=0.1.0`, and `deployment.environment.name=development`, set through `OTEL_RESOURCE_ATTRIBUTES`.
The handler's CloudWatch lines also arrive as log records, because the Collector's `telemetryapi` receiver turns the Lambda Telemetry API stream into log records and feeds the logs pipeline.
The `demo.invocations` counter arrives as a metric.

## Where the end-of-invocation flush happens, and why

Lambda freezes the execution environment the moment the handler returns.
Anything still queued in a batch processor at that instant is not exported late — it is lost, and it is lost silently.

On the manual path there is no instrumentation layer to own that, so the function does it itself.
The flush lives in `lambda/src/main.rs`, in the `service_fn` closure, after the handler future completes:

```rust
let result = handler(demo, event).instrument(span.clone()).await;
if let Err(err) = &result {
    span.set_status(Status::error(err.to_string()));
}
drop(span);
if let Err(err) = tracer_provider.force_flush() {
    eprintln!("trace flush failed: {err}");
}
if let Err(err) = meter_provider.force_flush() {
    eprintln!("metric flush failed: {err}");
}
result
```

Three things are worth being precise about.

`drop(span)` comes before the flush because `tracing-opentelemetry` ends the OpenTelemetry span when the last handle to the `tracing` span closes.
An open span is not in the batch processor yet, so flushing first exports the children and leaves the invocation span behind until the next invocation, if one comes.

The `SdkMeterProvider` flush is not optional either.
Its periodic reader exports every 60 seconds on its own thread, and that thread is frozen with the rest of the environment; without the explicit flush, the `demo.invocations` counter is exported only if a later invocation reuses the environment long enough to trip the reader.

The flush pushes telemetry to the Collector layer on loopback, not to Dash0.
The remote round trip is the Collector extension's job, and the `decouple` processor in `collector-config/otel-collector-config.yaml` is what keeps that round trip outside the billed invocation.
This is why the SDK's `OTEL_EXPORTER_OTLP_ENDPOINT` is `http://localhost:4318` and never the Dash0 ingress.

## Why the function creates its own invocation span

`lambda_runtime` already wraps each invocation in a `tracing` span, but that span has kind INTERNAL and is entered before the handler runs.
Once a span has been entered, `tracing-opentelemetry` ignores a changed `otel.kind`, so recording `otel.kind = "server"` on it has no effect.
The function therefore creates its own `invocation` span inside the `service_fn` closure with `otel.kind = "server"`, which becomes the SERVER span that backends treat as the function's entry point.

The function also never calls `lambda_runtime::tracing::init_default_subscriber()`.
It installs its own global subscriber, and the `try_init()` in `main.rs` then fails, leaving the function without the OpenTelemetry layer.

## Resource detection order

`LambdaResourceDetector` from `opentelemetry-aws` sets `faas.name`, `faas.version`, `faas.max_memory`, `cloud.provider`, `cloud.platform`, and `cloud.region`; on this path nothing else sets them.
The resource builder merges detectors in order, with later values winning, so `main.rs` adds the Lambda detector first and the environment detectors after it.
That order keeps `OTEL_SERVICE_NAME` and `OTEL_RESOURCE_ATTRIBUTES` from the stack in charge of every attribute they set.

## The configuration layer lands at the archive root

The Collector reads its configuration from `/opt/otel-collector-config.yaml`.
AWS unpacks layers into `/opt`, so the YAML must sit at the root of the layer archive; nested one directory deeper, the Collector falls back to its built-in configuration and exports nothing, without logging an error.

The stack builds the layer with `lambda.Code.fromAsset('collector-config')`.
CDK zips the *contents* of the asset directory, so `collector-config/otel-collector-config.yaml` becomes `otel-collector-config.yaml` at the root of the zip.
Keep that directory flat: a subdirectory under `collector-config/` reproduces the silent-failure mode.

## How the auth token reaches the Collector

The Collector configuration reads the token as `${env:DASH0_AUTH_TOKEN}`, which means the token has to be an environment variable on the function.
The stack sets it from the Secrets Manager secret with `dash0TokenSecret.secretValue.unsafeUnwrap()`, which emits a CloudFormation dynamic reference that is resolved at deploy time.
The token appears in neither this repository nor the synthesized template, but after deployment it sits in the function's environment block, where anyone holding `lambda:GetFunctionConfiguration` can read it.
The [.NET project](../dotnet-upstream-layers/README.md#how-the-auth-token-reaches-the-collector) discusses the tradeoff and the alternatives; they apply here unchanged.

## Teardown

```bash
npx cdk destroy
```

This removes the function, the Function URL, the configuration layer, and the log group.
The Secrets Manager secret is referenced, not created, by this stack and is left alone.

## Related guidance

- [AWS Lambda deployment](../../../skills/otel-instrumentation/rules/platforms/aws-lambda.md)
- [AWS Lambda Collector layer](../../../skills/otel-collector/rules/deployment/aws-lambda.md)
- [Rust instrumentation](../../../skills/otel-instrumentation/rules/sdks/rust.md)

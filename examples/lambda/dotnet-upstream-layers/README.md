# .NET on AWS Lambda with the upstream OpenTelemetry Collector layer

## What this project demonstrates

A single AWS Lambda function written in .NET, instrumented with the OpenTelemetry .NET SDK in code, exporting OTLP over loopback to the upstream OpenTelemetry Collector Lambda layer, which ships the telemetry to Dash0.
Everything is provisioned with AWS CDK v2.

.NET takes this path because it has to.
The [AWS Lambda deployment](../../../skills/otel-instrumentation/rules/platforms/aws-lambda.md) rule routes Go and .NET to manual SDK instrumentation: there is no upstream auto-instrumentation layer for either runtime, so there is nothing for `AWS_LAMBDA_EXEC_WRAPPER` to point at.
The function therefore attaches only two layers — the Collector and its configuration — and owns its own provider setup, handler wrapping, and end-of-invocation flush.

The handler does a little CPU work, issues one outbound HTTPS request that fails DNS resolution on purpose, issues one that succeeds, and returns a small JSON body.
That produces an invocation span with a failed child span, a successful child span, and two custom spans underneath it.

## Project layout

| Path | Purpose |
|------|---------|
| `bin/app.ts` | CDK entry point; reads configuration from environment variables. |
| `lib/dotnet-upstream-layers-stack.ts` | The stack: layers, function, environment variables, Function URL. |
| `collector-config/otel-collector-config.yaml` | Collector configuration, packaged as its own layer. |
| `lambda/Dash0DotnetDemo/Function.cs` | The handler and all OpenTelemetry wiring. |

## Prerequisites

- Node.js 20 or newer, for the CDK app.
- The .NET SDK 8.0 or newer, for the function.
  The function targets `net8.0` and runs on Lambda's managed `dotnet8` runtime; a newer SDK builds it fine.
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

The .NET build is a separate, explicit step rather than Docker bundling inside the CDK stack.

```bash
npm install
npm run publish:function
```

That runs `dotnet publish -c Release -r linux-arm64 --self-contained false -o lambda/Dash0DotnetDemo/publish`, and the stack picks the output directory up with `lambda.Code.fromAsset`.
The stack checks for `lambda/Dash0DotnetDemo/publish/Dash0DotnetDemo.dll` and fails synthesis with a pointer to this section if the step was skipped.

The choice is deliberate.
`dotnet publish` for a framework-dependent `linux-arm64` target is pure IL plus managed dependencies, so it is fully cross-platform: the same command produces byte-identical output on macOS, Linux, and Windows, on any host architecture, with no Docker daemon in the loop.
Docker bundling would add a multi-gigabyte `dotnet/sdk` image pull, a container round trip on every synth, and a hard dependency on a running Docker daemon, in exchange for nothing this project needs.

## Deploy

```bash
npx cdk deploy
```

The stack emits three outputs: `FunctionUrl`, `FunctionName`, and `LogGroupName`.

## Invoke

```bash
curl "$(aws cloudformation describe-stacks \
  --stack-name Dash0DotnetUpstreamLayersStack \
  --query "Stacks[0].Outputs[?OutputKey=='FunctionUrl'].OutputValue" \
  --output text)"
```

The response is a small JSON body carrying the request id, the computed checksum, the trace id, and the outcome of both outbound probes — one `failed`, one `ok`.

The Function URL has auth type `NONE`, so the endpoint is public for as long as the stack exists.
Tear the stack down when the exercise is over.

## Verify

### In CloudWatch

The log group is `/aws/lambda/Dash0DotnetUpstreamLayersStack-dotnet-demo`, also emitted as the `LogGroupName` output.
Both the handler and the Collector extension write there.

```bash
aws logs tail /aws/lambda/Dash0DotnetUpstreamLayersStack-dotnet-demo --since 10m --follow
```

Look for, in this order:

1. `Registering OpenTelemetry collector extension` and `Starting OpenTelemetry collector`, written by the Collector layer during the cold start.
   The absence of any Collector line means the layer is attached but not loading, which is almost always a region or architecture mismatch.
2. `Everything is ready. Begin running and processing data.`, the Collector's own startup line, which confirms the configuration YAML parsed.
   If the Collector instead reports `error decoding 'exporters'` or starts with no pipelines, the configuration layer did not land where `OPENTELEMETRY_COLLECTOR_CONFIG_URI` points.
3. `demo.invocation.start request_id=TEST-...` and `demo.invocation.end request_id=TEST-...`, written by the handler.
4. No `Export failed` or `context deadline exceeded` lines from the Collector.
   Those mean the configuration is loading but the ingress endpoint, the dataset, or the auth token is wrong.

Invoke at least once after deploying: the extension only registers on a cold start.

### In Dash0

Filter on `service.name = dash0-dotnet-demo`.
A single invocation produces one trace containing:

- The invocation span created by `AWSLambdaWrapper`, carrying `faas.*` attributes and `cloud.region`.
- `demo.work` and `demo.cpu`, the two custom spans from `Function.cs`.
- Two `HTTP GET` client spans from the HttpClient instrumentation: one to `aws.amazon.com` with status `UNSET` and an HTTP 200, and one to `example.test` with status `ERROR` and a DNS resolution failure.

The resource carries `service.namespace=examples`, `service.version=0.1.0`, and `deployment.environment.name=development`, set through `OTEL_RESOURCE_ATTRIBUTES`.
The handler's CloudWatch lines also arrive as log records, because the Collector's `telemetryapi` receiver turns the Lambda Telemetry API stream into log records and feeds the logs pipeline.
The `demo.invocations` counter arrives as a metric.

## Where the end-of-invocation flush happens, and why

Lambda freezes the execution environment the moment the handler returns.
Anything still queued in a batch processor at that instant is not exported late — it is lost, and it is lost silently.
This is the one rule that makes a working long-lived-service setup wrong on Lambda, and it is the reason the guidance treats layers as mandatory for every runtime that has them: the layer's extension owns the export and runs after the handler returns.

On the manual path there is no instrumentation layer to own that, so the function does it itself.
The flush lives in `lambda/Dash0DotnetDemo/Function.cs`, in the `finally` block of `TracingFunctionHandlerAsync`:

```csharp
public async Task<APIGatewayHttpApiV2ProxyResponse> TracingFunctionHandlerAsync(
    APIGatewayHttpApiV2ProxyRequest request,
    ILambdaContext context)
{
    try
    {
        return await AWSLambdaWrapper.TraceAsync(TracerProvider, FunctionHandlerAsync, request, context);
    }
    finally
    {
        TracerProvider.ForceFlush(FlushTimeoutMilliseconds);
        MeterProvider.ForceFlush(FlushTimeoutMilliseconds);
    }
}
```

Three things are worth being precise about.

`AWSLambdaWrapper.TraceAsync` already force-flushes the `TracerProvider` when it ends the invocation span, so the trace half of this is belt and braces.
The `MeterProvider` flush is not: the wrapper knows nothing about metrics, and without that line the `demo.invocations` counter would only ever be exported if a later invocation happened to reuse the same execution environment and trip the periodic reader.
`ForceFlush` takes a timeout in milliseconds and returns a boolean; it is called in a `finally` so a failing handler still exports the spans that explain the failure.

The flush pushes telemetry to the Collector layer on loopback, not to Dash0.
The remote round trip is the Collector extension's job, and the `decouple` processor in `collector-config/otel-collector-config.yaml` is what keeps that round trip outside the billed invocation.
This is why the SDK's `OTEL_EXPORTER_OTLP_ENDPOINT` is `http://localhost:4318` and never the Dash0 ingress.

## The configuration layer lands at the archive root

The Collector reads its configuration from `/opt/otel-collector-config.yaml`.
AWS unpacks layers into `/opt`, so the YAML must sit at the root of the layer archive; nested one directory deeper, the Collector falls back to its built-in configuration and exports nothing, without logging an error.

The stack builds the layer with `lambda.Code.fromAsset('collector-config')`.
CDK zips the *contents* of the asset directory, so `collector-config/otel-collector-config.yaml` becomes `otel-collector-config.yaml` at the root of the zip.
That is verified, not assumed — after `npx cdk synth`:

```bash
ls cdk.out/asset.*/otel-collector-config.yaml
```

The staged asset directory contains exactly one file, `otel-collector-config.yaml`, with no path prefix.
Adding a subdirectory under `collector-config/` would reproduce the silent-failure mode, so keep that directory flat.

## How the auth token reaches the Collector

The Collector configuration reads the token as `${env:DASH0_AUTH_TOKEN}`, which means the token has to be an environment variable on the function.
The stack sets it from the Secrets Manager secret:

```typescript
DASH0_AUTH_TOKEN: dash0TokenSecret.secretValue.unsafeUnwrap(),
```

That emits a CloudFormation dynamic reference, `{{resolve:secretsmanager:<arn>:SecretString:::}}`, into the template.
CloudFormation resolves it at deploy time, so the token appears in neither this repository nor the synthesized template nor the CDK asset bundle.

The tradeoff is real and worth stating plainly: after deployment, the resolved token sits in the function's environment block, where anyone holding `lambda:GetFunctionConfiguration` can read it.
That is inherent to this path.
The Collector layer's only way to obtain a credential is an environment variable, and Lambda environment variables are resolved at deploy time, not at invocation time.
The alternatives are to accept it, to encrypt the environment with a customer-managed KMS key and restrict `kms:Decrypt`, or to leave this path entirely for the Dash0 Lambda Extension, which resolves `DASH0_TOKEN_SECRET_ARN` itself at runtime and never puts the token in the function configuration.

The function's execution role is also granted `secretsmanager:GetSecretValue` on the secret.
That grant is not what makes the dynamic reference work — CloudFormation resolves it with the deploying principal's permissions — but it is there so the example can be extended to fetch the secret at runtime.

## Why there is no `AWS_LAMBDA_EXEC_WRAPPER`

On the upstream auto-instrumentation path, `AWS_LAMBDA_EXEC_WRAPPER` is what activates the instrumentation layer, and leaving it unset means the layers are attached but inert.
There is no .NET instrumentation layer, so there is no wrapper script at `/opt` to point at, and the variable is correctly absent here.
The equivalent activation step on this path is the static constructor in `Function.cs` that builds the providers, plus the handler pointing at `TracingFunctionHandlerAsync` rather than at the inner handler.

`DisableAwsXRayContextExtraction = true` is set on `AddAWSLambdaConfigurations` for a related reason.
The function runs with X-Ray tracing mode `PassThrough`, as the guidance requires, and Lambda then sends a `_X_AMZN_TRACE_ID` header marked not-sampled.
Without that option the instrumentation honours the flag and drops every span.

## Teardown

```bash
npx cdk destroy
```

This removes the function, the Function URL, the configuration layer, and the log group.
The Secrets Manager secret is referenced, not created, by this stack and is left alone.

## Related guidance

- [AWS Lambda deployment](../../../skills/otel-instrumentation/rules/platforms/aws-lambda.md)
- [AWS Lambda Collector layer](../../../skills/otel-collector/rules/deployment/aws-lambda.md)
- [.NET instrumentation](../../../skills/otel-instrumentation/rules/sdks/dotnet.md)

# Node.js Lambda with the upstream OpenTelemetry layers

## What this demonstrates

This project deploys a single Node.js AWS Lambda function instrumented with the three upstream [opentelemetry-lambda](https://github.com/open-telemetry/opentelemetry-lambda) layers: the Node.js instrumentation layer, the Collector layer, and a configuration layer built from a CDK directory asset.
It is **path A** of the decision process in [`skills/otel-instrumentation/rules/platforms/aws-lambda.md`](../../../skills/otel-instrumentation/rules/platforms/aws-lambda.md), the path to use when the backend is not Dash0 or when telemetry has to fan out to more than one backend.
Here it is pointed at Dash0 over OTLP/gRPC so the two example projects can be compared side by side.

There is no OpenTelemetry SDK code in the handler.
The instrumentation layer exports to the Collector layer over loopback, and the Collector layer owns the upstream call, so the export round trip is not billed as function duration.

The handler burns a little CPU, makes one HTTPS request to `https://example.test/` that always fails DNS resolution, makes a second request to `https://aws.amazon.com/` that succeeds, and returns a small JSON body.

## Prerequisites

- Node.js 20 or newer and npm.
- AWS credentials for the target account, with permission to create Lambda functions and layer versions, IAM roles, CloudWatch log groups, and Lambda function URLs.
- The CDK bootstrap stack in the target account and region (`npx cdk bootstrap aws://<account>/<region>`).
- An existing AWS Secrets Manager secret in the same region whose value is a Dash0 auth token stored as a **plain string**, not as JSON.

## Environment variables

Export these before synthesizing or deploying.

```bash
export AWS_REGION=eu-west-1
export CDK_DEFAULT_REGION="$AWS_REGION"
export CDK_DEFAULT_ACCOUNT=<your account id>

# OTLP/gRPC ingress endpoint, with no scheme, from Dash0: Settings -> Endpoints.
export DASH0_OTLP_GRPC_ENDPOINT=ingress.eu-west-1.aws.dash0.com:4317

# ARN of the Secrets Manager secret holding the auth token as a plain string.
export DASH0_TOKEN_SECRET_ARN=arn:aws:secretsmanager:eu-west-1:<account>:secret:dash0-token-aBcDeF

# Optional; defaults to "default".
export DASH0_DATASET=default
```

All three values reach the Collector through the function's environment, and the configuration file reads them back with `${env:...}` placeholders.
That keeps the configuration layer free of any environment-specific value, so the same layer content works for every deployment.

## How the auth token reaches the Collector

The Collector layer can only read the auth token from its own process environment, through `${env:DASH0_AUTH_TOKEN}` in the configuration file.
There is no Secrets Manager integration on that side, so something has to put the plaintext token into the function's environment.

This stack uses CloudFormation's Secrets Manager dynamic reference:

```ts
DASH0_AUTH_TOKEN: cdk.SecretValue.secretsManager(props.dash0TokenSecretArn).unsafeUnwrap(),
```

The synthesized template carries only `{{resolve:secretsmanager:<arn>:SecretString:::}}`, so **the token is never written into this repository, into the template, or into the configuration layer**.
CloudFormation resolves it at deploy time.

The tradeoff is real and worth stating plainly: after deployment the resolved token is a plaintext environment variable on the function, readable by anyone who can call `lambda:GetFunctionConfiguration`.
That is the same exposure the skill's anti-patterns call out for `DASH0_TOKEN`, and the upstream Collector layer offers no way to avoid it.
Two mitigations are available, and neither is applied here in order to keep the example minimal.

1. Restrict `lambda:GetFunctionConfiguration` on this function to the deployment role.
2. Switch to the Dash0 Lambda Extension (path B, see the sibling `nodejs-dash0-extension` project), which resolves the secret itself at runtime from `DASH0_TOKEN_SECRET_ARN` and never holds the token in the function configuration.

If a plaintext environment variable is unacceptable and path B is not an option, the remaining choice is to fork the Collector layer with a custom build that includes an authentication extension capable of reading the token from elsewhere.

## The configuration layer

`collector-config-layer/otel-collector-config.yaml` is the entire content of the configuration layer.
CDK's `lambda.Code.fromAsset` on a directory zips the **contents** of that directory, so the YAML lands at the root of the archive, AWS unpacks it to `/opt/otel-collector-config.yaml`, and `OPENTELEMETRY_COLLECTOR_CONFIG_URI` finds it there.
A YAML nested one directory deep would unpack to `/opt/<subdir>/otel-collector-config.yaml`, where the Collector never looks: it falls back to its default configuration and exports nothing, without reporting an error.

The layout was verified on the synthesized asset:

```console
$ npx cdk synth --quiet
$ ls cdk.out/asset.*/
otel-collector-config.yaml
```

The staged asset directory contains the YAML and nothing else, with no path prefix, which is exactly what the archive root must look like.

## Deploy

```bash
npm install
npx cdk deploy
```

## Invoke

The stack prints the function URL as the `FunctionUrl` output.

```bash
curl -s "$(aws cloudformation describe-stacks \
  --stack-name TestNodejsUpstreamLayersStack \
  --query "Stacks[0].Outputs[?OutputKey=='FunctionUrl'].OutputValue" \
  --output text)"
```

Call it a few times, since the first call is a cold start and the layers only initialise on one.

## Verify

### CloudWatch

The log group is `/aws/lambda/TEST-nodejs-upstream-layers`, also printed as the `LogGroupName` output.

```bash
aws logs tail /aws/lambda/TEST-nodejs-upstream-layers --since 10m
```

Look for these lines.

- `EXTENSION  Name: collector` in the Lambda platform `extension` record of the first invocation.
  Its absence means the Collector layer is attached but not loading, which is almost always a region or architecture mismatch — this stack pins `arm64` on both the function and the Collector layer ARN.
- The Collector's own startup output, including `Everything is ready. Begin running and processing data.` and a line naming each configured pipeline.
  The Collector writes to the function's log group, interleaved with the function's logs.
- The handler's JSON lines, `invocation started` and `upstream calls completed`, the latter reporting `"ok": false` for `https://example.test/` and `"ok": true` for `https://aws.amazon.com/`.
- No `error` line from the exporter.
  A `401` or `403` from the exporter means the resolved auth token is wrong; `unknown type` means a component in the configuration does not exist in this Collector build.

To see the telemetry itself, add a `debug` exporter with `verbosity: detailed` to the traces pipeline in `collector-config-layer/otel-collector-config.yaml`, redeploy, and remove it afterwards — at that verbosity every span is written to CloudWatch Logs and billed per ingested byte.

### Dash0

In the dataset named by `DASH0_DATASET`, look for the following.

- A service named `lambda-telemetry-demo`, not `TEST-nodejs-upstream-layers`.
- A trace per invocation with a child client span for `https://example.test/` carrying span status `ERROR` and a child client span for `https://aws.amazon.com/` carrying status `UNSET`.
- Resource attributes `cloud.provider=aws`, `cloud.region`, `faas.name`, `faas.version`, `faas.instance`, `service.namespace=otel-examples`, and `deployment.environment.name=test`.
- Spans and log records produced by the `telemetryapi` receiver from Lambda platform events, alongside the ones the instrumentation layer produced.

If telemetry lands in the wrong dataset, the `Dash0-Dataset` header is missing or wrong.
The debug exporter shows telemetry data, not outgoing headers, so check the packaged configuration file and the `DASH0_DATASET` environment variable directly.

## Teardown

```bash
npx cdk destroy
```

The configuration layer version and the log group are both owned by this stack and are removed with it.
The Secrets Manager secret is not managed by this stack and is left alone.

## Notes on the implementation

The handler is CommonJS on purpose.
The upstream Node.js instrumentation layer patches `require()` directly and additionally registers an ESM loader hook, but that hook is newer and its coverage varies by layer and runtime version.
CommonJS keeps the example about the layers rather than about module systems.

The upstream Node.js layer sets up a tracer provider only.
Metrics and logs from the function process are not exported by it: the log records that reach Dash0 on this path come from the Collector's `telemetryapi` receiver, which reads the Lambda Telemetry API.

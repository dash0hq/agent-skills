# Node.js Lambda with the Dash0 Lambda Extension

## What this demonstrates

This project deploys a single Node.js AWS Lambda function instrumented with the [Dash0 Lambda Extension](https://github.com/dash0hq/dash0-lambda-extension), attached as one Lambda layer.
It is **path B** of the decision process in [`skills/otel-instrumentation/rules/platforms/aws-lambda.md`](../../../skills/otel-instrumentation/rules/platforms/aws-lambda.md): the runtime is Node.js and Dash0 is the only backend, so a single layer replaces the three layers of path A.
There is no OpenTelemetry SDK code in the handler and no Collector layer — attaching the upstream Collector layer alongside the extension would make two extensions compete over the same Lambda Telemetry API subscription.

The handler burns a little CPU, makes one HTTPS request to `https://example.test/` that always fails DNS resolution, makes a second request to `https://aws.amazon.com/` that succeeds, and returns a small JSON body.
That produces one invocation span with a failing and a succeeding HTTP client span beneath it.

## Prerequisites

- Node.js 20 or newer and npm.
- AWS credentials for the target account, with permission to create Lambda functions, IAM roles, CloudWatch log groups, and Lambda function URLs.
- The CDK bootstrap stack in the target account and region (`npx cdk bootstrap aws://<account>/<region>`).
- An existing AWS Secrets Manager secret in the same region whose value is a Dash0 auth token stored as a **plain string**, not as JSON.
  Create the token under [Settings → Auth Tokens](https://app.dash0.com/settings/auth-tokens).
  If the secret stores JSON instead, add `DASH0_TOKEN_SECRET_KEY` to the function environment in `lib/dash0-extension-stack.ts`.

## Environment variables

Export these before synthesizing or deploying.

```bash
export AWS_REGION=eu-west-1
export CDK_DEFAULT_REGION="$AWS_REGION"
export CDK_DEFAULT_ACCOUNT=<your account id>

# OTLP/HTTP ingress endpoint, from Dash0: Settings -> Endpoints.
export DASH0_ENDPOINT=https://ingress.eu-west-1.aws.dash0.com:4318

# ARN of the Secrets Manager secret holding the auth token as a plain string.
export DASH0_TOKEN_SECRET_ARN=arn:aws:secretsmanager:eu-west-1:<account>:secret:dash0-token-aBcDeF

# Optional; defaults to "default".
export DASH0_DATASET=default
```

The region matters beyond the usual CDK reasons: Lambda layers are region-scoped resources, and the extension layer ARN is built from the stack region.
A layer ARN naming a different region than the function fails the deployment with a `ResourceNotFoundException`.

## Deploy

```bash
npm install
npx cdk deploy
```

## Invoke

The stack prints the function URL as the `FunctionUrl` output.

```bash
curl -s "$(aws cloudformation describe-stacks \
  --stack-name TestNodejsDash0ExtensionStack \
  --query "Stacks[0].Outputs[?OutputKey=='FunctionUrl'].OutputValue" \
  --output text)"
```

The first call is a cold start, which is what the extension needs in order to register.
Call it a few times so there are both cold-start and warm invocations to compare.

## Verify

### CloudWatch

The log group is `/aws/lambda/TEST-nodejs-dash0-extension`, also printed as the `LogGroupName` output.

```bash
aws logs tail /aws/lambda/TEST-nodejs-dash0-extension --since 10m
```

Look for these lines.

- `EXTENSION  Name: dash0-extension` in the Lambda platform `extension` record of the first invocation.
  Its absence means the layer is attached but not loading, which is almost always a region or architecture mismatch.
- A `dash0` extension startup line naming the configured endpoint and dataset.
- The handler's own JSON lines, `invocation started` and `upstream calls completed`, the latter reporting `"ok": false` for `https://example.test/` and `"ok": true` for `https://aws.amazon.com/`.

If no extension line appears at all, set `DASH0_EXTENSION_LOG_LEVEL=debug` on the function temporarily and invoke again.

### Dash0

In the dataset named by `DASH0_DATASET`, look for the following.

- A service named `lambda-telemetry-demo`, not `TEST-nodejs-dash0-extension`.
  The function name arrives separately as `faas.name`, which is the point of setting `OTEL_SERVICE_NAME` explicitly.
- A trace per invocation whose root span is the Lambda invocation, with a child client span for `https://example.test/` carrying span status `ERROR` and a child client span for `https://aws.amazon.com/` carrying status `UNSET`.
- Resource attributes `cloud.provider=aws`, `cloud.region`, `faas.name`, `faas.version`, `faas.instance`, `service.namespace=otel-examples`, and `deployment.environment.name=test`.
- Log records from the handler's `console.log` calls, correlated with the invocation trace.

## Teardown

```bash
npx cdk destroy
```

The log group is created by this stack with a `DESTROY` removal policy, so it goes away with the stack.
The Secrets Manager secret is not managed by this stack and is left alone.

## Notes on the implementation

The handler is CommonJS on purpose.
Layer-based auto-instrumentation patches `require()` directly, whereas an ESM handler — a `.mjs` file, or a `.js` file under a `"type": "module"` package — depends on the layer's ESM loader hook, whose availability varies by layer and runtime version.
CommonJS keeps the example about the layer rather than about module systems.

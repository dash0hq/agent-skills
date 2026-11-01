# Python Lambda with the Dash0 Lambda Extension

## What this project demonstrates

This is a runnable AWS CDK project that instruments a Python AWS Lambda function with the [Dash0 Lambda Extension](https://github.com/dash0hq/dash0-lambda-extension) layer.
It is **path B** of the decision process in [AWS Lambda deployment](../../../skills/otel-instrumentation/rules/platforms/aws-lambda.md#decision-process): the runtime is Python and Dash0 is the only backend, so one layer replaces the three layers of path A.

There is deliberately no OpenTelemetry SDK code in the handler.
The entire instrumentation comes from the layer plus the environment variables on the function, which is exactly what the rule file prescribes.

The handler burns some CPU, makes one HTTPS request that fails DNS resolution on purpose, makes one HTTPS request that succeeds, makes one AWS SDK call, and returns a small JSON body.
That produces a handler span with both a failing and a succeeding child span underneath it.

## Prerequisites

- Node.js 20 or newer, and npm.
- An AWS account, with credentials in the environment or in a profile, and a region that is in the [list of regions the extension layer is published to](https://github.com/dash0hq/dash0-lambda-extension#supported-regions).
- The CDK bootstrap stack in that account and region: `npx cdk bootstrap aws://<account>/<region>`.
- A Dash0 organization, with an OTLP/HTTP endpoint from Settings → Endpoints and an auth token from Settings → Auth Tokens.
- An AWS Secrets Manager secret in the same account and region holding that auth token **as a plain string**, not as JSON.

Create the secret like this, replacing the placeholder token:

```bash
aws secretsmanager create-secret \
  --name TEST-dash0-auth-token \
  --secret-string "<your-dash0-auth-token>" \
  --query ARN --output text
```

If you store the token as a JSON object instead, you also have to set `DASH0_TOKEN_SECRET_KEY` on the function; this project assumes the plain-string form.

## Environment variables

Export these before synthesizing or deploying.

```bash
export AWS_REGION=eu-west-1
export CDK_DEFAULT_REGION=$AWS_REGION
export CDK_DEFAULT_ACCOUNT=$(aws sts get-caller-identity --query Account --output text)

export DASH0_ENDPOINT=https://ingress.eu-west-1.aws.dash0.com:4318
export DASH0_TOKEN_SECRET_ARN=arn:aws:secretsmanager:eu-west-1:<account>:secret:TEST-dash0-auth-token-aBcDeF
export DASH0_DATASET=default
```

`DASH0_DATASET` is optional and defaults to `default`.
`DASH0_ENDPOINT` and `DASH0_TOKEN_SECRET_ARN` are required, and the app fails with an explicit message naming the variable if either is missing.

## Deploy

```bash
npm install
npx cdk deploy
```

The stack creates the function, attaches the Dash0 extension layer, grants the execution role `secretsmanager:GetSecretValue` on the token secret, and exposes a Lambda Function URL with auth type `NONE`.

## Invoke

The stack outputs `FunctionUrl`.
Curl it:

```bash
curl -s "$(aws cloudformation describe-stacks \
  --stack-name PythonDash0ExtensionStack \
  --query "Stacks[0].Outputs[?OutputKey=='FunctionUrl'].OutputValue" \
  --output text)"
```

The response is a JSON body containing `TEST-0001 ok`, a checksum, and the outcome of each outbound call.
The call to `https://example.test/` is reported with `"ok": false` — that is intentional.

Invoke it a few times: the first invocation is a cold start, which is what initializes the layer.

## Verify

### CloudWatch

The stack outputs `LogGroupName`.
Tail it:

```bash
aws logs tail "$(aws cloudformation describe-stacks \
  --stack-name PythonDash0ExtensionStack \
  --query "Stacks[0].Outputs[?OutputKey=='LogGroupName'].OutputValue" \
  --output text)" --follow
```

Look for:

- `EXTENSION Name: dash0-extension` in the `INIT_START` platform line, which is the extension registering itself.
  No extension line at all means the layer is attached but not loading, usually a region or architecture mismatch.
- The handler's own `TEST-0001 invocation start` and `TEST-0001 invocation end` lines.
- `TEST-0001 expected failure: URLError` or a similar DNS error type from the `example.test` call.

If the extension line is absent, redeploy with `DASH0_EXTENSION_LOG_LEVEL=debug` added to the function's environment, invoke again, and read the log group again.
Remove it afterwards.

### Dash0

In Dash0, filter the dataset you deployed to and look for:

- A service named `lambda-example`, which is `OTEL_SERVICE_NAME`, not the function name.
  `faas.name` carries the function name separately.
- A trace per invocation whose root span is the handler invocation.
- A child span for the AWS STS call with `rpc.system=aws-api`.
- `faas.name`, `faas.version`, `faas.instance`, `cloud.provider=aws`, and `cloud.region` on the resource, all supplied by the platform and the extension rather than by this project.
- The resource attributes this project does set: `service.namespace=agent-skills-examples`, `service.version=0.1.0`, and `deployment.environment.name=test`.

## Known gaps

The handler's `urllib` calls do **not** produce HTTP client spans.
Neither the upstream Python layer nor, as far as this example observed, the Dash0 Python distribution instruments `urllib.request` from the standard library; the instrumented HTTP clients are `requests`, `httpx`, `aiohttp`, and the AWS SDK.
The AWS STS call in the handler exists so that there is always at least one real child span.
The `example.test` call still shows up as a failure in the handler's logs and in the invocation's duration.

## Teardown

```bash
npx cdk destroy
```

The Secrets Manager secret is not managed by this stack, so delete it separately when you are done.

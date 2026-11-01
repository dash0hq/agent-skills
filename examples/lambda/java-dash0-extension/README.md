# Java Lambda with the Dash0 Lambda Extension

## What this project demonstrates

This project deploys a Java 21 AWS Lambda function instrumented by the [Dash0 Lambda Extension](https://github.com/dash0hq/dash0-lambda-extension), attached as a single layer.
It is **path B** of the decision process in [AWS Lambda deployment](../../../skills/otel-instrumentation/rules/platforms/aws-lambda.md#decision-process): the runtime is Java and Dash0 is the only backend, so one layer replaces the three layers of path A.

There is no OpenTelemetry SDK code in the handler.
The only changes relative to a plain Lambda function are the layer and the environment block, which is exactly what the rule file promises.
The handler has two dependencies: `aws-lambda-java-core` and the JDK HTTP client.

The handler burns a few million iterations of CPU, makes one HTTPS request to `https://example.test/` that fails DNS resolution on purpose, makes one HTTPS request to `https://aws.amazon.com/` that succeeds, and returns a small JSON body.
The failing request is the point: it produces an error span next to a successful one in the same trace.

## Prerequisites

- Node.js 20 or newer, and npm.
- Docker, running.
  The Java artifact is built inside `public.ecr.aws/sam/build-java21`, so no local JDK or Maven installation is needed.
  The first synth pulls that image, which is roughly two gigabytes.
- AWS credentials for the target account, and a CDK-bootstrapped environment (`npx cdk bootstrap`).
- An existing AWS Secrets Manager secret in the same region, holding the Dash0 auth token as a **plain string** (not a JSON object).
  If the secret is a JSON object, set `DASH0_TOKEN_SECRET_KEY` on the function as well — see the environment variable table in the rule file.

If you prefer to build with a local toolchain instead of Docker, you need JDK 21 and Maven 3.9 or newer, and you have to replace the `bundling` block in `lib/java-dash0-extension-stack.ts` with `lambda.Code.fromAsset('lambda/target/...')` plus a documented `mvn package` step.
The Docker route is the default because it needs nothing installed and produces the same artifact on every machine.

## Configuration

```bash
export AWS_REGION=eu-west-1
export CDK_DEFAULT_REGION="$AWS_REGION"
export CDK_DEFAULT_ACCOUNT="$(aws sts get-caller-identity --query Account --output text)"

# OTLP/HTTP ingress endpoint for your Dash0 organization.
# Settings -> Endpoints in the Dash0 app.
export DASH0_ENDPOINT='https://ingress.eu-west-1.aws.dash0.com:4318'

# ARN of the Secrets Manager secret holding the auth token as a plain string.
export DASH0_TOKEN_SECRET_ARN='arn:aws:secretsmanager:eu-west-1:111122223333:secret:TEST-dash0-token-AbCdEf'

# Optional; defaults to "default".
export DASH0_DATASET=default
```

`bin/app.ts` fails with a named error if `DASH0_ENDPOINT` or `DASH0_TOKEN_SECRET_ARN` is missing or empty.

The token itself is never read by this project and never appears in any file here.
Only the secret ARN goes into the template; the extension reads the secret at startup using the function's execution role, which the stack grants `secretsmanager:GetSecretValue` on that secret.

## Deploy

```bash
npm install
npx cdk deploy
```

## How to invoke

The stack creates a Lambda Function URL with auth type `NONE` and emits it as the `FunctionUrl` output.

```bash
curl "$(aws cloudformation describe-stacks \
  --stack-name JavaDash0ExtensionStack \
  --query "Stacks[0].Outputs[?OutputKey=='FunctionUrl'].OutputValue" \
  --output text)"
```

The response is a JSON body with `testCaseId`, `checksum`, and the outcome of both HTTPS requests.
The `unreachable` entry reports `"ok": false` with a DNS error — that is the intended result, not a failure of the deployment.

Invoke at least twice.
The first invocation is a cold start, which is what the extension needs in order to register.

## How to verify

### CloudWatch

The log group is emitted as the `LogGroupName` output.

```bash
aws logs tail "$(aws cloudformation describe-stacks \
  --stack-name JavaDash0ExtensionStack \
  --query "Stacks[0].Outputs[?OutputKey=='LogGroupName'].OutputValue" \
  --output text)" --since 10m
```

Look for:

- `EXTENSION  Name: dash0_lambda_extension` in the `INIT_START`/platform lines, which proves the layer loaded.
  The absence of any extension line means the layer is attached but not loading, which is almost always a region or architecture mismatch.
- The handler's own lines: `TEST-0001 invocation start`, `TEST-0001 fetch failed url=https://example.test/`, `TEST-0001 fetch ok url=https://aws.amazon.com/ status=200`, and `TEST-0001 invocation end`.
- `REPORT ... Init Duration:` on the first invocation, confirming a cold start happened.

If nothing from the extension appears, set `DASH0_EXTENSION_LOG_LEVEL=debug` on the function temporarily, invoke again, and remove it afterwards.

### Dash0

In the dataset you configured, look for:

- A service named `lambda-telemetry-sample` — not the Lambda function name.
  `OTEL_SERVICE_NAME` is set explicitly so that several functions of one logical service do not split into several services.
- Traces with a root span for the invocation, carrying `faas.name`, `faas.instance`, `cloud.provider=aws`, and `cloud.region`.
  None of those are set by this project; the platform and the extension supply them.
- Resource attributes `service.namespace=agent-skills-examples`, `service.version=1.0.0`, and `deployment.environment.name=test`.
- A child span for the `https://example.test/` call with `ERROR` status, and a child span for the `https://aws.amazon.com/` call with `UNSET` status and `http.response.status_code=200`.
- Logs correlated with the invocation trace ID, including the `TEST-0001` lines.
- Metrics `faas.invoke_duration`, `faas.init_duration`, `dash0.faas.billed_duration`, and `faas.mem_usage`, which the extension emits without any configuration.

## Teardown

```bash
npx cdk destroy
```

The Secrets Manager secret is referenced, not created, so it survives the teardown.

## Notes on memory and timeout

The function is configured with 1024 MB and a 30 second timeout.
A Java cold start on Lambda has to start a JVM, let the extension register and read the secret, and then run the handler, which makes two outbound TLS handshakes.
Lambda allocates vCPU in proportion to memory, so 1024 MB is also what keeps JIT warm-up and the TLS handshakes from dominating the invocation.
At 512 MB the cold start is roughly twice as long and the 30 second timeout stops being comfortable.

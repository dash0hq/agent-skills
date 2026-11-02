---
title: 'AWS Lambda with the Dash0 Lambda Extension'
impact: HIGH
tags:
  - aws
  - lambda
  - serverless
  - deployment
  - layers
  - dash0
---

# AWS Lambda with the Dash0 Lambda Extension

This is path B of [AWS Lambda deployment](./aws-lambda.md): a single layer that combines auto-instrumentation with an exporter shipping directly to Dash0.
Read the [decision process](./aws-lambda.md#decision-process) first — path B is the right choice when the runtime is Node.js, Python, or Java **and** Dash0 is the only backend.

The [Dash0 Lambda Extension](https://github.com/dash0hq/dash0-lambda-extension) replaces all three layers of path A.
Do not attach the upstream Collector layer alongside it: both subscribe to the Lambda Telemetry API and the result is duplicated or dropped telemetry.

When telemetry must fan out to more than one backend, or the runtime is Ruby, use [upstream OpenTelemetry layers](./aws-lambda-otel-layers.md) instead.

Auto-instrumentation supports Node.js, Python, and Java, on both `x86_64` and `arm64`.
The `dash0-extension-manual` layer works with any runtime: it carries no instrumentation, only the exporter, the Lambda lifecycle handling, and the extension's own event-derived enrichment.

ZIP-packaged functions attach the layer; container-image functions copy its contents into the image, see [container images](#container-images).

## Layer ARN

The extension is published from the Dash0 AWS account `115813213817`:

<!-- eval:skip -->
```
arn:aws:lambda:<region>:115813213817:layer:dash0-extension-node:<version>
arn:aws:lambda:<region>:115813213817:layer:dash0-extension-python:<version>
arn:aws:lambda:<region>:115813213817:layer:dash0-extension-java:<version>
arn:aws:lambda:<region>:115813213817:layer:dash0-extension-manual:<version>
```

Use the `manual` layer when the function is instrumented with the OpenTelemetry SDK in code — Go and .NET, or any runtime where you want to own the instrumentation — and only needs the extension's exporter and Lambda lifecycle handling.
See [runtimes without an auto-instrumentation layer](./aws-lambda.md#runtimes-without-an-auto-instrumentation-layer) for the in-code setup that pairs with it.

Layers are region-scoped: the ARN must name the same region as the function.

Resolve `<version>` from the [dash0-lambda-extension releases](https://github.com/dash0hq/dash0-lambda-extension/releases) rather than copying a version out of any documentation, including this file.

## Environment variables

| Variable | Required | Value |
|----------|:--------:|-------|
| `DASH0_ENDPOINT` | Yes | The OTLP/HTTP ingress endpoint, for example `https://ingress.eu-west-1.aws.dash0.com:4318` |
| `DASH0_DATASET` | No | The dataset slug telemetry is routed to; when unset the extension sends no `Dash0-Dataset` header and telemetry lands in the default dataset. Set it whenever the organization uses more than one dataset |
| `DASH0_TOKEN_SECRET_ARN` | One of | ARN of the AWS Secrets Manager secret holding the auth token |
| `DASH0_TOKEN_SECRET_KEY` | Conditional | The JSON key inside that secret; omit when the secret is a plain string. The AWS console's default "key/value" secret form produces JSON, so a secret created that way needs this variable |
| `DASH0_TOKEN` | One of | The auth token as a literal value |
| `AWS_LAMBDA_EXEC_WRAPPER` | For traces | `/opt/wrapper` |

Use `DASH0_TOKEN_SECRET_ARN` and grant the function's execution role `secretsmanager:GetSecretValue` on that secret.
The extension resolves it at runtime, so the token never enters the function configuration — this is the main operational advantage over [path A](./aws-lambda-otel-layers.md#the-auth-token-on-path-a), where the token must be an environment variable.

Reserve `DASH0_TOKEN` exclusively for throwaway experiments: a literal token in the function configuration is readable by anyone with `lambda:GetFunctionConfiguration`, and it ends up in the IaC source and in CloudTrail.
When both are set, `DASH0_TOKEN` wins.

Set the service identity attributes as on any other path, with `OTEL_SERVICE_NAME` and `OTEL_RESOURCE_ATTRIBUTES` — see [resource attributes on Lambda](./aws-lambda.md#resource-attributes-on-lambda).

Find the endpoint under [Settings → Endpoints](https://app.dash0.com/goto/settings/endpoints?endpoint_type=otlp_http) and the token under [Settings → Auth Tokens](https://app.dash0.com/settings/auth-tokens).

Optional variables:

| Variable | Default | Effect |
|----------|---------|--------|
| `DASH0_SEND_ON_INVOCATION_END` | `true` | Exports before the invocation completes. Set `false` to export on the **next** invocation instead, which cuts billed duration but can delay telemetry by up to 7 minutes when no further invocation follows |
| `DASH0_EXTENSION_LOG_LEVEL` | `warn` | One of `trace`, `debug`, `info`, `warn`, `error` |
| `DASH0_DISTRO_DEBUG` | `false` | Emits extra auto-instrumentation diagnostics |
| `DASH0_DISABLE_AUTO_INSTRUMENTATION` | `false` | Disables auto-instrumentation; the extension still emits a synthetic span per invocation |
| `DASH0_DISABLE_TELEMETRY_TRACES` | `false` | Disables auto-instrumentation **and** the synthetic spans — stronger than the variable above |
| `DASH0_DISABLE_TELEMETRY_METRICS` | `false` | Drops the supplementary FaaS metrics (`faas.invoke_duration`, `faas.init_duration`, `dash0.faas.billed_duration`, `faas.mem_usage`) |
| `DASH0_CREATE_PAYLOAD_LOG_RECORDS` | `true` | Emits log records carrying the invocation's request and response payloads, and those of each client call. Set `false` to stop capturing payloads entirely |
| `DASH0_MASK_RULES` | — | JSON array of regular expressions applied to all payloads. Setting it **replaces** the defaults, so restate them |
| `DASH0_MASK_ENV_VARS` | — | JSON array of regular expressions matching environment variables to mask in telemetry. Falls back to `DASH0_MASK_RULES` |

`DASH0_SEND_ON_INVOCATION_END` is the one knob here with a real tradeoff: the default keeps telemetry timely at the cost of billed duration, and `false` moves the export off the current invocation at the cost of delaying the last invocation's telemetry indefinitely.
Leave it at the default unless billed duration is a measured problem.

Every environment variable is captured into telemetry, including `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN`.
If you override `DASH0_MASK_RULES` or `DASH0_MASK_ENV_VARS`, keep `.*key.*`, `.*secret.*`, and `.*token.*` in the list, or those credentials are exported unmasked.
Write patterns as `.*name.*` rather than `name`, because Python matches from the start of the key.

Per-payload overrides — `DASH0_MASK_REQUEST_BODY`, `DASH0_MASK_RESPONSE_BODY`, `DASH0_MASK_REQUEST_HEADERS`, `DASH0_MASK_RESPONSE_HEADERS`, and `DASH0_MASK_QUERY_PARAMS` — take the same JSON array or the literal `all`, and their runtime support varies: Node.js covers all of them, Python covers some, and Java covers none.

The `DASH0_MASK_*` variables are a backstop, not the primary control.
Keep secrets out of telemetry at the source per [sensitive data](../sensitive-data.md).

## Instrumentation coverage

The extension's distro enables its own instrumentation set, which is not the same as the upstream layer's.
Notably, it enables the undici instrumentation by default, so Node.js `fetch` calls are traced without extra configuration — the same handler behaves differently on the two paths.

There is no per-instrumentation allowlist or denylist — nothing equivalent to `OTEL_NODE_ENABLED_INSTRUMENTATIONS` or `OTEL_PYTHON_DISABLED_INSTRUMENTATIONS` on [path A](./aws-lambda-otel-layers.md#selecting-instrumentations-by-runtime).
The two coarse switches are `DASH0_DISABLE_AUTO_INSTRUMENTATION=true`, which turns auto-instrumentation off while the extension still emits a synthetic span per invocation, and `DASH0_DISABLE_TELEMETRY_TRACES=true`, which suppresses both.

To trace a library the distro does not cover, add a manual span, or instrument in code and attach `dash0-extension-manual` instead.

## Collection modes

| Mode | `AWS_LAMBDA_EXEC_WRAPPER` | `DASH0_DISABLE_TELEMETRY_LOG_COLLECTION` | Collected |
|------|---------------------------|:----------------------------------------:|-----------|
| Traces and logs | `/opt/wrapper` | unset (default `false`) | Traces, logs, metrics |
| Traces only | `/opt/wrapper` | `true` | Traces, metrics |
| Logs only | unset | unset | Logs, metrics |

Two independent switches select the mode: `AWS_LAMBDA_EXEC_WRAPPER` turns auto-instrumentation on, and `DASH0_DISABLE_TELEMETRY_LOG_COLLECTION` turns Lambda Telemetry API log collection off.

In logs-only mode the extension binary in `/opt/extensions/` reads the Lambda Telemetry API directly, so logs arrive without any auto-instrumentation.

## Preconditions

Set the function's X-Ray tracing mode to `PassThrough`.
`Active` makes the X-Ray SDK compete with OpenTelemetry for the trace context and produces broken traces.

When `Active` is required for other reasons, set `DASH0_XRAY_TRACES_ENABLED=true` so the extension preserves the X-Ray context instead of fighting it.
Prefer `PassThrough` — reach for this only when something else in the account depends on Active tracing.

Remove any other vendor layer first — Datadog, New Relic, Dynatrace, Lumigo, ADOT, and the upstream OpenTelemetry layers all conflict with the Dash0 extension.

## Infrastructure as code

The layer list and the environment block are the only changes needed; the handler code is untouched.

### AWS SAM

<!-- eval:skip -->
```yaml
Resources:
  CheckoutFunction:
    Type: AWS::Serverless::Function
    Properties:
      Runtime: nodejs22.x
      Architectures: [arm64]
      Handler: index.handler
      Tracing: PassThrough
      Layers:
        - !Sub 'arn:aws:lambda:${AWS::Region}:115813213817:layer:dash0-extension-node:26'
      Environment:
        Variables:
          AWS_LAMBDA_EXEC_WRAPPER: /opt/wrapper
          DASH0_ENDPOINT: https://ingress.eu-west-1.aws.dash0.com:4318
          DASH0_DATASET: production
          DASH0_TOKEN_SECRET_ARN: !Ref Dash0TokenSecret
          OTEL_SERVICE_NAME: checkout
          OTEL_RESOURCE_ATTRIBUTES: service.namespace=shop,deployment.environment.name=production
      Policies:
        - AWSSecretsManagerGetSecretValuePolicy:
            SecretArn: !Ref Dash0TokenSecret
```

### Serverless Framework

The [`serverless-dash0`](https://github.com/dash0hq/dash0-lambda-extension) plugin attaches the layer and sets the environment variables for every function flagged with `dash0-traced: true`.

<!-- eval:skip -->
```yaml
plugins:
  - serverless-dash0

custom:
  dash0:
    endpoint: https://ingress.eu-west-1.aws.dash0.com:4318
    dataset: production
    tokenSecretArn: ${env:DASH0_TOKEN_SECRET_ARN}

functions:
  checkout:
    handler: index.handler
    tags:
      dash0-traced: true
```

### Terraform

<!-- eval:skip -->
```hcl
resource "aws_lambda_function" "checkout" {
  function_name = "checkout"
  runtime       = "nodejs22.x"
  architectures = ["arm64"]
  handler       = "index.handler"
  role          = aws_iam_role.checkout.arn

  layers = [
    "arn:aws:lambda:${var.region}:115813213817:layer:dash0-extension-node:26",
  ]

  tracing_config {
    mode = "PassThrough"
  }

  environment {
    variables = {
      AWS_LAMBDA_EXEC_WRAPPER  = "/opt/wrapper"
      DASH0_ENDPOINT           = "https://ingress.eu-west-1.aws.dash0.com:4318"
      DASH0_DATASET            = "production"
      DASH0_TOKEN_SECRET_ARN   = aws_secretsmanager_secret.dash0_token.arn
      OTEL_SERVICE_NAME        = "checkout"
      OTEL_RESOURCE_ATTRIBUTES = "service.namespace=shop,deployment.environment.name=production"
    }
  }
}
```

### AWS CDK

<!-- eval:fragment -->
```typescript
const dash0Layer = lambda.LayerVersion.fromLayerVersionArn(
  this,
  'Dash0Layer',
  `arn:aws:lambda:${this.region}:115813213817:layer:dash0-extension-node:26`,
);

const checkout = new lambda.Function(this, 'Checkout', {
  runtime: lambda.Runtime.NODEJS_22_X,
  architecture: lambda.Architecture.ARM_64,
  handler: 'index.handler',
  code: lambda.Code.fromAsset('src'),
  tracing: lambda.Tracing.PASS_THROUGH,
  layers: [dash0Layer],
  environment: {
    AWS_LAMBDA_EXEC_WRAPPER: '/opt/wrapper',
    DASH0_ENDPOINT: 'https://ingress.eu-west-1.aws.dash0.com:4318',
    DASH0_DATASET: 'production',
    DASH0_TOKEN_SECRET_ARN: dash0Token.secretArn,
    OTEL_SERVICE_NAME: 'checkout',
    OTEL_RESOURCE_ATTRIBUTES: 'service.namespace=shop,deployment.environment.name=production',
  },
});

dash0Token.grantRead(checkout);
```

### AWS CLI

```bash
aws lambda update-function-configuration \
  --function-name checkout \
  --layers "arn:aws:lambda:eu-west-1:115813213817:layer:dash0-extension-node:26" \
  --environment "Variables={AWS_LAMBDA_EXEC_WRAPPER=/opt/wrapper,DASH0_ENDPOINT=https://ingress.eu-west-1.aws.dash0.com:4318,DASH0_DATASET=production,DASH0_TOKEN_SECRET_ARN=$DASH0_TOKEN_SECRET_ARN,OTEL_SERVICE_NAME=checkout}"
```

`update-function-configuration` replaces the entire layer list and the entire environment block.
Read the current configuration with `aws lambda get-function-configuration` and merge, or the call silently drops layers and variables the function already had.

### Container images

Dash0 publishes an extension image per runtime, so a container-image function copies `/opt` out of it instead of attaching the layer:

<!-- eval:skip -->
```dockerfile
FROM public.ecr.aws/lambda/nodejs:20

COPY --from=public.ecr.aws/r1r2x3i6/extension-node:latest /opt /opt

ENV AWS_LAMBDA_EXEC_WRAPPER=/opt/wrapper
ENV DASH0_ENDPOINT=https://ingress.eu-west-1.aws.dash0.com:4318
ENV DASH0_DATASET=production

COPY index.js ${LAMBDA_TASK_ROOT}/
CMD ["index.handler"]
```

The image tags are `extension-node`, `extension-python`, and `extension-java`.
Supply `DASH0_TOKEN_SECRET_ARN` through the function configuration rather than baking a token into the image.

## Validation

Follow the [shared validation steps](./aws-lambda.md#validation).
When step 2 shows no extension line in the log group, set `DASH0_EXTENSION_LOG_LEVEL=debug` temporarily, invoke again, and remove it afterwards.

## Anti-patterns

- **Attaching the layer but leaving `AWS_LAMBDA_EXEC_WRAPPER` unset.**
  The extension collects logs only, and the absence of traces reads as a broken layer.
- **Setting `DASH0_TOKEN` as a literal in the function configuration or IaC.**
  The value is readable through `lambda:GetFunctionConfiguration` and is committed to source control.
  Use `DASH0_TOKEN_SECRET_ARN` with an AWS Secrets Manager secret.
- **Pointing `DASH0_TOKEN_SECRET_ARN` at a JSON secret without `DASH0_TOKEN_SECRET_KEY`.**
  The AWS console's default secret form produces JSON, so the extension reads the whole JSON document as the token and every export is rejected with a 401.
- **Attaching the upstream Collector layer as well.**
  Both subscribe to the Lambda Telemetry API; pick one path.
- **Leaving X-Ray tracing mode on `Active`.**
  The X-Ray SDK and OpenTelemetry compete for the trace context; set `PassThrough`, or `DASH0_XRAY_TRACES_ENABLED=true` when `Active` is mandatory.
- **Using a layer ARN whose region differs from the function's region.**
  Lambda layers are region-scoped; the function fails to update with a `ResourceNotFoundException`.
- **Copying a layer version out of documentation.**
  Layer versions move independently of the docs that mention them.
  Resolve the current version from the releases page at the time you write the configuration.

## References

- [AWS Lambda deployment](./aws-lambda.md)
- [AWS Lambda with upstream OpenTelemetry layers](./aws-lambda-otel-layers.md)
- [dash0hq/dash0-lambda-extension](https://github.com/dash0hq/dash0-lambda-extension)
- [Dash0 AWS Lambda setup](https://www.dash0.com/docs/dash0/infrastructure/aws/aws-lambda/setup)

# AWS Lambda instrumentation examples

Runnable AWS CDK projects that exercise the AWS Lambda guidance in [`skills/otel-instrumentation/rules/platforms/aws-lambda.md`](../../skills/otel-instrumentation/rules/platforms/aws-lambda.md) and [`skills/otel-collector/rules/deployment/aws-lambda.md`](../../skills/otel-collector/rules/deployment/aws-lambda.md).

These exist to test the guidance against reality.
Each project follows one path of the decision process end to end, so a wrong wrapper path, a stale layer version, or a broken Collector configuration shows up as a function that emits nothing rather than as a documentation bug nobody notices.

This directory is excluded from the published skill artifacts through `.tesslignore` and `.tileignore`.

## Projects

| Project | Runtime | Path | What it covers |
|---------|---------|------|----------------|
| [`nodejs-dash0-extension`](./nodejs-dash0-extension) | Node.js | B — Dash0 Lambda Extension | Single layer, `/opt/wrapper`, token from Secrets Manager |
| [`nodejs-upstream-layers`](./nodejs-upstream-layers) | Node.js | A — upstream OTel layers | Instrumentation, Collector, and configuration layers, `/opt/otel-handler` |
| [`python-dash0-extension`](./python-dash0-extension) | Python | B — Dash0 Lambda Extension | Single layer, `/opt/wrapper` |
| [`python-upstream-layers`](./python-upstream-layers) | Python | A — upstream OTel layers | The Python-specific wrapper `/opt/otel-instrument` |
| [`java-dash0-extension`](./java-dash0-extension) | Java | B — Dash0 Lambda Extension | Single layer, cold-start sizing |
| [`java-upstream-layers`](./java-upstream-layers) | Java | A — upstream OTel layers | The `javawrapper` versus `javaagent` cold-start tradeoff |
| [`dotnet-upstream-layers`](./dotnet-upstream-layers) | .NET | Manual SDK + Collector layer | No auto-instrumentation layer exists; in-code SDK with an end-of-invocation flush |

Each project is self-contained.
Run `npm install` and `npx cdk deploy` from inside its own directory; nothing depends on this directory or on the other projects.

For the full run — credentials, the Secrets Manager secret, the pass criteria, and teardown — follow "Running the AWS Lambda examples as a live eval" in [CONTRIBUTING.md](../../CONTRIBUTING.md).
The layer versions these projects pin go stale; refresh them as described under "Keeping pinned upstream versions current" in the same file before treating a failure as a guidance bug.

## Shared conventions

Every project is a CDK v2 app written in TypeScript, regardless of the Lambda runtime, so the stack code reads the same across all seven.

Configuration comes from environment variables, never from committed values:

| Variable | Used by | Meaning |
|----------|---------|---------|
| `DASH0_ENDPOINT` | Dash0 extension projects | OTLP/HTTP ingress endpoint |
| `DASH0_OTLP_GRPC_ENDPOINT` | Upstream-layer projects | OTLP/gRPC ingress endpoint |
| `DASH0_DATASET` | All | Dataset slug, defaults to `default` |
| `DASH0_TOKEN_SECRET_ARN` | All | ARN of an AWS Secrets Manager secret holding the auth token |

Find the endpoints under [Settings → Endpoints](https://app.dash0.com/goto/settings/endpoints?endpoint_type=otlp_grpc) and create a token under [Settings → Auth Tokens](https://app.dash0.com/settings/auth-tokens).

Every function exposes a Lambda Function URL so it can be exercised with a single `curl`, and every handler makes one failing and one succeeding outbound HTTPS request so the resulting trace has both an error span and a successful child span.

## Cost and safety

These projects create real AWS resources: a Lambda function, a Function URL with no authentication, a CloudWatch log group, and one or more Lambda layers.
The Function URL is public by design so the examples are easy to exercise — do not leave a stack deployed longer than the test needs.

Run `npx cdk destroy` in each project directory when you are finished.

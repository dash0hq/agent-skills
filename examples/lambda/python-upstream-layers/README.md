# Python Lambda with the upstream OpenTelemetry layers

## What this project demonstrates

This is a runnable AWS CDK project that instruments a Python AWS Lambda function with the three upstream [opentelemetry-lambda](https://github.com/open-telemetry/opentelemetry-lambda) layers: the Python instrumentation layer, the Collector layer, and a configuration layer built from a CDK directory asset.
It is **path A** of the decision process in [AWS Lambda deployment](../../../skills/otel-instrumentation/rules/platforms/aws-lambda.md#decision-process), and the Collector configuration follows [AWS Lambda Collector layer](../../../skills/otel-collector/rules/deployment/aws-lambda.md).

There is deliberately no OpenTelemetry SDK code in the handler.
Setting `AWS_LAMBDA_EXEC_WRAPPER` is what activates auto-instrumentation; without it the layers are attached but inert.

Python uses `/opt/otel-instrument`, not the `/opt/otel-handler` value that Node.js, Java, and Ruby use.
As of `layer-python/0.22.0` the layer also ships an `/opt/otel-handler` shim that forwards to `otel-instrument`, so both values work for Python, but `/opt/otel-instrument` is the one the rule file prescribes and the one this project sets.

The handler burns some CPU, makes one HTTPS request that fails DNS resolution on purpose, makes one HTTPS request that succeeds, makes one AWS SDK call, and returns a small JSON body.

## Prerequisites

- Node.js 20 or newer, and npm.
- An AWS account, with credentials in the environment or in a profile, and a region where the upstream layers are published.
- The CDK bootstrap stack in that account and region: `npx cdk bootstrap aws://<account>/<region>`.
- A Dash0 organization, with an OTLP/gRPC endpoint from Settings → Endpoints and an auth token from Settings → Auth Tokens.
- An AWS Secrets Manager secret in the same account and region holding that auth token as a plain string.

## Environment variables

```bash
export AWS_REGION=eu-west-1
export CDK_DEFAULT_REGION=$AWS_REGION
export CDK_DEFAULT_ACCOUNT=$(aws sts get-caller-identity --query Account --output text)

export DASH0_OTLP_GRPC_ENDPOINT=ingress.eu-west-1.aws.dash0.com:4317
export DASH0_TOKEN_SECRET_ARN=arn:aws:secretsmanager:eu-west-1:<account>:secret:TEST-dash0-auth-token-aBcDeF
export DASH0_DATASET=default
```

`DASH0_OTLP_GRPC_ENDPOINT` is host and port with no scheme, because the Collector's `otlp` exporter takes a bare endpoint and uses TLS by default.
`DASH0_DATASET` is optional and defaults to `default`.

## How the auth token reaches the Collector

The Collector configuration references the token as `${env:DASH0_AUTH_TOKEN}`, which the Collector expands from the environment of the Lambda execution environment.
That means the token has to be a function environment variable; there is no way for the upstream Collector layer to read Secrets Manager itself.

This project resolves the secret with a CloudFormation dynamic reference, `{{resolve:secretsmanager:<arn>:SecretString:::}}`.
The tradeoff is explicit:

- The literal token never appears in this repository, in the CDK source, or in the synthesized CloudFormation template.
  Inspect `cdk.out/PythonUpstreamLayersStack.template.json` after a synth and you will see only the `{{resolve:...}}` string.
- CloudFormation resolves it at deploy time, so the token **does** end up in plaintext in the function's configuration, readable by anyone with `lambda:GetFunctionConfiguration`.

The rule file's warning against literal tokens is about writing them into source and IaC, which this avoids.
It does not have a mechanism that keeps the token out of the function configuration entirely, because the upstream Collector layer has none.
Path B, the Dash0 Lambda Extension, does: the extension reads the secret from Secrets Manager at runtime, so only the ARN is ever in the configuration.
If the plaintext token in the function configuration is unacceptable, use path B.

## The configuration layer asset layout

The YAML must sit at the **root** of the layer archive.
AWS unpacks layers into `/opt`, so a file zipped inside a subdirectory lands at `/opt/<subdir>/otel-collector-config.yaml` and the Collector silently falls back to its default configuration without reporting an error.

The stack generates the configuration into `build/collector-config/` and passes that directory to `lambda.Code.fromAsset`.
CDK zips the **contents** of an asset directory, so a directory holding exactly one file produces an archive with that file at the root.
This was verified on the synthesized asset: `cdk.out/asset.<hash>/` contains `otel-collector-config.yaml` and nothing else, and zipping that directory's contents produces a listing with `otel-collector-config.yaml` and no path prefix.

## Deploy

```bash
npm install
npx cdk deploy
```

## Invoke

```bash
curl -s "$(aws cloudformation describe-stacks \
  --stack-name PythonUpstreamLayersStack \
  --query "Stacks[0].Outputs[?OutputKey=='FunctionUrl'].OutputValue" \
  --output text)"
```

Invoke it a few times; the first invocation is a cold start, which is what initializes the layers.

## Verify

### CloudWatch

```bash
aws logs tail "$(aws cloudformation describe-stacks \
  --stack-name PythonUpstreamLayersStack \
  --query "Stacks[0].Outputs[?OutputKey=='LogGroupName'].OutputValue" \
  --output text)" --follow
```

Look for:

- `EXTENSION Name: collector` in the `INIT_START` platform line.
  No extension line means the Collector layer is attached but not loading, usually a region or architecture mismatch.
- A Collector startup line such as `Everything is ready. Begin running and processing data.`
  The Collector's own output goes to the function's log group, interleaved with the handler's.
- Lines naming the configuration path, confirming the Collector read `/opt/otel-collector-config.yaml` rather than falling back to its default.
  A fallback shows up as a Collector that starts with no `otlp/dash0` exporter and exports nothing.
- The handler's `TEST-0001 invocation start` and `TEST-0001 invocation end` lines.

If the Collector does not start at all, the most likely cause is a configuration error.
Add a `debug` exporter to the traces pipeline in `renderCollectorConfig` in `lib/python-upstream-layers-stack.ts` temporarily, redeploy, and read the log group again.
Remove it before leaving the stack running: at `detailed` verbosity it writes every span to CloudWatch Logs, which is billed per ingested byte.

### Dash0

In Dash0, filter the dataset you deployed to and look for:

- A service named `lambda-example`, which is `OTEL_SERVICE_NAME`.
- A trace per invocation with the handler span as the root.
- A child span for the AWS STS call.
- Log records from the function, which arrive through the `telemetryapi` receiver rather than through the instrumentation layer.
- `faas.name`, `faas.version`, `faas.instance`, `cloud.provider=aws`, and `cloud.region` on the resource, which the layer's wrapper script sets from the Lambda reserved environment variables.

A missing or incorrect `Dash0-Dataset` header puts telemetry in the default dataset instead.
The debug exporter shows telemetry data, not outgoing headers, so check the generated `build/collector-config/otel-collector-config.yaml` directly.

## Known gaps

The handler's `urllib` calls do **not** produce HTTP client spans.
The upstream Python layer's instrumentation set covers `requests`, `httpx`, `aiohttp`, Django, Flask, FastAPI, the database drivers, and the AWS SDK, but not `urllib` or `urllib3` from the standard library; `urllib3` is explicitly removed from the layer build.
The AWS STS call in the handler exists so that there is always at least one real child span without adding a `pip install` step.

This gap is closable, unlike the equivalent one on Node.js.
The layer runs `opentelemetry-instrument`, which discovers instrumentors through the `opentelemetry_instrumentor` entry point group across the whole of `sys.path`, and `/var/task` is on `sys.path`.
Installing the instrumentation into the deployment package is therefore enough:

```bash
pip install --target ./lambda --no-deps "opentelemetry-instrumentation-urllib==0.66b1"
```

Verified against the deployed function on 2026-11-01: the two `urllib.request.urlopen` calls then produce client spans from the `opentelemetry.instrumentation.urllib` scope, with no change to `handler.py`.
Pin the version to the one in the layer release's `requirements.txt` and pass `--no-deps`, or `pip` installs a second copy of the API, SDK, and semantic conventions into `/var/task`, which shadows `/opt/python` and breaks the layer.
This project deliberately leaves the gap open so that the default layer behaviour stays visible.

## Teardown

```bash
npx cdk destroy
```

The Secrets Manager secret is not managed by this stack, so delete it separately when you are done.

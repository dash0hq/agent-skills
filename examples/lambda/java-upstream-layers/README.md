# Java Lambda with the upstream OpenTelemetry layers

## What this project demonstrates

This project deploys a Java 21 AWS Lambda function instrumented by the upstream [opentelemetry-lambda](https://github.com/open-telemetry/opentelemetry-lambda) layers: the Java instrumentation layer, the Collector layer, and a configuration layer holding the Collector YAML.
It is **path A** of the decision process in [AWS Lambda deployment](../../../skills/otel-instrumentation/rules/platforms/aws-lambda.md#decision-process), the path to take when the backend is not Dash0 or when telemetry has to fan out to more than one backend.
This example still exports to Dash0, so that path A and path B can be compared side by side.

There is no OpenTelemetry SDK code in the handler.
The handler has two dependencies: `aws-lambda-java-core` and the JDK HTTP client.
It burns a few million iterations of CPU, makes one HTTPS request to `https://example.test/` that fails DNS resolution on purpose, makes one HTTPS request to `https://aws.amazon.com/` that succeeds, and returns a small JSON body.

## Which Java layer this uses, and how to switch

The rule file gives two Java instrumentation layers and states a preference:

> Use `opentelemetry-javawrapper` by default: the `opentelemetry-javaagent` layer loads the full Java agent and adds seconds to cold starts, which usually requires provisioned concurrency to be acceptable in production.

This project follows that preference and attaches `opentelemetry-javawrapper-0_22_0`.

**There is a consequence the rule file does not mention.**
The wrapper layer instruments the Lambda invocation itself and the AWS SDK, and nothing else.
The [upstream Java README](https://github.com/open-telemetry/opentelemetry-lambda/blob/main/java/README.md) is explicit: "For any other library, such as OkHttp, you will need to include the corresponding library instrumentation [...] and modify your code to initialize it."
The `javaagent` layer, by contrast, "will be automatically loaded and instrument your application for all supported libraries."

So with the default `javawrapper` layer this function produces a **single invocation span** and no child spans for the two outbound HTTPS calls.
The error from `https://example.test/` is caught in the handler and never surfaces as a span, because the JDK HTTP client is not instrumented.
To see the outbound calls as child spans, switch to the agent.

To switch, change one constant at the top of `lib/java-upstream-layers-stack.ts`:

```typescript
const JAVA_INSTRUMENTATION_LAYER: 'javawrapper' | 'javaagent' = 'javaagent';
```

Then redeploy.
Nothing else changes: both layers publish from the same release, carry the same version string, and both are activated by `AWS_LAMBDA_EXEC_WRAPPER=/opt/otel-handler`.
Expect the first cold start after the switch to take several seconds longer.

The practical rule is the one the upstream README states and this rule file omits: pick `javawrapper` when the invocation span is enough or when the function only talks to AWS services, and pick `javaagent` when you need HTTP, JDBC, or messaging client spans and can absorb the cold start.

## Prerequisites

- Node.js 20 or newer, and npm.
- Docker, running.
  The Java artifact is built inside `public.ecr.aws/sam/build-java21`, so no local JDK or Maven installation is needed.
  The first synth pulls that image, which is roughly two gigabytes.
- AWS credentials for the target account, and a CDK-bootstrapped environment (`npx cdk bootstrap`).
- An existing AWS Secrets Manager secret in the same region, holding the Dash0 auth token as a plain string.

If you prefer a local toolchain, you need JDK 21 and Maven 3.9 or newer, and you have to replace the `bundling` block in `lib/java-upstream-layers-stack.ts` with a path to a pre-built artifact plus a documented `mvn package` step.
The Docker route is the default because it needs nothing installed and produces the same artifact on every machine.

## Configuration

```bash
export AWS_REGION=eu-west-1
export CDK_DEFAULT_REGION="$AWS_REGION"
export CDK_DEFAULT_ACCOUNT="$(aws sts get-caller-identity --query Account --output text)"

# OTLP/gRPC ingress endpoint: host and port, no scheme.
export DASH0_OTLP_GRPC_ENDPOINT='ingress.eu-west-1.aws.dash0.com:4317'

# ARN of the Secrets Manager secret holding the auth token as a plain string.
export DASH0_TOKEN_SECRET_ARN='arn:aws:secretsmanager:eu-west-1:111122223333:secret:TEST-dash0-token-AbCdEf'

# Optional; defaults to "default".
export DASH0_DATASET=default
```

`bin/app.ts` fails with a named error if `DASH0_OTLP_GRPC_ENDPOINT` or `DASH0_TOKEN_SECRET_ARN` is missing or empty.

## How the auth token reaches the Collector

The Collector configuration references the token as `${env:DASH0_AUTH_TOKEN}`, so the value has to be an environment variable on the function.
Unlike the Dash0 Lambda Extension, the Collector layer has no Secrets Manager integration — it cannot resolve a secret ARN itself.

This stack resolves the secret with a CloudFormation dynamic reference:

```typescript
const dash0AuthToken = tokenSecret.secretValue.unsafeUnwrap();
```

That puts the literal string `{{resolve:secretsmanager:<arn>:SecretString:::}}` into the template and lets CloudFormation substitute the real value at deploy time.

The tradeoff, stated plainly:

- The token is **not** in this repository, **not** in `cdk.out`, and **not** in the synthesised template.
  Inspect `cdk.out/JavaUpstreamLayersStack.template.json` and you will find only the ARN-shaped resolve string.
- The token **is** readable afterwards through `lambda:GetFunctionConfiguration`, because CloudFormation writes the resolved plaintext into the function's environment block.
  The instrumentation rule file names exactly this exposure as the reason to prefer `DASH0_TOKEN_SECRET_ARN` over `DASH0_TOKEN` — but that advice only applies to path B.
  On path A there is no secret-ARN mechanism, so the exposure is unavoidable as long as the token lives in an environment variable.
- Restrict `lambda:GetFunctionConfiguration` on this function, and rotate the token on the usual schedule.

The alternative — passing the token through a CDK context value or a plain environment variable read in `bin/app.ts` — is strictly worse: it puts the plaintext in your shell history and in `cdk.context.json`, on top of the same function-configuration exposure.
That is why this project does not do it.

## The configuration layer lands at the root of the archive

The rule file is explicit that a YAML file zipped inside a subdirectory ends up at `/opt/<subdir>/otel-collector-config.yaml`, where the Collector does not find it, and that the Collector then falls back to its default configuration without reporting an error.

CDK zips the *contents* of the asset directory, not the directory itself.
`collector-config/` contains exactly one file, so the archive contains `otel-collector-config.yaml` with no path prefix.

This has been verified against the synthesised asset, and you can verify it yourself after a synth.
CDK stages the layer asset as a directory and zips it at publish time, so the check at synth time is on the staged directory:

```bash
npx cdk synth > /dev/null

# The layer asset is the one that is not the function code asset.
find cdk.out -maxdepth 2 -name 'otel-collector-config.yaml'
```

The file is reported directly under `cdk.out/asset.<hash>/`, with no intermediate directory, which is what puts it at the root of the published archive and therefore at `/opt/otel-collector-config.yaml`.
After a deploy you can confirm the published archive the same way the rule file prescribes:

```bash
aws lambda get-layer-version-by-arn --arn "<the config layer ARN>" \
  --query 'Content.Location' --output text | xargs curl -s -o /tmp/config-layer.zip
unzip -l /tmp/config-layer.zip
```

The listing must show `otel-collector-config.yaml` with no path prefix.

One note on layer ordering: the stack passes the layers as instrumentation, Collector, configuration, and CloudFormation renders them in a different order.
That is harmless here, because the three layers write disjoint sets of files into `/opt` — ordering only matters when two layers ship the same path.

## Deploy

```bash
npm install
npx cdk deploy
```

## How to invoke

```bash
curl "$(aws cloudformation describe-stacks \
  --stack-name JavaUpstreamLayersStack \
  --query "Stacks[0].Outputs[?OutputKey=='FunctionUrl'].OutputValue" \
  --output text)"
```

Invoke at least twice.
The first invocation is a cold start, which is what the layers need in order to initialise.

## How to verify

### CloudWatch

```bash
aws logs tail "$(aws cloudformation describe-stacks \
  --stack-name JavaUpstreamLayersStack \
  --query "Stacks[0].Outputs[?OutputKey=='LogGroupName'].OutputValue" \
  --output text)" --since 10m
```

Look for:

- `EXTENSION  Name: collector` in the platform lines, which proves the Collector layer loaded.
- Collector startup lines such as `Everything is ready. Begin running and processing data.`
  The Collector writes to the function's log group, interleaved with the function's own logs.
- `[collector] Reading config from <...>` or an equivalent line naming `/opt/otel-collector-config.yaml`.
  If the Collector reports that it is using a default configuration, the configuration layer is not where the Collector expects it.
- The absence of `Error: failed to get config` or `unknown type:` lines.
  An `unknown type:` line names a component that this Collector build does not have.
- The handler's own lines: `TEST-0001 invocation start`, `TEST-0001 fetch failed url=https://example.test/`, `TEST-0001 fetch ok url=https://aws.amazon.com/ status=200`, and `TEST-0001 invocation end`.

To see what the Collector is actually exporting, add `debug` to the traces pipeline in `collector-config/otel-collector-config.yaml`, redeploy, invoke, and read the log group.
Remove it afterwards: at `detailed` verbosity it writes every span to CloudWatch Logs, which is billed per ingested byte.

### Dash0

Look for:

- A service named `lambda-telemetry-sample`.
- One span per invocation, carrying `faas.name`, `faas.instance`, `cloud.provider=aws`, and `cloud.region`.
- Resource attributes `service.namespace=agent-skills-examples`, `service.version=1.0.0`, and `deployment.environment.name=test`.
- Telemetry in the dataset named by `DASH0_DATASET`.
  A missing or incorrect `Dash0-Dataset` header silently routes everything to the default dataset, and the debug exporter will not reveal it because it shows telemetry, not outgoing headers.
- Log records produced by the `telemetryapi` receiver, including the `TEST-0001` lines and the platform `REPORT` records.

With the default `javawrapper` layer, do **not** expect child spans for the two HTTPS calls.
Switch to `javaagent` as described above to get them.

## Teardown

```bash
npx cdk destroy
```

This also deletes the configuration layer, which the stack creates.
The Secrets Manager secret is referenced, not created, so it survives.

## Notes on memory and timeout

The function is configured with 1024 MB and a 30 second timeout.
A Java cold start on Lambda has to start a JVM, let the Collector extension register and parse its configuration, and then run the handler, which makes two outbound TLS handshakes.
Lambda allocates vCPU in proportion to memory, so 1024 MB is also what keeps JIT warm-up from dominating the invocation.
Switching to the `javaagent` layer makes this matter considerably more — the agent adds seconds to the cold start, and 1024 MB with 30 seconds is close to the minimum at which that stays safe without provisioned concurrency.

---
title: 'AWS Lambda with upstream OpenTelemetry layers'
impact: HIGH
tags:
  - aws
  - lambda
  - serverless
  - deployment
  - layers
---

# AWS Lambda with upstream OpenTelemetry layers

This is path A of [AWS Lambda deployment](./aws-lambda.md): three layers attached to the function, no code changes, telemetry exported by a Collector running as a Lambda extension.
Read the [decision process](./aws-lambda.md#decision-process) first — path A is the right choice when the runtime is Node.js, Python, Java, or Ruby and the backend is anything other than Dash0 alone, or telemetry must reach more than one backend.

For the Dash0-specific single-layer alternative, see [AWS Lambda with the Dash0 Lambda Extension](./aws-lambda-dash0-layers.md).
For Go and .NET, which have no instrumentation layer, see [runtimes without an auto-instrumentation layer](./aws-lambda.md#runtimes-without-an-auto-instrumentation-layer).

## The three layers

Attach the language instrumentation layer, the Collector layer, and a configuration layer holding the Collector configuration.
The order they appear in the function's layer list does not matter, because the three unpack into `/opt` without colliding on any filename.

Layers are region-scoped resources.
Every ARN must name the same region as the function, and the Collector layer must match the function's architecture.

### Layer ARNs

The upstream layers are published from AWS account `184161586896`:

<!-- eval:skip -->
```
# Instrumentation layer — pick one runtime
arn:aws:lambda:<region>:184161586896:layer:opentelemetry-nodejs-<version>:1
arn:aws:lambda:<region>:184161586896:layer:opentelemetry-python-<version>:1
arn:aws:lambda:<region>:184161586896:layer:opentelemetry-javaagent-<version>:1
arn:aws:lambda:<region>:184161586896:layer:opentelemetry-javawrapper-<version>:1
arn:aws:lambda:<region>:184161586896:layer:opentelemetry-ruby-dev-<version>:1

# Collector layer — match the function architecture
arn:aws:lambda:<region>:184161586896:layer:opentelemetry-collector-amd64-<version>:1
arn:aws:lambda:<region>:184161586896:layer:opentelemetry-collector-arm64-<version>:1
```

**Write `<version>` with underscores, not dots.**
Release `0.24.0` is `0_24_0` in the layer name, so the collector ARN reads `opentelemetry-collector-arm64-0_24_0:1`.
Substituting the release string literally produces a layer that does not exist, and the function update fails with `ResourceNotFoundException`.

The trailing `:1` is the Lambda layer version, which is distinct from the release version already encoded in the name.
Upstream publishes each release as layer version `1`, so `:1` is correct unless a layer was republished in that region.

Resolve `<version>` from the [opentelemetry-lambda releases](https://github.com/open-telemetry/opentelemetry-lambda/releases) rather than copying a version out of any documentation, including this file.

### Choosing between the two Java layers

Java has two instrumentation layers, and the choice is a tradeoff.

| | `opentelemetry-javaagent` | `opentelemetry-javawrapper` |
|---|---|---|
| Instruments | Every library the OpenTelemetry Java agent supports — HTTP clients, JDBC, messaging, and so on | The invocation and the AWS SDK, nothing else |
| Cold start | Seconds; generally needs provisioned concurrency in production | Much lower |

Choose by what the function calls:

- The function only calls AWS services, or an invocation span alone is enough — use `opentelemetry-javawrapper`.
- The function makes HTTP, JDBC, or messaging calls you need as child spans — use `opentelemetry-javaagent` and expect slow cold starts.

With the wrapper, any other client library produces no spans unless you add its instrumentation as a dependency and initialize it in code.
A function on the wrapper that calls OkHttp or a JDBC driver yields a single invocation span, which reads as broken auto-instrumentation but is the documented behavior.

Both layers use `AWS_LAMBDA_EXEC_WRAPPER=/opt/otel-handler`.
The wrapper layer is published from the `layer-javaagent/<version>` release — there is no `layer-javawrapper` release tag to search for.

**The `javawrapper` layer requires `aws-lambda-java-events` on the function classpath.**
Its `TracingRequestWrapper` calls `extractHeaders()`, which references `APIGatewayProxyRequestEvent`, so a function built with only `aws-lambda-java-core` fails every invocation with `java.lang.NoClassDefFoundError` and returns a 502.
The failure is at invocation time, not deploy time, and the stack trace names an API Gateway class even when the trigger is a Function URL, which makes it read like a trigger misconfiguration.

```xml
<dependency>
  <groupId>com.amazonaws</groupId>
  <artifactId>aws-lambda-java-events</artifactId>
  <version>3.11.4</version>
</dependency>
```

## Environment variables

| Variable | Value |
|----------|-------|
| `AWS_LAMBDA_EXEC_WRAPPER` | Node.js: `/opt/otel-handler`, Python: `/opt/otel-instrument`, Java (both layers): `/opt/otel-handler`, Ruby: `/opt/otel-handler` |
| `OPENTELEMETRY_COLLECTOR_CONFIG_URI` | `/opt/otel-collector-config.yaml` (configuration layer), `/var/task/otel-collector-config.yaml` (bundled in the deployment package), or `s3://<bucket>.s3.<region>.amazonaws.com/<key>` |
| `OTEL_SERVICE_NAME` | The logical service name — see [resource attributes on Lambda](./aws-lambda.md#resource-attributes-on-lambda) |
| `OTEL_RESOURCE_ATTRIBUTES` | `service.namespace`, `service.version`, and `deployment.environment.name` |
| `OTEL_TRACES_SAMPLER` | `always_on` — sample in the Collector, not the SDK |

Setting `AWS_LAMBDA_EXEC_WRAPPER` is what activates auto-instrumentation.
Without it the layers are attached but inert, and the function emits nothing.

Write `/opt/otel-instrument` for new Python functions.
Current Python layers also ship `/opt/otel-handler` as a shim that execs `otel-instrument`, and upstream intends to converge on `otel-handler`, so an existing Python function using `/opt/otel-handler` is working and does not need "fixing".

Using the `s3://` form requires `s3:GetObject` on that object in the function's execution role.

The Collector configuration referenced by `OPENTELEMETRY_COLLECTOR_CONFIG_URI` may name further environment variables through `${env:...}` placeholders.
The Collector expands those from its own process environment, which Lambda populates from this same environment block, so every name used in the configuration must be set on the function — see [the auth token on path A](#the-auth-token-on-path-a).

## What the instrumentation layers actually instrument

An instrumentation layer covers the libraries bundled into it, not every outbound call the function makes.
A call made with a library the layer does not instrument produces no client span, which reads as "the layer is not working" when it is working as designed.

The cases that bite most often:

- **Python:** `requests`, `httpx`, `aiohttp`, `botocore`, the web frameworks, and the common database drivers are instrumented.
  `urllib` is not, and the layer build explicitly removes `urllib3`.
  A handler that calls `urllib.request.urlopen` gets an invocation span with no HTTP children; a `boto3` call in the same handler does produce a client span.
  This one is fixable by adding a dependency: see [tuning the Python instrumentation set](#tuning-the-python-instrumentation-set).
- **Node.js:** the global `fetch` API is not traced out of the box.
  The undici instrumentation that handles `fetch` is bundled in the layer but is not enabled by default, while `net` is — so a `fetch` call yields `tcp.connect` and `tls.connect` internal spans and **no HTTP client span**, which looks like partial instrumentation rather than a missing one.
  Fix it with an environment variable, not a dependency: see [tuning the Node.js instrumentation set](#tuning-the-nodejs-instrumentation-set).
- **Node.js, module format:** auto-instrumentation hooks `require()`, so CommonJS handlers work directly.
  ESM handlers depend on the layer's loader hook, which is a recent addition — an ESM handler on an older layer produces the invocation span and no children.
- **Java, `javawrapper` layer:** only the invocation and the AWS SDK, as described above.

When a call must be traced and the layer does not cover its library, either switch to a covered library or add a manual span.

## Selecting instrumentations by runtime

Each runtime resolves its instrumentation set differently, so the mechanism for turning one on or off is not portable between them.
Pick the row for the runtime, then follow the matching section below.

| Runtime | Default set | Turn one off | Turn one on | Add one the layer does not ship |
|---|---|---|---|---|
| Node.js | Subset of what is bundled | `OTEL_NODE_DISABLED_INSTRUMENTATIONS` | `OTEL_NODE_ENABLED_INSTRUMENTATIONS` (allowlist, replaces the defaults) | `global.configureInstrumentations` hook; a package added to the function bundle is ignored |
| Python | Every bundled package | `OTEL_PYTHON_DISABLED_INSTRUMENTATIONS` | Nothing to turn on — bundled means enabled | `pip install` it into the deployment package; it is discovered through entry points |
| Java, `javaagent` | Every instrumentation in the agent | `OTEL_INSTRUMENTATION_<NAME>_ENABLED=false` | `OTEL_INSTRUMENTATION_<NAME>_ENABLED=true` | Not applicable — the agent covers the library or it does not |
| Java, `javawrapper` | Invocation and AWS SDK only | Not configurable | Not configurable | Add the library instrumentation as a function dependency and wire it in code, or switch to `javaagent` |
| Ruby | Every instrumentation in `opentelemetry-instrumentation-all` | `OTEL_RUBY_INSTRUMENTATION_<NAME>_ENABLED=false` | `OTEL_RUBY_INSTRUMENTATION_<NAME>_ENABLED=true` | Not applicable — the layer calls `use_all` over its own bundle |

The decisive difference is **Python versus Node.js**.
On Python the layer only puts `/opt/python` on `PYTHONPATH` and runs `opentelemetry-instrument`, which discovers instrumentors through setuptools entry points across the whole path — so `/var/task` counts and a package installed into the deployment bundle is picked up.
On Node.js the layer loads the instrumentation copies it bundles and never looks at the function's `node_modules`, so installing an instrumentation package into the function has no effect at all.

Go and .NET have no instrumentation layer, so there is nothing to switch: the instrumentation set is exactly the list registered in code — see [runtimes without an auto-instrumentation layer](./aws-lambda.md#runtimes-without-an-auto-instrumentation-layer).

### Tuning the Node.js instrumentation set

The upstream Node.js layer bundles roughly 30 instrumentations but enables only a subset by default: `dns`, `express`, `graphql`, `grpc`, `hapi`, `http`, `ioredis`, `koa`, `mongodb`, `mysql`, `net`, `pg`, and `redis`, plus the AWS SDK and Lambda instrumentations, which are always on.

Everything else — `undici`, `kafkajs`, `amqplib`, `cassandra-driver`, `dataloader`, `fs`, `knex`, `memcached`, `mongoose`, `mysql2`, `nestjs-core`, `pino`, `bunyan`, `winston`, `restify`, `socket.io`, `connect` — ships in the layer but stays off until you ask for it.
No extra dependency is needed; these are environment-variable switches.

| Variable | Effect |
|----------|--------|
| `OTEL_NODE_ENABLED_INSTRUMENTATIONS` | **Allowlist.** Replaces the default set entirely — only what you name is loaded |
| `OTEL_NODE_DISABLED_INSTRUMENTATIONS` | **Denylist.** Starts from the default set and removes what you name |

Both take a comma-separated list of package names with the `@opentelemetry/instrumentation-` prefix stripped.

There is no "add to the defaults" variable.
To keep the defaults and add one instrumentation, you must spell out the whole allowlist:

```bash
# Trace fetch and plain HTTP, and drop the low-level net spans.
OTEL_NODE_ENABLED_INSTRUMENTATIONS="http,undici,aws-sdk"

# Or keep every default and only remove the noisy ones.
OTEL_NODE_DISABLED_INSTRUMENTATIONS="net,dns,fs"
```

Prefer the allowlist on Lambda.
Each enabled instrumentation is loaded and patched during init, so a short list measurably shortens cold starts, and `net`, `dns`, and `fs` generate high-volume low-value spans that cost money to store.

For a library the layer does not bundle at all, assign the layer's global configuration hook before the handler loads:

<!-- eval:fragment -->
```js
global.configureInstrumentations = () => [
  new MyCustomInstrumentation(),
];
```

The layer also exposes `configureTracer`, `configureMeter`, `configureLogger`, `configureSdkRegistration`, `configureLambdaInstrumentation`, and `configureAwsInstrumentation` as the same kind of global hook.

### Tuning the Python instrumentation set

The upstream Python layer installs a fixed list of instrumentation packages into `/opt/python` and enables every one of them.
The list covers `aiohttp-client`, `asgi`, `asyncpg`, `boto3sqs`, `botocore`, `celery`, `dbapi`, `django`, `fastapi`, `falcon`, `flask`, `grpc`, `jinja2`, `mysql`, `psycopg2`, `pymemcache`, `pymongo`, `pymysql`, `pyramid`, `redis`, `requests`, `sqlalchemy`, `sqlite3`, `starlette`, `tornado`, and `wsgi`.
There is no enable switch, because there is nothing bundled-but-off.

Turn one off with `OTEL_PYTHON_DISABLED_INSTRUMENTATIONS`:

```bash
# Comma-separated ENTRY POINT names, not package names.
OTEL_PYTHON_DISABLED_INSTRUMENTATIONS="django,sqlite3"

# `*` disables every instrumentation while leaving the SDK and the
# invocation span in place.
OTEL_PYTHON_DISABLED_INSTRUMENTATIONS="*"
```

The names are the entry point names, which are not always the package name minus the prefix: `opentelemetry-instrumentation-grpc` registers `grpc_client` and `grpc_server`, and `opentelemetry-instrumentation-kafka-python` registers `kafka`.
Read the entry point name out of the package rather than guessing it:

```bash
python3 -c "from importlib.metadata import entry_points; print([e.name for e in entry_points(group='opentelemetry_instrumentor')])"
```

To instrument a library the layer does not ship, install the instrumentation package **into the function's deployment package**.
`opentelemetry-instrument` discovers instrumentors through the `opentelemetry_instrumentor` entry point group across the whole of `sys.path`, and `/var/task` is on `sys.path`, so a package installed next to the handler is loaded exactly like a bundled one.

```bash
# Pin the SAME version as the layer's instrumentation packages -- read it from
# the layer release's requirements.txt. Use --no-deps so pip does not pull a
# second, conflicting copy of the API, SDK, or semantic conventions.
pip install --target ./build --no-deps "opentelemetry-instrumentation-urllib==0.66b1"
```

This is the fix for the `urllib` gap: with that package in the bundle, `urllib.request.urlopen` produces client spans from the `opentelemetry.instrumentation.urllib` scope with no code change to the handler.

### Tuning the Java instrumentation set

The two Java layers behave completely differently here, so check which one is attached first.

The `javaagent` layer carries the full OpenTelemetry Java agent, and every instrumentation has an individual switch.
The system property is `otel.instrumentation.<name>.enabled`; the environment variable is the same name uppercased with dots and dashes replaced by underscores:

```bash
# Turn one instrumentation off.
OTEL_INSTRUMENTATION_JDBC_ENABLED=false

# Opt-in-only mode: nothing is instrumented except what you name.
OTEL_INSTRUMENTATION_COMMON_DEFAULT_ENABLED=false
OTEL_INSTRUMENTATION_AWS_SDK_ENABLED=true
OTEL_INSTRUMENTATION_AWS_LAMBDA_ENABLED=true
OTEL_INSTRUMENTATION_OPENTELEMETRY_API_ENABLED=true
OTEL_INSTRUMENTATION_OPENTELEMETRY_INSTRUMENTATION_ANNOTATIONS_ENABLED=true
```

Opt-in-only mode is the lever that makes the `javaagent` layer viable on Lambda: the agent's cold-start cost is dominated by scanning and matching classes for instrumentations you do not use.
Set `OTEL_INSTRUMENTATION_COMMON_DEFAULT_ENABLED=false` and re-enable only the libraries the function actually calls.
Keep `opentelemetry-api` and `opentelemetry-instrumentation-annotations` enabled whenever the function also has manual instrumentation, or `@WithSpan` and `Span.current()` stop working.
Some instrumentations depend on others, so verify in a backend after narrowing the list rather than assuming the result.

The `javawrapper` layer has **no instrumentation switches**, because it is not an agent.
It is a plain classpath layer carrying the `aws-lambda-events` library instrumentation plus the AWS SDK autoconfiguration, and it instruments the invocation and AWS SDK calls only.
To trace anything else on `javawrapper`, add the matching `io.opentelemetry.instrumentation` library artifact to the function's own dependencies and wire it in code, or switch the function to the `javaagent` layer and accept the cold start.

### Tuning the Ruby instrumentation set

The upstream Ruby layer calls `OpenTelemetry::SDK.configure { |c| c.use_all }`, so every instrumentation in `opentelemetry-instrumentation-all` is on.
Each one has an individual switch named after its gem:

```bash
# The layer sets this one to false itself, because instrumenting Net::HTTP
# also instruments the Lambda runtime's own control-plane calls and floods
# the trace with runtime noise. Set it to true only if the handler makes
# Net::HTTP calls you need traced, and expect that noise.
OTEL_RUBY_INSTRUMENTATION_NET_HTTP_ENABLED=true

# Turn others off individually.
OTEL_RUBY_INSTRUMENTATION_RACK_ENABLED=false
```

The layer also preloads the `require`s it finds in the handler file before configuring the SDK, so instrumentation only attaches to libraries required at the top level of the handler file.
A library required lazily inside a function body is loaded after `use_all` has run and is not instrumented.

## Exporter endpoint

The instrumentation layer exports to the Collector layer over loopback inside the same execution environment.
The defaults are correct for every runtime, so do not set `OTEL_EXPORTER_OTLP_ENDPOINT` unless you are deliberately bypassing the Collector layer.

| Runtime | Supported protocols | Default endpoint |
|---------|---------------------|------------------|
| Java | `grpc`, `http/protobuf`, `http/json` | `http://localhost:4317` |
| Node.js | `http/protobuf` | `http://localhost:4318` |
| Python | `http/protobuf`, `http/json` | `http://localhost:4318` |
| Ruby | `http/protobuf` | `http://localhost:4318` |

This applies to path A only — [manual instrumentation](./aws-lambda.md#runtimes-without-an-auto-instrumentation-layer) must set the endpoint explicitly.

## The auth token on path A

The Collector layer has no Secrets Manager integration.
Its configuration expands `${env:...}` placeholders from the function's own environment, so on path A the token necessarily ends up in the function configuration, where `lambda:GetFunctionConfiguration` can read it.
There is no way around this on path A: Lambda environment variables are resolved at deploy time.

Keep the token out of source control and out of the deployment template by resolving it at deploy time instead of writing it literally — a CloudFormation dynamic reference (`{{resolve:secretsmanager:<arn>:SecretString:::}}`) or the equivalent in Terraform or CDK.
Restrict `lambda:GetFunctionConfiguration` on the function, and encrypt the environment with a customer-managed KMS key when the token must not be readable by every principal that can describe the function.

When Dash0 is the only backend, this is a concrete reason to prefer [the Dash0 Lambda Extension](./aws-lambda-dash0-layers.md): it resolves `DASH0_TOKEN_SECRET_ARN` at runtime, so the token never enters the function configuration at all.

## Collector configuration

The Collector layer starts with no usable configuration of its own.
Build the configuration layer and write its YAML as described in [AWS Lambda Collector layer](../../../otel-collector/rules/deployment/aws-lambda.md).

## Infrastructure as code

Path A is not just a layer list: it also needs `OPENTELEMETRY_COLLECTOR_CONFIG_URI` in the environment block and a **new resource** for the configuration layer, built from a directory containing `otel-collector-config.yaml` at its root.

Verify the staged asset holds the YAML with no path prefix before deploying — CDK stages it under `cdk.out/asset.<hash>/`, which is the equivalent of the `unzip -l` check in [AWS Lambda Collector layer](../../../otel-collector/rules/deployment/aws-lambda.md#configuration-layer-recommended).

### AWS SAM

<!-- eval:skip -->
```yaml
Resources:
  CollectorConfigLayer:
    Type: AWS::Serverless::LayerVersion
    Properties:
      ContentUri: ./collector-config/
      CompatibleArchitectures: [arm64]

  CheckoutFunction:
    Type: AWS::Serverless::Function
    Properties:
      Runtime: python3.12
      Architectures: [arm64]
      Handler: handler.lambda_handler
      Tracing: PassThrough
      Layers:
        - !Sub 'arn:aws:lambda:${AWS::Region}:184161586896:layer:opentelemetry-python-0_24_0:1'
        - !Sub 'arn:aws:lambda:${AWS::Region}:184161586896:layer:opentelemetry-collector-arm64-0_24_0:1'
        - !Ref CollectorConfigLayer
      Environment:
        Variables:
          AWS_LAMBDA_EXEC_WRAPPER: /opt/otel-instrument
          OPENTELEMETRY_COLLECTOR_CONFIG_URI: /opt/otel-collector-config.yaml
          OTEL_SERVICE_NAME: checkout
          OTEL_RESOURCE_ATTRIBUTES: service.namespace=shop,deployment.environment.name=production
          OTEL_TRACES_SAMPLER: always_on
          OTLP_ENDPOINT: ingress.eu-west-1.aws.dash0.com:4317
          OTLP_AUTH_TOKEN: '{{resolve:secretsmanager:otlp-token:SecretString:::}}'
```

`ContentUri` points at the **directory**, so `otel-collector-config.yaml` lands at the archive root and unpacks to `/opt/otel-collector-config.yaml`.

### Terraform

<!-- eval:skip -->
```hcl
resource "aws_lambda_layer_version" "collector_config" {
  layer_name               = "otel-collector-config"
  filename                 = data.archive_file.collector_config.output_path
  source_code_hash         = data.archive_file.collector_config.output_base64sha256
  compatible_architectures = ["arm64"]
}

resource "aws_lambda_function" "checkout" {
  function_name = "checkout"
  runtime       = "python3.12"
  architectures = ["arm64"]
  handler       = "handler.lambda_handler"
  role          = aws_iam_role.checkout.arn

  layers = [
    "arn:aws:lambda:${var.region}:184161586896:layer:opentelemetry-python-0_24_0:1",
    "arn:aws:lambda:${var.region}:184161586896:layer:opentelemetry-collector-arm64-0_24_0:1",
    aws_lambda_layer_version.collector_config.arn,
  ]

  tracing_config {
    mode = "PassThrough"
  }

  environment {
    variables = {
      AWS_LAMBDA_EXEC_WRAPPER            = "/opt/otel-instrument"
      OPENTELEMETRY_COLLECTOR_CONFIG_URI = "/opt/otel-collector-config.yaml"
      OTEL_SERVICE_NAME                  = "checkout"
      OTEL_RESOURCE_ATTRIBUTES           = "service.namespace=shop,deployment.environment.name=production"
      OTEL_TRACES_SAMPLER                = "always_on"
      OTLP_ENDPOINT                      = "ingress.eu-west-1.aws.dash0.com:4317"
      OTLP_AUTH_TOKEN                    = data.aws_secretsmanager_secret_version.otlp_token.secret_string
    }
  }
}
```

### AWS CDK

<!-- eval:fragment -->
```typescript
// Code.fromAsset on a directory zips the directory CONTENTS, so the YAML lands
// at the archive root. Nesting it in a subdirectory makes the Collector fall
// back to its default configuration and export nothing.
const configLayer = new lambda.LayerVersion(this, 'OtelCollectorConfigLayer', {
  code: lambda.Code.fromAsset(path.join(__dirname, '..', 'collector-config')),
  compatibleArchitectures: [lambda.Architecture.ARM_64],
});

const checkout = new lambda.Function(this, 'Checkout', {
  runtime: lambda.Runtime.PYTHON_3_12,
  architecture: lambda.Architecture.ARM_64,
  handler: 'handler.lambda_handler',
  code: lambda.Code.fromAsset('src'),
  tracing: lambda.Tracing.PASS_THROUGH,
  layers: [
    lambda.LayerVersion.fromLayerVersionArn(this, 'OtelPythonLayer',
      `arn:aws:lambda:${this.region}:184161586896:layer:opentelemetry-python-0_24_0:1`),
    lambda.LayerVersion.fromLayerVersionArn(this, 'OtelCollectorLayer',
      `arn:aws:lambda:${this.region}:184161586896:layer:opentelemetry-collector-arm64-0_24_0:1`),
    configLayer,
  ],
  environment: {
    AWS_LAMBDA_EXEC_WRAPPER: '/opt/otel-instrument',
    OPENTELEMETRY_COLLECTOR_CONFIG_URI: '/opt/otel-collector-config.yaml',
    OTEL_SERVICE_NAME: 'checkout',
    OTEL_RESOURCE_ATTRIBUTES: 'service.namespace=shop,deployment.environment.name=production',
    OTEL_TRACES_SAMPLER: 'always_on',
    OTLP_ENDPOINT: 'ingress.eu-west-1.aws.dash0.com:4317',
    OTLP_AUTH_TOKEN: cdk.SecretValue.secretsManager('otlp-token').unsafeUnwrap(),
  },
});
```

The region in every layer ARN must match the function's region, so synthesize with a concrete region rather than an environment-agnostic stack.

### Container images

Layers cannot be attached to a container-image function, but a layer is only a tarball that AWS unpacks into `/opt`.
Copy the contents of each of the three layers into the image in a multi-stage build and set the same environment variables — see [container images](./aws-lambda.md#container-images).

## Anti-patterns

- **Attaching the layers but leaving `AWS_LAMBDA_EXEC_WRAPPER` unset.**
  The layers are inert without it and the function emits nothing.
- **Using the wrong wrapper path for the runtime.**
  `/opt/otel-instrument` does not exist in the Node.js, Java, or Ruby layers, so the function fails at startup before the handler runs.
  The Python layer is the exception: it ships both, and `/opt/otel-handler` works there as a shim.
- **Expecting the Node.js layer to trace `fetch` out of the box.**
  The undici instrumentation is bundled but off by default, so `fetch` calls produce `tcp.connect` and `tls.connect` spans and no HTTP client span.
  Add `undici` to `OTEL_NODE_ENABLED_INSTRUMENTATIONS`.
- **Adding `@opentelemetry/instrumentation-undici` to the function's dependencies to fix the above.**
  The layer already contains it; a second copy in the deployment package is not what the layer loads, so the spans still do not appear.
- **Putting package names in `OTEL_PYTHON_DISABLED_INSTRUMENTATIONS`.**
  The variable takes entry point names, and an unrecognized name is ignored silently, so the instrumentation stays on and the configuration looks correct.
  Read the names out of the installed packages with `entry_points(group="opentelemetry_instrumentor")`.
- **Installing a Python instrumentation package into the bundle without pinning it to the layer's version.**
  `pip` pulls its own `opentelemetry-api`, `opentelemetry-sdk`, and `opentelemetry-semantic-conventions` into `/var/task`, which shadows `/opt/python` and breaks the layer.
  Pin the version from the layer release's `requirements.txt` and install with `--no-deps`.
- **Building a Java function for the `javawrapper` layer without `aws-lambda-java-events`.**
  Every invocation fails with `NoClassDefFoundError` on `APIGatewayProxyRequestEvent`, at invocation time rather than deploy time.
- **Attaching an `amd64` Collector layer to an `arm64` function.**
  The extension binary fails to start and the function exports nothing, while the handler keeps succeeding.
- **Zipping the Collector configuration inside a subdirectory.**
  The Collector does not find it at `/opt/otel-collector-config.yaml`, falls back to its default configuration, and exports nothing.
- **Attaching the Dash0 Lambda Extension alongside these layers.**
  Both subscribe to the Lambda Telemetry API; pick one path.

## References

- [AWS Lambda deployment](./aws-lambda.md)
- [AWS Lambda with the Dash0 Lambda Extension](./aws-lambda-dash0-layers.md)
- [AWS Lambda Collector layer](../../../otel-collector/rules/deployment/aws-lambda.md)
- [Lambda auto-instrumentation](https://opentelemetry.io/docs/platforms/faas/lambda-auto-instrument/)
- [open-telemetry/opentelemetry-lambda](https://github.com/open-telemetry/opentelemetry-lambda)

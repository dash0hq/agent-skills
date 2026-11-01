---
title: "AWS Lambda Collector layer"
impact: HIGH
tags:
  - deployment
  - aws
  - lambda
  - serverless
  - layers
---

# AWS Lambda Collector layer

The AWS Lambda Collector layer adds a purpose-built Collector distribution that acts as an **external** Lambda extension.
It enables export of telemetry asynchronously with respect to the Lambda function returning a response to the client.
It has a different component set and a different correct configuration from Collectors outside of Lambda:

* The `telemetryapi` receiver subscribes to the [Lambda Telemetry API](https://docs.aws.amazon.com/lambda/latest/dg/telemetry-api.html) and turns platform events, function logs, and platform metrics into spans, log records, and metrics.
* The `decouple` processor returns the SDK's loopback export even though the Collector's exporter has not yet finished talking to the backend.

For the function-side configuration — which layers to attach, wrapper paths, and environment variables — see [AWS Lambda with upstream OpenTelemetry layers](../../../otel-instrumentation/rules/platforms/aws-lambda-otel-layers.md).

## When to use the Collector Lambda layer

Attach the Collector layer when the function uses the **upstream OpenTelemetry Lambda layers**.
Do **not** attach it when the function uses the Dash0 Lambda Extension, which itself acts as an external extension.

The layer ARN is region- and architecture-scoped:

<!-- eval:skip -->
```
arn:aws:lambda:<region>:184161586896:layer:opentelemetry-collector-amd64-<version>:1
arn:aws:lambda:<region>:184161586896:layer:opentelemetry-collector-arm64-<version>:1
```

Write `<version>` with underscores, not dots: release `0.24.0` gives `opentelemetry-collector-arm64-0_24_0:1`.
Substituting the release string literally produces a layer that does not exist.

Resolve `<version>` from the [opentelemetry-lambda releases](https://github.com/open-telemetry/opentelemetry-lambda/releases).

## Collector configuration

The Collector layer ships without a usable configuration.
Supply one that receives from both the Lambda Telemetry API and OTLP, decouples the pipeline, and exports to the backend over OTLP/gRPC.

The example below is written for the `opentelemetry-lambda` Collector build, which has the `telemetryapi` receiver and the `decouple` processor.

<!-- telemetryapi and decouple do not exist in `otelcol-contrib` -->
<!-- eval:skip -->
```yaml
receivers:
  telemetryapi:
  otlp:
    protocols:
      grpc:
        endpoint: "localhost:4317"
      http:
        endpoint: "localhost:4318"

processors:
  decouple:

exporters:
  otlp_grpc:
    endpoint: ${env:OTLP_ENDPOINT}
    headers:
      Authorization: "Bearer ${env:OTLP_AUTH_TOKEN}"

service:
  pipelines:
    traces:
      receivers: [telemetryapi, otlp]
      processors: [decouple]
      exporters: [otlp_grpc]
    metrics:
      receivers: [otlp]
      processors: [decouple]
      exporters: [otlp_grpc]
    logs:
      receivers: [telemetryapi, otlp]
      processors: [decouple]
      exporters: [otlp_grpc]
```

Parameterize the endpoint with `${env:...}` as well as the token, rather than writing it literally.
The configuration ships as its own layer, and a literal endpoint means one layer version per region or per backend, which defeats the point of versioning it separately from the function.

The `otlp` receiver must bind to `localhost`, not `0.0.0.0`.
Only the instrumentation layer in the same execution environment needs to connect to the `otlp` receiver.
Binding to all interfaces would expose the receiver to the function's VPC when one is attached.

If a configuration declares the `batch` processor without `decouple`, the layer appends `decouple` automatically.
Do not rely on that: declare it explicitly so the configuration says what it does.

Every `${env:...}` placeholder is resolved from the **function's** environment, so each one named in the configuration has to be set in the function's environment variables.
`OTLP_ENDPOINT` and `OTLP_AUTH_TOKEN` are names this configuration chooses; pick whatever names the deployment already uses, and keep them backend-neutral so the same configuration layer works against a different OTLP endpoint.

Do not write the literal token into the configuration file: the configuration is packaged as a layer, and layers are readable by any principal that can call `lambda:GetLayerVersion`.

The Collector layer has no Secrets Manager integration, so the token has to reach the function as an environment variable and is therefore readable through `lambda:GetFunctionConfiguration`.
Resolve it at deploy time from a secret rather than committing it — a CloudFormation dynamic reference, or the Terraform or CDK equivalent — and see [the auth token on path A](../../../otel-instrumentation/rules/platforms/aws-lambda-otel-layers.md#the-auth-token-on-path-a) for the mitigations and for why the Dash0 Lambda Extension avoids the problem entirely.

### The `telemetryapi` receiver builds its own resource

The receiver does not read `OTEL_RESOURCE_ATTRIBUTES`.
It emits `service.name`, `cloud.provider`, `cloud.region`, `faas.version`, `faas.max_memory`, and `service.instance.id`, and nothing else.

So platform spans such as `init <function>` arrive under a **different resource** from the SDK's spans, missing `service.namespace`, `service.version`, `deployment.environment.name`, `cloud.account.id`, and `faas.instance`.
In the backend they show up as a second, thinner instance of the same service rather than grouping with it.

Stamp the missing attributes back on with a `transform` processor, and put it in every pipeline alongside `decouple`:

<!-- eval:skip -->
```yaml
processors:
  decouple:
  transform/telemetry_api_resource:
    error_mode: ignore
    trace_statements:
      - set(resource.attributes["service.namespace"], "${env:OTEL_SERVICE_NAMESPACE}")
          where resource.attributes["service.namespace"] == nil
      - set(resource.attributes["deployment.environment.name"], "${env:OTEL_DEPLOYMENT_ENVIRONMENT_NAME}")
          where resource.attributes["deployment.environment.name"] == nil
```

Set `OTEL_SERVICE_NAMESPACE` and `OTEL_DEPLOYMENT_ENVIRONMENT_NAME` on the function next to `OTEL_RESOURCE_ATTRIBUTES` so the two sources agree.
The `where ... == nil` guard keeps the statement from overwriting the SDK's own values on spans that already carry them.
The example wires it into traces and logs only: its metrics support is at `Development` stability, and the invocation metrics it emits overlap with what the backend already derives from the platform spans.
Add `telemetryapi` to the metrics pipeline when you specifically want the Telemetry API's platform metrics.

## How this differs from the standard Collector guidance

The rest of this skill targets long-lived Collectors.
Three of its rules are wrong inside Lambda:

| Standard guidance | Inside Lambda |
|-------------------|---------------|
| `memory_limiter` first in every pipeline | Omit it. The execution environment has a fixed memory cap and processes one invocation at a time, so there is no sustained burst for the limiter to absorb — and shedding data in a process that is about to freeze loses it permanently. |
| Exporter batching via `sending_queue: { batch: {} }` with `file_storage` | Omit both. There is no durable filesystem to persist a queue to, and a queue that outlives the freeze is never drained. Use `decouple` instead. |
| `k8s_attributes` and `resource_detection` for enrichment | Not applicable. The FaaS and cloud resource attributes come from the instrumentation layer — see [resource attributes on Lambda](../../../otel-instrumentation/rules/platforms/aws-lambda.md#resource-attributes-on-lambda). |

Do not "fix" a Lambda Collector configuration into the shape used for a Kubernetes DaemonSet.

## Building the configuration layer

Point the function at the configuration with `OPENTELEMETRY_COLLECTOR_CONFIG_URI`.
There are three ways to supply it.

### Configuration layer (recommended)

Package the configuration as its own layer so it can be updated and versioned independently of the function code.

```bash
zip otel-collector-config.zip otel-collector-config.yaml

aws lambda publish-layer-version \
  --layer-name otel-collector-config \
  --zip-file fileb://otel-collector-config.zip \
  --compatible-architectures arm64
```

Then set `OPENTELEMETRY_COLLECTOR_CONFIG_URI=/opt/otel-collector-config.yaml`.

The YAML file must sit at the **root** of the archive.
AWS unpacks layers into `/opt`, so a file zipped inside a subdirectory lands at `/opt/<subdir>/otel-collector-config.yaml` and the Collector falls back to its default configuration without reporting an error.
Verify with `unzip -l otel-collector-config.zip` before publishing: the listing must show `otel-collector-config.yaml` with no path prefix.

When an IaC tool builds the layer for you, check its staged directory instead, because the archive does not exist until publish time.
With CDK, `lambda.Code.fromAsset(dir)` zips the *contents* of `dir`, so point it at a directory holding only `otel-collector-config.yaml` and confirm with `ls cdk.out/asset.*/otel-collector-config.yaml` after `cdk synth`.
After deploying, check the archive itself: call `aws lambda get-layer-version-by-arn`, download the URL it returns, and run `unzip -l` on the result.

### Bundled with the function

Place the file in the deployment package and set `OPENTELEMETRY_COLLECTOR_CONFIG_URI=/var/task/otel-collector-config.yaml`.
Simplest to set up, but changing the configuration requires redeploying the function code.

### From S3

Set `OPENTELEMETRY_COLLECTOR_CONFIG_URI=s3://<bucket>.s3.<region>.amazonaws.com/<key>` and grant the function's execution role `s3:GetObject` on that object.
This allows changing the configuration for many functions at once, at the cost of an S3 read on every cold start.

## Validating the setup with the debug exporter

Add the `debug` exporter to one pipeline temporarily, invoke the function, and read the CloudWatch log group:

<!-- telemetryapi and decouple do not exist in `otelcol-contrib` -->
<!-- eval:skip -->
```yaml
exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    traces:
      receivers: [telemetryapi, otlp]
      processors: [decouple]
      exporters: [debug, otlp_grpc]
```

The Collector's own output goes to the function's log group, interleaved with the function's logs.
Remove the debug exporter before going to production: at `detailed` verbosity it writes every span to CloudWatch Logs, which is billed per ingested byte.

## Dash0 dataset routing

If the Dash0 organization uses multiple [datasets](https://www.dash0.com/documentation/dash0/key-concepts/datasets), add the `Dash0-Dataset` header to the OTLP exporter in the Collector configuration.

<!-- eval:skip -->
```yaml
exporters:
  otlp_grpc:
    endpoint: ${env:OTLP_ENDPOINT}
    headers:
      Authorization: "Bearer ${env:OTLP_AUTH_TOKEN}"
      Dash0-Dataset: "my-dataset"
```

A missing or incorrect `Dash0-Dataset` header causes telemetry to land in the default dataset.
The debug exporter shows telemetry data, not outgoing headers — verify the header in the packaged configuration file directly.

## Anti-patterns

- **Zipping the configuration inside a subdirectory.**
  The Collector does not find it at `/opt/otel-collector-config.yaml`, silently falls back to its default configuration, and exports nothing.
  Zip the YAML at the root of the archive.
- **Omitting the `decouple` processor.**
  The Collector pipeline runs synchronously on the call that fed it, so the handler's loopback export blocks until the backend has answered.
  The response is held for the full upstream round trip even though the Collector is a separate process.
- **Carrying `memory_limiter`, `sending_queue`, and `file_storage` over from a Kubernetes configuration.**
  There is no durable filesystem for the queue, and a queue not drained before the freeze is lost.
- **Binding the OTLP receiver to `0.0.0.0`.**
  Only the in-process instrumentation layer needs to reach it; bind to `localhost`.
- **Attaching an architecture-mismatched Collector layer.**
  The extension binary never starts, the handler keeps succeeding, and no telemetry is exported.
- **Attaching the Collector layer alongside the Dash0 Lambda Extension.**
  Both subscribe to the Lambda Telemetry API and duplicate or drop telemetry.
- **Writing the auth token literally into the configuration file.**
  The configuration is packaged as a layer and is readable through `lambda:GetLayerVersion`.
  Reference it with an `${env:...}` placeholder resolved from the function's environment.
- **Leaving the debug exporter enabled in production.**
  Every span is written to CloudWatch Logs and billed per ingested byte.

## References

- [AWS Lambda deployment](../../../otel-instrumentation/rules/platforms/aws-lambda.md)
- [AWS Lambda with upstream OpenTelemetry layers](../../../otel-instrumentation/rules/platforms/aws-lambda-otel-layers.md)
- [Collector Lambda layer](https://github.com/open-telemetry/opentelemetry-lambda/tree/main/collector)
- [Lambda auto-instrumentation](https://opentelemetry.io/docs/platforms/faas/lambda-auto-instrument/)
- [AWS Lambda Telemetry API](https://docs.aws.amazon.com/lambda/latest/dg/telemetry-api.html)
- [Dash0 AWS Lambda integration](https://www.dash0.com/hub/integrations/int_aws_lambda/overview)
- [Exporters](../exporters.md)

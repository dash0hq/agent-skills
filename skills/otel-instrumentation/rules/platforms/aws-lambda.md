---
title: 'AWS Lambda OpenTelemetry layers'
impact: HIGH
tags:
  - aws
  - lambda
  - serverless
  - opentelemetry
  - adot
  - deployment
---

# AWS Lambda OpenTelemetry layers

Use this rule when an application runs on AWS Lambda and the deployment change includes OpenTelemetry instrumentation or Lambda layers.
Keep application code and function-deployment changes in this skill.
When the deployment uses a Collector extension layer, configure the layer and its configuration URI here, but author the referenced Collector YAML with the `otel-collector` skill.

Do not copy a layer ARN or version from this file.
Resolve the current layer from the authoritative publisher for the function's region, architecture, runtime, and selected distribution, then pin the exact resolved ARN in the deployment.

## Decision process

Follow these steps in order and stop when a branch says to stop.

1. Inspect the function package type in CloudFormation, SAM, CDK, Terraform, or the deployment configuration.
   If the function uses `PackageType: Image`, do not add `Layers`: [AWS Lambda layers](https://docs.aws.amazon.com/lambda/latest/dg/chapter-layers.html) are available only to `.zip` functions.
   Package the SDK, auto-instrumentation, and any required Lambda extension into the container image instead, then follow the language SDK rule.
2. For a `.zip` function, preserve an existing instrumentation family.
   If the deployment already uses ADOT, CloudWatch Application Signals, or an AWS-managed ADOT layer, continue with ADOT.
   If it already uses layers from `open-telemetry/opentelemetry-lambda`, continue with upstream OpenTelemetry.
3. If the function has no existing OpenTelemetry distribution, select from explicit project intent.
   Use ADOT when the project explicitly requires AWS-managed ADOT, CloudWatch Application Signals, or the AWS-supported distribution.
   Use upstream OpenTelemetry Lambda layers when the project explicitly requires the vendor-neutral OpenTelemetry Lambda distribution or backend-neutral OTLP export.
4. If neither distribution is explicit, use the upstream OpenTelemetry Lambda distribution for backend-neutral OpenTelemetry.
   Do not silently enable Application Signals or another AWS-specific telemetry backend as a side effect of adding OpenTelemetry.
5. Resolve the runtime, architecture, and region from the deployment.
   Do not infer `x86_64`, `arm64`, or a region from the developer workstation.
6. Resolve a currently supported layer ARN from the selected distribution's official documentation or release.
   Verify runtime support and that the ARN is published for the target region and architecture.
7. Pin the resolved layer ARN in IaC.
   Do not resolve “latest” at Lambda startup, and do not make a deployment silently change layer versions on each apply.
8. Configure the wrapper and exporter path exactly as documented by the selected runtime and distribution.
   Wrapper paths are not interchangeable between upstream OpenTelemetry and ADOT.
9. Verify the deployed function with a controlled invocation before broad rollout.
   Confirm telemetry, function behavior, duration, cold-start behavior, and the absence of duplicate spans.

## Upstream OpenTelemetry Lambda layers

The upstream [`open-telemetry/opentelemetry-lambda`](https://github.com/open-telemetry/opentelemetry-lambda) project publishes 2 layer families designed to work together: a language-specific layer and a Collector extension layer.
The language-specific layer initializes runtime instrumentation, while the Collector layer runs a stripped-down OpenTelemetry Collector as a Lambda extension.

Before selecting a language layer, read the current upstream [`Extension Layer Language Support`](https://github.com/open-telemetry/opentelemetry-lambda#extension-layer-language-support) section.
If the runtime is listed under “Additional language tooling not currently supported,” do not fabricate a layer ARN; use the language SDK rule and the runtime's supported Lambda instrumentation path instead.

Resolve current ARNs from the upstream repository's [release information](https://github.com/open-telemetry/opentelemetry-lambda/releases).
The upstream project publishes ARN patterns and release-specific versions, but this skill intentionally does not freeze those versions.

### Upstream Node.js ZIP example

For the upstream Node.js layer, the [runtime documentation](https://github.com/open-telemetry/opentelemetry-lambda/tree/main/nodejs) uses `AWS_LAMBDA_EXEC_WRAPPER=/opt/otel-handler`.
Re-check the runtime README when resolving the layer because the wrapper is part of the layer contract, not a universal OpenTelemetry Lambda constant.

<!-- eval:cloudformation -->
```yaml
AWSTemplateFormatVersion: '2010-09-09'
Transform: AWS::Serverless-2016-10-31
Parameters:
  OTelLanguageLayerArn:
    Type: String
  OTelCollectorLayerArn:
    Type: String
Resources:
  CheckoutFunction:
    Type: AWS::Serverless::Function
    Properties:
      Runtime: nodejs22.x
      Handler: index.handler
      CodeUri: src/
      Architectures:
        - arm64
      Layers:
        - Ref: OTelLanguageLayerArn
        - Ref: OTelCollectorLayerArn
      Environment:
        Variables:
          AWS_LAMBDA_EXEC_WRAPPER: /opt/otel-handler
          OPENTELEMETRY_COLLECTOR_CONFIG_URI: /var/task/collector.yaml
          OTEL_SERVICE_NAME: checkout-service
          OTEL_RESOURCE_ATTRIBUTES: deployment.environment.name=production
```

Treat `OTelLanguageLayerArn` and `OTelCollectorLayerArn` as deployment inputs whose values are exact, reviewed layer-version ARNs.
Resolve those values from the upstream release for the target region and architecture rather than copying an ARN from another region.

### Collector configuration handoff

The upstream Collector extension reads a custom Collector configuration from the URI in `OPENTELEMETRY_COLLECTOR_CONFIG_URI`.
A local file in the function package can use `/var/task/collector.yaml`; HTTP and S3 URIs are also supported by the upstream extension.

Do not duplicate receivers, processors, exporters, or pipelines in this skill.
Author the referenced file with the `otel-collector` guidance for [pipelines](../../../otel-collector/rules/pipelines.md) and [exporters](../../../otel-collector/rules/exporters.md), keeping endpoints and credentials in the project's deployment and secret-delivery mechanisms.

The Lambda extension is a stripped-down distribution, so a general `otelcol-contrib` configuration may reference unavailable components.
Before deploying the referenced configuration, verify every component against the extension's current [`collector/lambdacomponents/default.go`](https://github.com/open-telemetry/opentelemetry-lambda/blob/main/collector/lambdacomponents/default.go).

## AWS Distro for OpenTelemetry

Use AWS Distro for OpenTelemetry when the repository already uses ADOT or when the deployment explicitly requires AWS-managed ADOT or CloudWatch Application Signals.
Resolve the managed layer ARN from the current ADOT Lambda documentation for the exact runtime, architecture, and region.

Distinguish the current optimized ADOT Lambda layers from the legacy ADOT layers that embed a Collector.
The current [optimized/Application Signals documentation](https://aws-otel.github.io/docs/getting-started/lambda/) uses `AWS_LAMBDA_EXEC_WRAPPER=/opt/otel-instrument` and documents custom OTLP export for traces without adding the legacy embedded Collector path.
Verify metrics and logs separately against the documentation for the selected layer instead of generalizing the traces behavior to every signal.
Older runtime-specific ADOT pages document different wrapper paths, including `/opt/otel-handler` and Java handler-specific variants.
Do not copy a wrapper from a legacy page into a current optimized layer, or vice versa; resolve the wrapper from the documentation for the exact layer generation and runtime being deployed.

Do not combine an ADOT auto-instrumentation layer with an upstream OpenTelemetry language layer unless the official documentation for that exact setup requires both.
Two independent SDK initializers can produce duplicate spans, duplicate resource detection, conflicting propagators, and unnecessary cold-start work.

Application Signals is an AWS-specific ADOT path.
Do not enable it merely because a user asked for OpenTelemetry, and do not describe the optimized Application Signals layer as equivalent to the backend-neutral upstream Collector layer.
Use the legacy embedded-Collector ADOT path only when its Collector-based behavior is actually required, for example a compatibility requirement that the optimized layer cannot satisfy.

## Container-image functions

AWS Lambda does not apply Lambda layers to functions deployed from [container images](https://docs.aws.amazon.com/lambda/latest/dg/images-create.html).
When `PackageType: Image` is present, keep `Layers` absent and install the required OpenTelemetry runtime dependencies and extensions in the image.

<!-- eval:bad -->
```yaml
# BAD — Lambda layers cannot be attached to a container-image function.
Resources:
  CheckoutFunction:
    Type: AWS::Lambda::Function
    Properties:
      PackageType: Image
      Code:
        ImageUri: example.invalid/checkout:TEST-1
      Layers:
        - arn:aws:lambda:eu-west-1:123456789012:layer:otel:1
```

Do not convert a ZIP function to a container image solely to add OpenTelemetry.
Packaging changes affect build, deployment, rollback, image scanning, startup behavior, and ownership, so treat them as a separate decision.

## Credentials and configuration

Never place an ingest token, authorization header, or backend credential directly in committed CloudFormation, SAM, CDK, Terraform, `collector.yaml`, or application source.
Use the project's established secret-delivery mechanism and reference the secret at deployment or runtime.

Do not treat `NoEcho` on a CloudFormation parameter as a complete secret-management design.
It hides the value from selected CloudFormation surfaces but does not change the fact that the resulting Lambda environment variable is available to the function process.

Keep `service.name` explicit and stable.
Set additional resource attributes according to [resource attributes](../resources.md), using `deployment.environment.name` rather than the deprecated `deployment.environment`.

## Avoid duplicate initialization

Before adding a layer, search the function package and deployment for existing SDK initialization, auto-instrumentation hooks, `AWS_LAMBDA_EXEC_WRAPPER`, ADOT configuration, and OpenTelemetry Lambda libraries.
If an existing path already initializes the SDK, either keep that path or replace it deliberately; do not stack a second initializer on top.

Common duplicate-initialization signals include 2 root spans for 1 invocation, duplicate HTTP client spans, conflicting `service.name` values, and 2 exporter pipelines.
Treat those as a deployment defect, not as expected Lambda behavior.

## Invocation-end export and cold starts

Lambda freezes execution environments between invocations, so buffered telemetry must follow the selected Lambda instrumentation's lifecycle contract.
Do not copy shutdown code from a long-running server into a Lambda handler without checking the runtime-specific Lambda instrumentation.

The upstream OpenTelemetry Lambda project provides invocation-end provider flushing for supported runtime implementations.
Do not add a second unconditional `forceFlush` to every invocation unless the selected runtime documentation requires it.

Measure cold-start and invocation duration before and after adding or changing layers.
A layer that exports correct telemetry can still be an unacceptable deployment if it causes a material latency or timeout regression.

## Verification checklist

After deployment, perform these checks in order.

1. Invoke 1 known test request and record its AWS request ID.
2. Confirm that the function returns the same status and payload as before instrumentation.
3. Confirm that 1 invocation produces 1 expected invocation/root trace, not duplicate roots from multiple initializers.
4. Confirm that `service.name`, environment, function identity, region, and architecture attributes match the deployed function.
5. Confirm that downstream AWS SDK or HTTP spans are children of the invocation trace where the selected runtime supports that instrumentation.
6. Confirm that exporter or extension errors do not appear in CloudWatch Logs.
7. Compare cold-start duration and steady-state duration with the pre-change baseline.
8. Invoke enough times to exercise a warm execution environment and verify that telemetry continues after freeze/thaw cycles.
9. Roll back immediately if instrumentation changes function behavior, causes timeouts, duplicates telemetry, or loses trace continuity.

## Scope boundary

This rule covers upstream OpenTelemetry layers and AWS-managed ADOT selection for Lambda application deployments.
Do not add the Dash0 Lambda Extension here; keep Dash0-specific Lambda layer behavior in a separate rule or change so the vendor-neutral decision process remains reusable.

## References

- [AWS Lambda layers](https://docs.aws.amazon.com/lambda/latest/dg/chapter-layers.html).
- [AWS Lambda container images](https://docs.aws.amazon.com/lambda/latest/dg/images-create.html).
- [OpenTelemetry Lambda](https://github.com/open-telemetry/opentelemetry-lambda).
- [OpenTelemetry Lambda Collector extension](https://github.com/open-telemetry/opentelemetry-lambda/tree/main/collector).
- [AWS Distro for OpenTelemetry Lambda](https://aws-otel.github.io/docs/getting-started/lambda/).
- [Resource attributes](../resources.md).

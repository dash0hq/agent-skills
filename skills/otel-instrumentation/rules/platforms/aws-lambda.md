---
title: 'AWS Lambda deployment'
impact: HIGH
tags:
  - aws
  - lambda
  - serverless
  - deployment
  - layers
---

# AWS Lambda deployment

AWS Lambda functions are instrumented by attaching Lambda layers and setting environment variables on the function configuration, not by adding an SDK bootstrap file to the deployment package.

Lambda freezes the execution environment as soon as the handler returns.
Any telemetry still buffered in the process at that moment is lost, so the standard long-lived `BatchSpanProcessor` setup described in the SDK rules is wrong here.
Both supported paths solve this with an external Lambda extension — a separate process in the execution environment — that owns the export, which is why layers are mandatory for anything but hand-written instrumentation.

This file is the entry point.
It carries the decision process and everything that is the same on both paths; the path-specific configuration lives in one of two files:

| Path | File |
|------|------|
| A — OpenTelemetry layers | [AWS Lambda with upstream OpenTelemetry layers](./aws-lambda-otel-layers.md) |
| B — Dash0 layers | [AWS Lambda with the Dash0 Lambda Extension](./aws-lambda-dash0-layers.md) |

## Decision process

Follow these steps in order.
Stop at the first match.

1. Code signing is enabled on the function.
   Signed functions reject unsigned layers.
   Consider re-signing the layer through your own signing profile and hosting it yourself.
   Otherwise, use manual SDK instrumentation — see [runtimes without an auto-instrumentation layer](#runtimes-without-an-auto-instrumentation-layer) and be mindful of the issues with exporting telemetry and runtime freeze.
2. The function is packaged as a container image.
   Layers cannot be attached, but the layer contents can be copied into the image — see [container images](#container-images).
   Do not fall back to manual SDK instrumentation if you can fix it by copying layer content into the image.
3. The runtime is Node.js, Python, or Java **and** Dash0 is the only backend.
   Use [path B: the Dash0 Lambda layers](./aws-lambda-dash0-layers.md).
4. The runtime is Node.js, Python, Java, or Ruby **and** the backend is not Dash0, or telemetry must fan out to more than one backend.
   Use [path A: upstream OpenTelemetry layers](./aws-lambda-otel-layers.md).
5. The runtime is Go or .NET.
   No auto-instrumentation layer exists for either, so the function must be instrumented in code — see [runtimes without an auto-instrumentation layer](#runtimes-without-an-auto-instrumentation-layer).
   Choose the exporter side by backend: with Dash0 as the only backend, attach the `dash0-extension-manual` layer, which exports for you and resolves `DASH0_TOKEN_SECRET_ARN` at runtime; otherwise attach the upstream Collector layer and write a Collector configuration.

| | Path A: upstream layers | Path B: Dash0 extension | Manual SDK |
|---|---|---|---|
| Runtimes | Node.js, Python, Java, Ruby | Node.js, Python, Java | Any |
| Layers attached | Instrumentation, Collector, configuration | Dash0 extension | Collector or `dash0-extension-manual` |
| Code changes | None | None | Handler wrapping and provider setup |
| Backends | Any OTLP endpoint, several at once | Dash0 | Any OTLP endpoint |
| Signals | Traces from the layer, logs from the Collector's `telemetryapi` receiver, metrics only if the app emits them | Traces, metrics, logs | Whatever you instrument |
| Container image packaging | Copy `/opt` from the layer | Copy `/opt` from the published extension image | Yes |
| Auth token location | Function environment, readable through `lambda:GetFunctionConfiguration` | Secrets Manager, resolved at runtime | Depends on the exporter layer |

## Runtimes without an auto-instrumentation layer

Go and .NET have no upstream instrumentation layer.
Instrument in code, and attach a layer that owns the export so the handler never blocks on a round trip to the backend: the upstream Collector layer, or `dash0-extension-manual` when Dash0 is the only backend.
Both run as external Lambda extensions — separate processes in the execution environment — so the in-process SDK exports over loopback and the upstream call happens on the extension's lifecycle rather than the handler's.

Do not set `AWS_LAMBDA_EXEC_WRAPPER` on this path, there is no wrapper script to run.

Setting `OTEL_EXPORTER_OTLP_ENDPOINT` **is** required here: nothing configures the SDK for you, so point it at the Collector layer on loopback explicitly and ensure the SDK flushes the telemetry before returning the response.

### Go

Set up the tracer provider as described in [Go instrumentation](../sdks/go.md), then wrap the handler.
`otellambda.WithFlusher` is what makes this work on Lambda: it forces a flush at the end of every invocation, before the execution environment freezes.

<!-- eval:fragment -->
```go
import (
	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	// The detector package is also named `lambda`; alias it to avoid the collision.
	lambdadetector "go.opentelemetry.io/contrib/detectors/aws/lambda"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-lambda-go/otellambda"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	ctx := context.Background()

	detector := lambdadetector.NewResourceDetector()
	res, err := resource.New(ctx, resource.WithDetectors(detector))
	if err != nil {
		log.Fatalf("resource detection failed: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Instrument downstream AWS SDK v2 calls.
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("aws config failed: %v", err)
	}
	otelaws.AppendMiddlewares(&cfg.APIOptions)

	lambda.Start(otellambda.InstrumentHandler(handleRequest,
		otellambda.WithTracerProvider(tp),
		otellambda.WithFlusher(tp),
	))
}
```

The AWS Lambda resource detector does not set `faas.id` or `cloud.account.id`, because neither is available outside an invocation.
The handler instrumentation sets them as span attributes instead.
This may require normalization with the `transform` processor in the observability pipeline.

By default `otellambda` starts a new trace per invocation.
For asynchronous triggers that carry trace context in the payload — SQS, SNS, Kinesis, EventBridge — pass `otellambda.WithEventToCarrier` so context propagates across the queue instead of breaking the trace at the boundary.

### .NET

Set up the SDK as described in [.NET instrumentation](../sdks/dotnet.md), then apply the four Lambda-specific requirements below.
Omitting any of them produces a function that succeeds while exporting nothing.

1. **Wrap the handler** with `OpenTelemetry.Instrumentation.AWSLambda`.
   Add `.AddAWSLambdaConfigurations()` to the tracer provider and make the configured handler `AWSLambdaWrapper.TraceAsync(tracerProvider, OriginalHandler, input, context)`.
   `AddAWSLambdaConfigurations()` is also what supplies the `faas.*` and `cloud.*` resource attributes on this path — without an instrumentation layer, nothing else sets them.
2. **Disable X-Ray context extraction.**
   The X-Ray tracing mode is `PassThrough`, so Lambda always passes a not-sampled `_X_AMZN_TRACE_ID`.
   The AWS Lambda instrumentation honors that header by default and drops every span.
   Set `options.DisableAwsXRayContextExtraction = true` on `AddAWSLambdaConfigurations`.
3. **Set the OTLP protocol explicitly.**
   The .NET OTLP exporter defaults to gRPC.
   Pointing it at port 4318 without `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf` gives a gRPC client talking to an HTTP receiver and no telemetry.
   Either set that variable and use `http://localhost:4318`, or leave the default gRPC protocol and use `http://localhost:4317`.
4. **Force a flush before the handler returns**, in a `finally` block: `tracerProvider.ForceFlush()` and `meterProvider.ForceFlush()`.
   The execution environment freezes on return and anything still buffered is lost.

<!-- eval:fragment -->
```csharp
var tracerProvider = Sdk.CreateTracerProviderBuilder()
    .AddAWSLambdaConfigurations(options => options.DisableAwsXRayContextExtraction = true)
    .AddHttpClientInstrumentation()
    .AddOtlpExporter()
    .Build();

// Configured Lambda handler: Assembly::Namespace.Type::TracingHandler
public async Task<Response> TracingHandler(Request input, ILambdaContext context)
    => await AWSLambdaWrapper.TraceAsync(tracerProvider, Handler, input, context);

public async Task<Response> Handler(Request input, ILambdaContext context)
{
    try { /* ... */ }
    finally { tracerProvider.ForceFlush(); }
}
```

The function's configured handler must be the wrapper method, in the form `Assembly::Namespace.Type::Method`.
Pointing the handler at the original method bypasses the instrumentation entirely.

## Container images

Lambda layers cannot be attached to a container-image function, but a layer is a tarball that AWS unpacks into `/opt`.
Copy the same contents into the image in a multi-stage build and set the same environment variables, and both paths work unchanged.

- Path B: copy from the published Dash0 extension image — see [container images](./aws-lambda-dash0-layers.md#container-images).
- Path A: copy `/opt` out of each of the three upstream layers the same way and set the path A environment variables.

## Resource attributes on Lambda

The platform and the layers supply the FaaS and cloud attributes automatically.
Do not set these by hand:

| Attribute | Source |
|-----------|--------|
| `faas.name` | The function name |
| `faas.version` | The function version or alias |
| `faas.instance` | The execution environment |
| `cloud.provider` | `aws` |
| `cloud.region` | The function's region |
| `cloud.account.id` | Resolved per invocation from the invocation context |

Set the service identity attributes explicitly:

<!-- eval:skip -->
```
OTEL_SERVICE_NAME=checkout
OTEL_RESOURCE_ATTRIBUTES=service.namespace=shop,service.version=1.4.2,deployment.environment.name=production
```

When `OTEL_SERVICE_NAME` is unset, `service.name` falls back to the function name.
That is wrong whenever several functions make up one logical service: `checkout-create`, `checkout-confirm`, and `checkout-refund` are three functions and one service, and leaving the default splits them into three unrelated services in the backend.
Set `OTEL_SERVICE_NAME` to the service, and let `faas.name` carry the function identity.

On path A the Collector layer's `telemetryapi` receiver builds its own resource that does **not** carry these attributes, see [the `telemetryapi` receiver builds its own resource](../../../otel-collector/rules/deployment/aws-lambda.md#the-telemetryapi-receiver-builds-its-own-resource).

See [resolving configuration values](../resolve-values.md) for how to derive each value from the codebase, and the `faas` and `aws` namespace rows in [attributes](../../../otel-semantic-conventions/rules/attributes.md) for the full attribute registry.

## Validation

1. Invoke the function once: `aws lambda invoke --function-name <name> /dev/null`.
   A cold start is required for the layer to initialize, so invoke after the configuration update, not before.
2. Read the invocation's CloudWatch log group.
   The extension logs its registration at the start of the first invocation; the absence of any extension line means the layer is attached but not loading, usually a region or architecture mismatch.
3. Confirm spans arrive in the backend with the expected `service.name` and a populated `faas.name`.
   Group the spans by instrumentation scope: the scope name is what identifies which instrumentation produced each span, and it is the only reliable way to tell a missing instrumentation from a missing layer.
   See [validation](../validation.md) for the full backend validation gate.

## Anti-patterns

These apply on every path.
For path-specific mistakes see the [upstream layers](./aws-lambda-otel-layers.md#anti-patterns) and [Dash0 extension](./aws-lambda-dash0-layers.md#anti-patterns) anti-pattern lists.

- **Relying on a `BatchSpanProcessor` without a force flush in a hand-instrumented function.**
  Lambda freezes the execution environment the moment the handler returns, and the queued spans are lost.
  Use `otellambda.WithFlusher` in Go, or the equivalent end-of-invocation flush in other runtimes.
- **Exporting straight to a remote backend from the function process.**
  The handler cannot return until the export finishes, because the export is in the same process on the same critical path, so the caller waits for the full round trip to the backend.
  Export to an extension layer on loopback instead: it is a separate process, and it owns the upstream call.
- **Attaching a vendor layer alongside the Dash0 or upstream OpenTelemetry layer.**
  Datadog, New Relic, Dynatrace, Lumigo, and ADOT all patch the same runtime hooks and produce duplicate or broken traces.
  Remove the other layer before attaching this one.
- **Using a layer ARN whose region differs from the function's region.**
  Lambda layers are region-scoped; the function fails to update with a `ResourceNotFoundException`.
- **Leaving X-Ray tracing mode on `Active`.**
  The X-Ray SDK and OpenTelemetry compete for the trace context; set `PassThrough`.
- **Leaving `OTEL_SERVICE_NAME` unset.**
  `service.name` falls back to the function name, which splits a multi-function service into several unrelated services in the backend.
- **Copying a layer version out of documentation.**
  Layer versions move independently of the docs that mention them.
  Resolve the current version from the releases page at the time you write the configuration.

## References

- [AWS Lambda with upstream OpenTelemetry layers](./aws-lambda-otel-layers.md)
- [AWS Lambda with the Dash0 Lambda Extension](./aws-lambda-dash0-layers.md)
- [AWS Lambda Collector layer](../../../otel-collector/rules/deployment/aws-lambda.md)
- [Lambda auto-instrumentation](https://opentelemetry.io/docs/platforms/faas/lambda-auto-instrument/)
- [Lambda manual instrumentation](https://opentelemetry.io/docs/platforms/faas/lambda-manual-instrument/)
- [open-telemetry/opentelemetry-lambda](https://github.com/open-telemetry/opentelemetry-lambda)
- [dash0hq/dash0-lambda-extension](https://github.com/dash0hq/dash0-lambda-extension)
- [FaaS semantic conventions](https://opentelemetry.io/docs/specs/semconv/resource/faas/)
- [Resource attributes](../resources.md)

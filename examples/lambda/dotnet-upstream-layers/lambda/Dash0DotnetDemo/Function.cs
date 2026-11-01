using System.Diagnostics;
using System.Diagnostics.Metrics;
using Amazon.Lambda.APIGatewayEvents;
using Amazon.Lambda.Core;
using OpenTelemetry;
using OpenTelemetry.Instrumentation.AWSLambda;
using OpenTelemetry.Metrics;
using OpenTelemetry.Trace;

[assembly: LambdaSerializer(typeof(Amazon.Lambda.Serialization.SystemTextJson.DefaultLambdaJsonSerializer))]

namespace Dash0DotnetDemo;

/// <summary>
/// A .NET Lambda function instrumented with the OpenTelemetry .NET SDK in code.
///
/// There is no upstream OpenTelemetry auto-instrumentation layer for .NET, so the
/// provider setup, the handler wrapping, and the end-of-invocation flush all live here.
/// The upstream Collector layer runs as a Lambda extension in the same execution
/// environment and receives OTLP over loopback.
/// </summary>
public class Function
{
    private const string ActivitySourceName = "Dash0DotnetDemo";
    private const string MeterName = "Dash0DotnetDemo";

    /// <summary>How long to wait for the exporter to drain before the handler returns.</summary>
    private const int FlushTimeoutMilliseconds = 2_000;

    private static readonly ActivitySource Source = new(ActivitySourceName);
    private static readonly Meter Meter = new(MeterName);
    private static readonly Counter<long> InvocationCounter =
        Meter.CreateCounter<long>("demo.invocations", unit: "{invocation}");

    private static readonly HttpClient HttpClient = new()
    {
        Timeout = TimeSpan.FromSeconds(5),
    };

    private static readonly TracerProvider TracerProvider;
    private static readonly MeterProvider MeterProvider;

    static Function()
    {
        // Built once per execution environment, during the init phase, so the cost is
        // not paid on every invocation.
        //
        // AddAWSLambdaConfigurations registers the AWS Lambda resource detector
        // (cloud.provider, cloud.region, faas.name, faas.version) and the invocation
        // span. DisableAwsXRayContextExtraction is required because the function runs
        // with X-Ray tracing mode PassThrough: without it the instrumentation honours
        // the "not sampled" X-Ray trace header that Lambda always sends and drops
        // every span.
        //
        // The OTLP endpoint, protocol, service name, and resource attributes come from
        // the OTEL_* environment variables set by the CDK stack; the SDK reads them.
        TracerProvider = Sdk.CreateTracerProviderBuilder()
            .AddSource(ActivitySourceName)
            .AddHttpClientInstrumentation()
            .AddAWSLambdaConfigurations(options => options.DisableAwsXRayContextExtraction = true)
            .AddOtlpExporter()
            .Build()!;

        MeterProvider = Sdk.CreateMeterProviderBuilder()
            .AddMeter(MeterName)
            .AddOtlpExporter()
            .Build()!;
    }

    /// <summary>
    /// The Lambda handler entry point. This is the method named in the CDK stack's
    /// <c>handler</c> property; it exists only to wrap the real handler.
    /// </summary>
    public async Task<APIGatewayHttpApiV2ProxyResponse> TracingFunctionHandlerAsync(
        APIGatewayHttpApiV2ProxyRequest request,
        ILambdaContext context)
    {
        try
        {
            return await AWSLambdaWrapper.TraceAsync(TracerProvider, FunctionHandlerAsync, request, context);
        }
        finally
        {
            // THE LAMBDA FLUSH. Lambda freezes the execution environment as soon as the
            // handler returns, so anything still sitting in the batch processor at that
            // moment is lost. AWSLambdaWrapper.TraceAsync already force-flushes the
            // TracerProvider when the invocation span ends; this block makes that
            // explicit and additionally flushes the MeterProvider, which the wrapper
            // knows nothing about.
            TracerProvider.ForceFlush(FlushTimeoutMilliseconds);
            MeterProvider.ForceFlush(FlushTimeoutMilliseconds);
        }
    }

    /// <summary>
    /// The real handler: a little CPU work, one outbound call that fails, one that
    /// succeeds, and a small JSON body.
    /// </summary>
    private async Task<APIGatewayHttpApiV2ProxyResponse> FunctionHandlerAsync(
        APIGatewayHttpApiV2ProxyRequest request,
        ILambdaContext context)
    {
        InvocationCounter.Add(1);

        var requestId = $"TEST-{context.AwsRequestId}";
        context.Logger.LogInformation($"demo.invocation.start request_id={requestId}");

        using var activity = Source.StartActivity("demo.work");
        activity?.SetTag("demo.request.id", requestId);

        var checksum = ComputeChecksum();
        activity?.SetTag("demo.checksum", checksum);

        // Deliberately unresolvable: example.test is a reserved domain, so DNS fails and
        // the HttpClient instrumentation records a child span with status ERROR.
        var unreachable = await ProbeAsync("https://example.test/");

        // Reachable: produces a successful child CLIENT span.
        var reachable = await ProbeAsync("https://aws.amazon.com/");

        activity?.SetStatus(ActivityStatusCode.Ok);
        context.Logger.LogInformation($"demo.invocation.end request_id={requestId} checksum={checksum}");

        var body = System.Text.Json.JsonSerializer.Serialize(new
        {
            requestId,
            checksum,
            traceId = Activity.Current?.TraceId.ToString(),
            probes = new[] { unreachable, reachable },
        });

        return new APIGatewayHttpApiV2ProxyResponse
        {
            StatusCode = 200,
            Headers = new Dictionary<string, string> { ["content-type"] = "application/json" },
            Body = body,
        };
    }

    /// <summary>A small amount of CPU work, so the invocation span has measurable duration.</summary>
    private static long ComputeChecksum()
    {
        using var activity = Source.StartActivity("demo.cpu");
        long acc = 0;
        for (var i = 1; i <= 2_000_000; i++)
        {
            acc = (acc + (i * 31L)) % 1_000_003L;
        }

        activity?.SetTag("demo.iterations", 2_000_000);
        return acc;
    }

    /// <summary>
    /// Issues one outbound HTTPS request and reports the outcome. Failures are caught so
    /// the invocation keeps going; the failed child span is the point of the exercise.
    /// </summary>
    private static async Task<ProbeResult> ProbeAsync(string url)
    {
        try
        {
            using var response = await HttpClient.GetAsync(url, HttpCompletionOption.ResponseHeadersRead);
            return new ProbeResult(url, "ok", (int)response.StatusCode, null);
        }
        catch (Exception ex)
        {
            // The HttpClient instrumentation already marked its own CLIENT span ERROR.
            // Record the failure on the enclosing custom span as an event, and carry on.
            Activity.Current?.AddEvent(new ActivityEvent(
                "exception",
                tags: new ActivityTagsCollection
                {
                    ["exception.type"] = ex.GetType().FullName,
                    ["exception.message"] = ex.Message,
                }));
            return new ProbeResult(url, "failed", null, $"{ex.GetType().Name}: {ex.Message}");
        }
    }

    private sealed record ProbeResult(string Url, string Outcome, int? StatusCode, string? Error);
}

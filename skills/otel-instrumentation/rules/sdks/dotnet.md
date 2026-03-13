---
title: ".NET Instrumentation"
impact: HIGH
tags:
  - dotnet
  - backend
  - server
---

# .NET Instrumentation

Instrument .NET applications to generate traces, logs, and metrics for deep insights into behavior and performance.

## Use cases

- **HTTP Request Monitoring**: Understand outgoing and incoming HTTP requests through traces and metrics, with drill-downs to database level
- **Database Performance**: Observe which database statements execute and measure their duration for optimization
- **Error Detection**: Reveal uncaught errors and the context in which they happened

---

## Installation

Download and run the auto-instrumentation install script:

```bash
curl -L -O https://github.com/open-telemetry/opentelemetry-dotnet-instrumentation/releases/latest/download/otel-dotnet-auto-install.sh
./otel-dotnet-auto-install.sh
. $HOME/.otel-dotnet-auto/instrument.sh
```

**Note**: This script is not supported on Apple Silicon.
For Windows, use the [PowerShell guide](https://opentelemetry.io/docs/zero-code/dotnet/getting-started/).

---

## Environment variables

All environment variables that control the SDK behavior:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_SERVICE_NAME` | Yes | `unknown_service` | Identifies your service in telemetry data |
| `OTEL_TRACES_EXPORTER` | Yes | `none` | **Must set to `otlp`** to export traces |
| `OTEL_METRICS_EXPORTER` | No | `none` | Set to `otlp` to export metrics |
| `OTEL_LOGS_EXPORTER` | No | `none` | Set to `otlp` to export logs |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Yes | `http://localhost:4318` | OTLP collector endpoint |
| `OTEL_EXPORTER_OTLP_HEADERS` | No | - | Headers for authentication (e.g., `Authorization=Bearer TOKEN`) |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | No | `http/protobuf` | Protocol: `grpc`, `http/protobuf`, or `http/json` |
| `OTEL_RESOURCE_ATTRIBUTES` | No | - | Additional resource attributes (e.g., `deployment.environment=production`) |

**Critical**: Without `OTEL_TRACES_EXPORTER=otlp`, the SDK defaults to `none` and no telemetry is exported.

### Where to get configuration values

1. **OTLP Endpoint**: Your observability platform's OTLP endpoint
   - In Dash0: [Settings → Organization → Endpoints](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Cb29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
   - Format: `https://<region>.your-platform.com`
2. **Auth Token**: API token for telemetry ingestion
   - In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)
3. **Service Name**: Choose a descriptive name (e.g., `order-api`, `checkout-service`)

---

## Configuration

### 1. Activate the SDK

The SDK is activated by sourcing the instrument script after installation:

```bash
. $HOME/.otel-dotnet-auto/instrument.sh
```

This sets the necessary .NET profiler environment variables that enable auto-instrumentation at runtime.

### 2. Set service name

```bash
export OTEL_SERVICE_NAME="my-service"
```

### 3. Enable exporters

**This step is required** - without it, no telemetry is sent:

```bash
# Required for traces
export OTEL_TRACES_EXPORTER="otlp"

# Optional: also export metrics and logs
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"
```

### 4. Configure endpoint

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"
```

### 5. Optional: target specific dataset

```bash
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN,Dash0-Dataset=my-dataset"
```

---

## Complete setup

### Using environment variables

```bash
# Activate auto-instrumentation
. $HOME/.otel-dotnet-auto/instrument.sh

# Service identification
export OTEL_SERVICE_NAME="my-service"

# Enable exporters (required!)
export OTEL_TRACES_EXPORTER="otlp"
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"

# Configure endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"

dotnet run
```

### Using launchSettings.json

Add environment variables to your `Properties/launchSettings.json`:

```json
{
  "profiles": {
    "MyApp": {
      "commandName": "Project",
      "environmentVariables": {
        "OTEL_SERVICE_NAME": "my-service",
        "OTEL_TRACES_EXPORTER": "otlp",
        "OTEL_METRICS_EXPORTER": "otlp",
        "OTEL_LOGS_EXPORTER": "otlp",
        "OTEL_EXPORTER_OTLP_ENDPOINT": "https://<OTLP_ENDPOINT>",
        "OTEL_EXPORTER_OTLP_HEADERS": "Authorization=Bearer YOUR_AUTH_TOKEN",
        "OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf"
      }
    }
  }
}
```

**Note**: You must still source the instrument script before running `dotnet run`.

---

## Local development

### Console exporter

For development without a collector, use the console exporter to see telemetry in your terminal:

```bash
. $HOME/.otel-dotnet-auto/instrument.sh
export OTEL_SERVICE_NAME="my-service"
export OTEL_TRACES_EXPORTER="console"
export OTEL_METRICS_EXPORTER="console"
export OTEL_LOGS_EXPORTER="console"

dotnet run
```

This prints spans, metrics, and logs directly to stdout—useful for verifying instrumentation works before configuring a remote backend.

### Without a collector

If you set `OTEL_TRACES_EXPORTER=otlp` but have no collector running, you will see connection errors.
This is expected behavior.

**Options:**
1. Use `console` exporter during development (recommended for quick testing)
2. Run a local OpenTelemetry Collector
3. Point directly to your observability backend

---

## Resource configuration

Set `service.name`, `service.version`, and `deployment.environment.name` for every deployment.
See [resource attributes](../resources.md) for the full list of required and recommended attributes.

---

## Kubernetes setup

See [Kubernetes deployment](../platforms/k8s.md) for pod metadata injection, resource attributes, and Dash0 Kubernetes Operator guidance.

---

## Supported libraries

The auto-instrumentation package automatically instruments:

| Category | Libraries |
|----------|-----------|
| HTTP | ASP.NET Core, HttpClient |
| Database | SqlClient, Entity Framework Core |
| gRPC | Grpc.Net.Client |
| Messaging | MassTransit |
| Logging | ILogger (Microsoft.Extensions.Logging) |
| Runtime | .NET Runtime metrics, process metrics |

Refer to [OpenTelemetry documentation](https://opentelemetry.io/docs/zero-code/dotnet/instrumentations/) for the complete list.

---

## Custom spans

Add business context to auto-instrumented traces using `System.Diagnostics.ActivitySource` and `Activity`, the .NET native tracing API that OpenTelemetry bridges:

```csharp
using System.Diagnostics;

public class OrderService
{
    private static readonly ActivitySource Source = new("MyService");

    public async Task<Order> ProcessOrder(Order order)
    {
        using var activity = Source.StartActivity("order.process");
        try
        {
            activity?.SetTag("order.id", order.Id);
            activity?.SetTag("order.total", order.Total);
            var result = await SaveOrder(order);
            return result;
        }
        catch (Exception ex)
        {
            activity?.SetStatus(ActivityStatusCode.Error, ex.Message);
            throw;
        }
    }
}
```

### Span status rules

See [span status code](../spans.md#span-status-code) for the full rules.
This section shows how to apply them in .NET.

#### Always include a status message with `ERROR`

The second argument to `SetStatus` is the status message.
It must contain the exception type and a short explanation — enough to understand the failure without opening the full trace.

```csharp
// BAD: no status message
activity?.SetStatus(ActivityStatusCode.Error);

// BAD: generic message with no diagnostic value
activity?.SetStatus(ActivityStatusCode.Error, "something went wrong");

// GOOD: specific message with exception type and context
activity?.SetStatus(ActivityStatusCode.Error, $"{ex.GetType().Name}: {ex.Message}");
```

Do not include stack traces in the status message.
Record those in a log record with `exception.stacktrace` instead.

```csharp
// BAD: stack trace in the status message
activity?.SetStatus(ActivityStatusCode.Error, ex.ToString());

// GOOD: short message only
activity?.SetStatus(ActivityStatusCode.Error, ex.Message);
```

#### Use `OK` only for confirmed success

Set status to `OK` when application logic has explicitly verified the operation succeeded.
Leave status `UNSET` if the code simply did not encounter an error.

```csharp
// GOOD: explicit confirmation from downstream
var response = await httpClient.GetAsync(url);
if (response.IsSuccessStatusCode)
{
    activity?.SetStatus(ActivityStatusCode.Ok);
}

// BAD: setting OK speculatively
activity?.SetStatus(ActivityStatusCode.Ok);
return await SomeMethodAsync(); // might still fail after this point
```

---

## Structured logging

Configure your logging framework to serialize exceptions into a single structured field so that stack traces do not break the one-line-per-record contract.
See [logs](../logs.md) for general guidance on structured logging and exception stack traces.

### Serilog with compact JSON

[Serilog](https://serilog.net/) with `Serilog.Formatting.Compact` produces single-line JSON output with exceptions serialized into a structured field.

```csharp
using Serilog;
using Serilog.Formatting.Compact;

Log.Logger = new LoggerConfiguration()
    .WriteTo.Console(new CompactJsonFormatter())
    .CreateLogger();

try
{
    ProcessOrder(order);
}
catch (Exception ex)
{
    Log.Error(ex, "order.failed {@OrderId}", order.Id);
}
```

The `CompactJsonFormatter` serializes the exception (including its stack trace) into an `"x"` field as a single escaped string.

### Microsoft.Extensions.Logging with JSON console

ASP.NET Core's built-in console logger supports JSON output starting from .NET 5.

```csharp
builder.Logging.AddJsonConsole();
```

```csharp
try
{
    ProcessOrder(order);
}
catch (Exception ex)
{
    logger.LogError(ex, "order.failed, OrderId={OrderId}", order.Id);
}
```

The JSON console formatter serializes exceptions into a structured field, keeping each log record on a single line.

---

## Troubleshooting

### No telemetry appearing

**Check exporters are enabled:**
```bash
echo $OTEL_TRACES_EXPORTER  # Should be "otlp" or "console", not empty
```

The SDK defaults `OTEL_TRACES_EXPORTER` to `none`, which silently discards all telemetry.

**Verify the instrument script was sourced:**
```bash
echo $CORECLR_ENABLE_PROFILING  # Should be "1"
```

### Connection errors

This means the SDK is working but cannot reach the collector:
- **No collector running**: Start a local collector or use `OTEL_TRACES_EXPORTER=console`
- **Wrong endpoint**: Check `OTEL_EXPORTER_OTLP_ENDPOINT` is correct
- **Port mismatch**: gRPC uses 4317, HTTP uses 4318

### Apple Silicon not supported

The install script does not support Apple Silicon (arm64 macOS).
Use a Linux or Windows environment, or run inside a container for local development on Apple Silicon.

### "Exporter is empty" or similar warnings

Usually means `OTEL_TRACES_EXPORTER` (or metrics/logs) is not set.
Set it explicitly:
```bash
export OTEL_TRACES_EXPORTER="otlp"
```

---

## Resources

- [OpenTelemetry .NET Documentation](https://opentelemetry.io/docs/languages/dotnet/)
- [Zero-Code Instrumentation for .NET](https://opentelemetry.io/docs/zero-code/dotnet/)
- [Environment Variable Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)

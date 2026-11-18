# C# project assembly fixture

Create the telemetry helper:

```csharp
// Telemetry/Tracing.cs
using System.Diagnostics;

namespace SkillExamples.Telemetry;

public static class Tracing
{
    public static readonly ActivitySource Source = new("checkout");
}
```

Use it from a service; the record and the service share the project with the helper:

```csharp
using System.Diagnostics;
using SkillExamples.Telemetry;

public record Order(string Id, string CustomerEmail);

public class CheckoutService
{
    public void Place(Order order)
    {
        using var activity = Tracing.Source.StartActivity("order.place");
        activity?.SetTag("order.id", order.Id);
        activity?.SetStatus(ActivityStatusCode.Ok);
    }
}
```

Program.cs holds the top-level statements and wires the pinned OpenTelemetry packages:

```csharp
// Program.cs
using OpenTelemetry;
using OpenTelemetry.Trace;

using var tracerProvider = Sdk.CreateTracerProviderBuilder()
    .SetSampler(new AlwaysOnSampler())
    .AddSource("checkout")
    .AddOtlpExporter()
    .Build();

new CheckoutService().Place(new Order("TEST-0001", "user@example.test"));
```

A snippet without a type declaration is a fragment and is not compiled:

```csharp
activity?.SetTag("order.id", orderId);
```

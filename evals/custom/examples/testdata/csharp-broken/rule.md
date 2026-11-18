# Broken C# project fixture

```csharp
using System.Diagnostics;

public static class Tracing
{
    public static readonly ActivitySource Source = new("checkout");
}
```

```csharp
using System.Diagnostics;

public class CheckoutService
{
    public void Place(string orderId)
    {
        using var activity = Tracing.Source.StartActivity("order.place");
        activity?.SetTag("enduser.id", "TEST-0001");
        activity?.SetStatus(ActivityStatusCode.Failed);
    }
}
```

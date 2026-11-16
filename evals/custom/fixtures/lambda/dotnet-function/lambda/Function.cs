using System.Text.Json;
using Amazon.Lambda.Core;

[assembly: LambdaSerializer(typeof(Amazon.Lambda.Serialization.SystemTextJson.DefaultLambdaJsonSerializer))]

namespace Checkout;

/// <summary>
/// Checkout handler: looks up inventory with one outbound HTTP call and returns a synthetic order.
/// There is deliberately no OpenTelemetry code here.
/// </summary>
public class Function
{
    private static readonly Uri InventoryUrl = new("https://checkip.amazonaws.com/");
    private static readonly HttpClient Http = new() { Timeout = TimeSpan.FromSeconds(5) };

    public async Task<Dictionary<string, object>> FunctionHandler(Dictionary<string, object>? input, ILambdaContext context)
    {
        var inventory = (await Http.GetStringAsync(InventoryUrl)).Trim();
        context.Logger.LogInformation("{\"message\":\"checkout completed\",\"order.id\":\"TEST-0001\"}");
        return new Dictionary<string, object>
        {
            ["statusCode"] = 200,
            ["body"] = JsonSerializer.Serialize(new
            {
                order_id = "TEST-0001",
                customer_email = "user@example.test",
                status = "confirmed",
                inventory,
            }),
        };
    }
}

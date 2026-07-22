// Eval fixture: deliberately uninstrumented ASP.NET Core minimal API (see
// evals/fixtures/README.md for the contract). It serves GET /checkout,
// performs one outbound HTTP call to DOWNSTREAM_URL while handling it,
// listens on PORT (default 8080), and uses only obviously synthetic data
// (user@example.test, TEST-0001).
var builder = WebApplication.CreateBuilder(args);

var port = Environment.GetEnvironmentVariable("PORT") ?? "8080";
builder.WebHost.UseUrls($"http://0.0.0.0:{port}");
builder.Services.AddHttpClient();

var app = builder.Build();

var downstreamUrl = Environment.GetEnvironmentVariable("DOWNSTREAM_URL")
    ?? "http://localhost:9090/inventory";

app.MapGet("/checkout", async (IHttpClientFactory clientFactory, ILogger<Program> logger) =>
{
    string inventory;
    try
    {
        var client = clientFactory.CreateClient();
        client.Timeout = TimeSpan.FromSeconds(5);
        inventory = await client.GetStringAsync(downstreamUrl);
    }
    catch (Exception ex)
    {
        logger.LogError(ex, "inventory lookup failed");
        return Results.Json(new { error = "inventory unavailable" }, statusCode: 502);
    }

    logger.LogInformation("checkout completed for order {OrderId} by {CustomerEmail}",
        "TEST-0001", "user@example.test");

    return Results.Json(new
    {
        order_id = "TEST-0001",
        customer_email = "user@example.test",
        status = "confirmed",
        inventory,
    });
});

app.Run();

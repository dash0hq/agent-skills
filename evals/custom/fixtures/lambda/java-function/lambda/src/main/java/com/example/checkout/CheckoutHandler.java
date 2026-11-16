package com.example.checkout;

import com.amazonaws.services.lambda.runtime.Context;
import com.amazonaws.services.lambda.runtime.RequestHandler;
import java.io.IOException;
import java.io.UncheckedIOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.Map;

/**
 * Checkout handler: looks up inventory with one outbound HTTP call and returns a synthetic order.
 * There is deliberately no OpenTelemetry code here.
 */
public class CheckoutHandler implements RequestHandler<Map<String, Object>, Map<String, Object>> {

  private static final URI INVENTORY_URL = URI.create("https://checkip.amazonaws.com/");
  private static final Duration REQUEST_TIMEOUT = Duration.ofSeconds(5);
  private static final HttpClient HTTP = HttpClient.newBuilder().connectTimeout(REQUEST_TIMEOUT).build();

  @Override
  public Map<String, Object> handleRequest(Map<String, Object> event, Context context) {
    HttpRequest request = HttpRequest.newBuilder(INVENTORY_URL).timeout(REQUEST_TIMEOUT).GET().build();
    String inventory;
    try {
      inventory = HTTP.send(request, HttpResponse.BodyHandlers.ofString()).body().trim();
    } catch (IOException e) {
      throw new UncheckedIOException(e);
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      throw new IllegalStateException(e);
    }
    context.getLogger().log("{\"message\":\"checkout completed\",\"order.id\":\"TEST-0001\"}");
    return Map.of(
        "statusCode", 200,
        "body", "{\"order_id\":\"TEST-0001\",\"customer_email\":\"user@example.test\",\"status\":\"confirmed\",\"inventory\":\"" + inventory + "\"}");
  }
}

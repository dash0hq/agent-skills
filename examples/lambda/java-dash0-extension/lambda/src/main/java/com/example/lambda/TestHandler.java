package com.example.lambda;

import com.amazonaws.services.lambda.runtime.Context;
import com.amazonaws.services.lambda.runtime.RequestHandler;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Deliberately plain Lambda handler.
 *
 * <p>There is no OpenTelemetry SDK code here on purpose: both example projects rely entirely on
 * layer-based auto-instrumentation, so any SDK bootstrap in the handler would invalidate the test.
 * The only dependencies are {@code aws-lambda-java-core} and the JDK HTTP client.
 */
public class TestHandler implements RequestHandler<Map<String, Object>, Map<String, Object>> {

  private static final HttpClient HTTP_CLIENT =
      HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).build();

  private static final String UNREACHABLE_URL = "https://example.test/";
  private static final String REACHABLE_URL = "https://aws.amazon.com/";

  @Override
  public Map<String, Object> handleRequest(Map<String, Object> event, Context context) {
    context.getLogger().log("TEST-0001 invocation start requestId=" + context.getAwsRequestId());

    long checksum = burnCpu();

    // Fails DNS resolution on purpose: .test is a reserved, never-resolvable TLD. The failure is
    // what produces an error span, so it is caught and reported rather than rethrown.
    Map<String, Object> unreachable = fetch(UNREACHABLE_URL, context);
    Map<String, Object> reachable = fetch(REACHABLE_URL, context);

    Map<String, Object> body = new LinkedHashMap<>();
    body.put("testCaseId", "TEST-0001");
    body.put("service", System.getenv().getOrDefault("OTEL_SERVICE_NAME", "unset"));
    body.put("requestId", context.getAwsRequestId());
    body.put("remainingTimeMillis", context.getRemainingTimeInMillis());
    body.put("checksum", checksum);
    body.put("unreachable", unreachable);
    body.put("reachable", reachable);

    context.getLogger().log("TEST-0001 invocation end checksum=" + checksum);
    return body;
  }

  /** A few million iterations of integer work, so the invocation has measurable CPU time. */
  private static long burnCpu() {
    long checksum = 0L;
    for (int i = 1; i <= 4_000_000; i++) {
      checksum += (i * 2654435761L) % 1_000_003L;
    }
    return checksum;
  }

  private static Map<String, Object> fetch(String url, Context context) {
    Map<String, Object> result = new LinkedHashMap<>();
    result.put("url", url);
    try {
      HttpRequest request =
          HttpRequest.newBuilder(URI.create(url))
              .timeout(Duration.ofSeconds(10))
              .header("user-agent", "TEST-0001-lambda-telemetry-sample")
              .GET()
              .build();
      HttpResponse<String> response = HTTP_CLIENT.send(request, HttpResponse.BodyHandlers.ofString());
      result.put("ok", true);
      result.put("status", response.statusCode());
      result.put("bytes", response.body().length());
      context.getLogger().log("TEST-0001 fetch ok url=" + url + " status=" + response.statusCode());
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      result.put("ok", false);
      result.put("error", e.getClass().getSimpleName() + ": " + e.getMessage());
      context.getLogger().log("TEST-0001 fetch interrupted url=" + url);
    } catch (Exception e) {
      result.put("ok", false);
      result.put("error", e.getClass().getSimpleName() + ": " + e.getMessage());
      context.getLogger().log("TEST-0001 fetch failed url=" + url + " error=" + e);
    }
    return result;
  }
}

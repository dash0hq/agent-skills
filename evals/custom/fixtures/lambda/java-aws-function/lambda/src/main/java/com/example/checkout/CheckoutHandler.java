package com.example.checkout;

import com.amazonaws.services.lambda.runtime.Context;
import com.amazonaws.services.lambda.runtime.RequestHandler;
import java.util.Map;
import software.amazon.awssdk.http.urlconnection.UrlConnectionHttpClient;
import software.amazon.awssdk.services.sts.StsClient;

/**
 * Checkout handler: confirms the payment account with one outbound AWS API call and returns a
 * synthetic order. There is deliberately no OpenTelemetry code here. STS GetCallerIdentity needs
 * no IAM permission.
 */
public class CheckoutHandler implements RequestHandler<Map<String, Object>, Map<String, Object>> {

  private static final StsClient STS =
      StsClient.builder().httpClientBuilder(UrlConnectionHttpClient.builder()).build();

  @Override
  public Map<String, Object> handleRequest(Map<String, Object> event, Context context) {
    STS.getCallerIdentity();
    context.getLogger().log("{\"message\":\"checkout completed\",\"order.id\":\"TEST-0001\"}");
    return Map.of(
        "statusCode", 200,
        "body", "{\"order_id\":\"TEST-0001\",\"customer_email\":\"user@example.test\",\"status\":\"confirmed\"}");
  }
}

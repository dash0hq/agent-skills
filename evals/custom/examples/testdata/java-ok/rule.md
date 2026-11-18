# Java project assembly fixture

Name the file of a helper with a path comment:

```java
// src/main/java/test/checkout/telemetry/Tracing.java
package test.checkout.telemetry;

import io.opentelemetry.api.GlobalOpenTelemetry;
import io.opentelemetry.api.trace.Tracer;

public final class Tracing {

    private Tracing() {}

    public static Tracer tracer() {
        return GlobalOpenTelemetry.getTracer("checkout");
    }
}
```

Without a path comment, the package and the public type name the file:

```java
package test.checkout;

import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.StatusCode;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import test.checkout.telemetry.Tracing;

public class CheckoutService {

    private static final Logger logger = LoggerFactory.getLogger(CheckoutService.class);

    public void placeOrder(Order order) {
        Span span = Tracing.tracer().spanBuilder("order.place").startSpan();
        try (var scope = span.makeCurrent()) {
            span.setAttribute("order.id", order.id());
            span.setAttribute("enduser.id", "TEST-0001");
            span.setAttribute("server.address", "shop.example.test");
        } catch (RuntimeException e) {
            span.setStatus(StatusCode.ERROR, e.getClass().getSimpleName() + ": " + e.getMessage());
            logger.atError().addKeyValue("order.id", order.id()).log("order.place.failed");
            throw e;
        } finally {
            span.end();
        }
    }
}

record Order(String id) {}
```

A type without a package is written to the default package:

```java
enum Region { EU, US }
```

A snippet without a type declaration is a fragment and is not compiled:

```java
span.setAttribute("order.id", orderId);
```

# Broken Java project fixture

```java
import io.opentelemetry.api.trace.Span;

public class Enduser {
    public static void tag() {
        Span.current().setAttribute("enduser.id", "TEST-0001");
    }
}
```

```java
import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.StatusCode;

public class Checkout {
    public static void fail() {
        Span.current().setStatus(StatusCode.FAILED, "TEST-0001 rejected");
    }
}
```

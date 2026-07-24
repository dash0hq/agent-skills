// Eval fixture: deliberately uninstrumented Java HTTP service (see
// evals/fixtures/README.md for the contract). It is a Spring Boot Web
// application whose embedded Tomcat plus Spring WebMVC stack the scenario's
// zero-code Java agent auto-instruments, so a SERVER span is produced for
// GET /checkout. It serves GET /checkout, performs one outbound HTTP call to
// DOWNSTREAM_URL while handling it, listens on PORT (default 8080, wired
// through application.properties), and uses only obviously synthetic data
// (user@example.test, TEST-0001). No instrumentation dependencies of its
// own beyond Spring Boot Web: the scenario attaches the agent at run time.
package checkout;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.Map;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

// Spring configuration classes (implied by @SpringBootApplication) may not be
// final, so this class is intentionally left non-final.
@SpringBootApplication
@RestController
public class CheckoutServer {

    // A single reused client keeps the outbound behavior to exactly one GET
    // per handled request, matching the fixture contract.
    private final HttpClient client = HttpClient.newBuilder()
            .connectTimeout(Duration.ofSeconds(5))
            .build();

    private final String downstreamUrl =
            env("DOWNSTREAM_URL", "http://localhost:9090/inventory");

    public static void main(String[] args) {
        SpringApplication.run(CheckoutServer.class, args);
    }

    @GetMapping(path = "/checkout", produces = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> checkout() {
        HttpRequest request = HttpRequest.newBuilder(URI.create(downstreamUrl))
                .timeout(Duration.ofSeconds(5))
                .GET()
                .build();
        String inventory;
        try {
            HttpResponse<String> response =
                    client.send(request, HttpResponse.BodyHandlers.ofString());
            inventory = response.body();
        } catch (Exception e) {
            throw new InventoryUnavailableException(e);
        }

        return Map.of(
                "order_id", "TEST-0001",
                "customer_email", "user@example.test",
                "status", "confirmed",
                "inventory", inventory);
    }

    private static String env(String name, String fallback) {
        String value = System.getenv(name);
        return value == null || value.isEmpty() ? fallback : value;
    }

    // Surfaces a failed inventory lookup as a 502 without pulling in extra
    // Spring types; the default error handling maps the annotation.
    @org.springframework.web.bind.annotation.ResponseStatus(
            org.springframework.http.HttpStatus.BAD_GATEWAY)
    private static final class InventoryUnavailableException extends RuntimeException {
        InventoryUnavailableException(Throwable cause) {
            super("inventory unavailable", cause);
        }
    }
}

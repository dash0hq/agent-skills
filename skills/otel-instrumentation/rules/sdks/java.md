---
title: "Java Instrumentation"
impact: HIGH
tags:
  - java
  - backend
  - server
---

# Java Instrumentation

Instrument Java applications to generate traces, logs, and metrics for deep insights into behavior and performance.

## Use cases

- **HTTP Request Monitoring**: Understand outgoing and incoming HTTP requests through traces and metrics, with drill-downs to database level
- **Database Performance**: Observe which database statements execute and measure their duration for optimization
- **Error Detection**: Reveal uncaught errors and the context in which they happened

---

## Installation

```sh
wget https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar
```

**Note**: The javaagent.jar contains both the agent and instrumentation libraries, enabling automatic instrumentation without modifying source code.

---

## Environment variables

All environment variables that control the SDK behavior:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_SERVICE_NAME` | Yes | `unknown_service:java` | Identifies your service in telemetry data |
| `OTEL_TRACES_EXPORTER` | No | `otlp` | Traces exporter (default is already `otlp`) |
| `OTEL_METRICS_EXPORTER` | No | `otlp` | Metrics exporter (default is already `otlp`) |
| `OTEL_LOGS_EXPORTER` | No | `otlp` | Logs exporter (default is already `otlp`) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Yes | `http://localhost:4318` | OTLP collector endpoint |
| `OTEL_EXPORTER_OTLP_HEADERS` | No | - | Headers for authentication (e.g., `Authorization=Bearer TOKEN`) |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | No | `http/protobuf` | Protocol: `grpc`, `http/protobuf`, or `http/json` |
| `OTEL_RESOURCE_ATTRIBUTES` | No | - | Additional resource attributes (e.g., `deployment.environment=production`) |

**Note**: Unlike Node.js, the Java agent defaults all exporters to `otlp` and the protocol to `http/protobuf`.

### Where to get configuration values

1. **OTLP Endpoint**: Your observability platform's OTLP endpoint
   - In Dash0: [Settings → Organization → Endpoints](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Cb29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
   - Format: `https://<region>.your-platform.com`
2. **Auth Token**: API token for telemetry ingestion
   - In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)
3. **Service Name**: Choose a descriptive name (e.g., `order-api`, `checkout-service`)

---

## Configuration

### 1. Activate the SDK

The SDK is activated by attaching the Java agent to the JVM.

**Via `-javaagent` JVM parameter:**
```sh
java -javaagent:path/to/opentelemetry-javaagent.jar -jar myapp.jar
```

**Via `JAVA_TOOL_OPTIONS` environment variable:**
```sh
export JAVA_TOOL_OPTIONS="-javaagent:path/to/opentelemetry-javaagent.jar"
```

**Note**: The `JAVA_TOOL_OPTIONS` approach applies the agent to every JVM process started in that shell session, including build tools like Maven and Gradle.

### 2. Set service name

```sh
export OTEL_SERVICE_NAME="my-service"
```

### 3. Enable exporters

The Java agent defaults all exporters to `otlp`, so no additional configuration is needed to start exporting telemetry.

To explicitly set exporters:

```sh
# Already the default, but can be set explicitly
export OTEL_TRACES_EXPORTER="otlp"
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"
```

### 4. Configure endpoint

```sh
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"
```

### 5. Optional: target specific dataset

```sh
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN,Dash0-Dataset=my-dataset"
```

---

## Complete setup

### Using environment variables

```sh
# Service identification
export OTEL_SERVICE_NAME="my-service"

# Configure endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"

# Run with the agent
java -javaagent:path/to/opentelemetry-javaagent.jar -jar myapp.jar
```

### Using JAVA_TOOL_OPTIONS

```sh
# Service identification
export OTEL_SERVICE_NAME="my-service"

# Configure endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"

# Activate SDK for all JVM processes
export JAVA_TOOL_OPTIONS="-javaagent:path/to/opentelemetry-javaagent.jar"

java -jar myapp.jar
```

### Using JVM system properties

```sh
java \
  -javaagent:path/to/opentelemetry-javaagent.jar \
  -Dotel.service.name=my-service \
  -Dotel.exporter.otlp.endpoint=https://<OTLP_ENDPOINT> \
  -Dotel.exporter.otlp.headers="Authorization=Bearer YOUR_AUTH_TOKEN" \
  -jar myapp.jar
```

---

## Local development

### Console exporter

For development without a collector, use the console exporter to see telemetry in your terminal:

```sh
export OTEL_SERVICE_NAME="my-service"
export OTEL_TRACES_EXPORTER="console"
export OTEL_METRICS_EXPORTER="console"
export OTEL_LOGS_EXPORTER="console"

java -javaagent:path/to/opentelemetry-javaagent.jar -jar myapp.jar
```

This prints spans, metrics, and logs directly to stdout—useful for verifying instrumentation works before configuring a remote backend.

### Without a collector

If you set `OTEL_EXPORTER_OTLP_ENDPOINT` but have no collector running, you will see connection errors.
This is expected behavior.

**Options:**
1. Use `console` exporter during development (recommended for quick testing)
2. Run a local OpenTelemetry Collector
3. Point directly to your observability backend

---

## Resource configuration

Set `service.name`, `service.version`, and `deployment.environment.name` for every deployment.
See [resource attributes](../resources.md) for the full list of required and recommended attributes.

---

## Kubernetes setup

See [Kubernetes deployment](../platforms/k8s.md) for pod metadata injection, resource attributes, and Dash0 Kubernetes Operator guidance.

---

## Supported libraries

The auto-instrumentation agent automatically instruments:

| Category | Libraries |
|----------|-----------|
| Web frameworks | Spring Boot, Spring MVC, Spring WebFlux, Servlet, JAX-RS |
| HTTP clients | OkHttp, Apache HttpClient, java.net.HttpURLConnection |
| Database | JDBC, Hibernate, R2DBC |
| Redis | Lettuce, Jedis |
| Messaging | Kafka, RabbitMQ, JMS |
| gRPC | gRPC |
| Logging | Log4j, Logback, java.util.logging |
| AWS | AWS SDK v1, AWS SDK v2 |

Refer to the [OpenTelemetry registry](https://opentelemetry.io/ecosystem/registry/?language=java) for the complete list.

---

## Custom spans

Add business context to auto-instrumented traces:

```java
import io.opentelemetry.api.GlobalOpenTelemetry;
import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.StatusCode;
import io.opentelemetry.api.trace.Tracer;

public class OrderService {

    private static final Tracer tracer =
        GlobalOpenTelemetry.getTracer("my-service");

    public Order processOrder(Order order) {
        Span span = tracer.spanBuilder("order.process").startSpan();
        try (var scope = span.makeCurrent()) {
            span.setAttribute("order.id", order.getId());
            span.setAttribute("order.total", order.getTotal());
            Order result = saveOrder(order);
            return result;
        } catch (Exception e) {
            span.setStatus(StatusCode.ERROR);
            span.recordException(e);
            throw e;
        } finally {
            span.end();
        }
    }
}
```

---

## Structured logging

Configure your logging framework to serialize exceptions into a single structured field so that stack traces do not break the one-line-per-record contract.
See [logs](../logs.md) for general guidance on structured logging and exception stack traces.

### Logback with logstash-logback-encoder

The [logstash-logback-encoder](https://github.com/logfellow/logstash-logback-encoder) produces single-line JSON with stack traces serialized into a `stack_trace` field.

```xml
<!-- logback.xml -->
<configuration>
  <appender name="STDOUT" class="ch.qos.logback.core.ConsoleAppender">
    <encoder class="net.logstash.logback.encoder.LogstashEncoder" />
  </appender>

  <root level="INFO">
    <appender-ref ref="STDOUT" />
  </root>
</configuration>
```

The encoder automatically captures exceptions passed to `logger.error("message", exception)` and serializes the full stack trace as an escaped string in the `stack_trace` JSON field.

### Log4j2 with JSON layout

Log4j2's `JsonTemplateLayout` produces single-line JSON output with stack traces in a structured field.

```xml
<!-- log4j2.xml -->
<Configuration>
  <Appenders>
    <Console name="STDOUT" target="SYSTEM_OUT">
      <JsonTemplateLayout eventTemplateUri="classpath:EcsLayout.json" />
    </Console>
  </Appenders>

  <Loggers>
    <Root level="INFO">
      <AppenderRef ref="STDOUT" />
    </Root>
  </Loggers>
</Configuration>
```

Avoid using `PatternLayout` for production logging — it produces multi-line stack traces that break log collectors.

---

## Troubleshooting

### No telemetry appearing

**Verify the agent is loaded:**
```sh
ps aux | grep opentelemetry-javaagent
```

Look for `opentelemetry-javaagent` in the JVM arguments.
If it is missing, the agent is not attached to the JVM process.

**Enable debug logging:**
```sh
export OTEL_LOG_LEVEL="debug"
```

Or via JVM system property:
```sh
java -Dotel.javaagent.debug=true -javaagent:path/to/opentelemetry-javaagent.jar -jar myapp.jar
```

### Connection errors

```
WARN io.opentelemetry.exporter.internal.http.HttpExporter - Failed to export spans.
```

This means the SDK is working but cannot reach the collector:
- **No collector running**: Start a local collector or use `OTEL_TRACES_EXPORTER=console`
- **Wrong endpoint**: Check `OTEL_EXPORTER_OTLP_ENDPOINT` is correct
- **Port mismatch**: gRPC uses 4317, HTTP uses 4318

### Agent not instrumenting libraries

**Symptom**: Agent loads but specific libraries are not instrumented

**Fix**: Verify the library version is supported by checking the [OpenTelemetry registry](https://opentelemetry.io/ecosystem/registry/?language=java).
Some very old library versions may not be covered by the auto-instrumentation.

### Conflicts with other Java agents

Running multiple Java agents (e.g., APM agents) alongside the OpenTelemetry agent can cause conflicts.
Remove other Java agents before attaching the OpenTelemetry agent.

---

## Resources

- [OpenTelemetry Java Documentation](https://opentelemetry.io/docs/languages/java/)
- [Java Agent Releases](https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases)
- [Environment Variable Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)

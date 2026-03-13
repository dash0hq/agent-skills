---
title: "Scala Instrumentation"
impact: HIGH
tags:
  - scala
  - jvm
  - backend
  - server
---

# Scala Instrumentation

Instrument Scala applications to generate traces, logs, and metrics for deep insights into behavior and performance.

## Use cases

- **HTTP Request Monitoring**: Understand outgoing and incoming HTTP requests through traces and metrics, with drill-downs to database level
- **Database Performance**: Observe which database statements execute and measure their duration for optimization
- **Error Detection**: Reveal uncaught errors and the context in which they happened

## Installation

Scala runs on the JVM, so it uses the same OpenTelemetry Java agent as Java applications.
Download the agent JAR:

```sh
wget https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar
```

**Note**: The javaagent.jar contains both the agent and instrumentation libraries, enabling automatic instrumentation without modifying source code.

For sbt projects, add a task to download the agent:

```scala
// build.sbt
lazy val downloadAgent = taskKey[File]("Download the OpenTelemetry Java agent")
downloadAgent := {
  val agentFile = target.value / "opentelemetry-javaagent.jar"
  if (!agentFile.exists()) {
    val url = new java.net.URL(
      "https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar"
    )
    IO.transfer(url.openStream(), agentFile)
  }
  agentFile
}
```

Add the OpenTelemetry API dependency for custom spans:

```scala
// build.sbt
libraryDependencies += "io.opentelemetry" % "opentelemetry-api" % "1.47.0"
```

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

**Note**: The Java agent defaults all exporters to `otlp` and the protocol to `http/protobuf`.

### Where to get configuration values

1. **OTLP Endpoint**: Your observability platform's OTLP endpoint
   - In Dash0: [Settings → Organization → Endpoints](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Cb29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
   - Format: `https://<region>.your-platform.com`
2. **Auth Token**: API token for telemetry ingestion
   - In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)
3. **Service Name**: Choose a descriptive name (e.g., `order-api`, `checkout-service`)

## Configuration

### 1. Activate the SDK

The SDK is activated by attaching the Java agent to the JVM.

**Via sbt `javaOptions`:**
```scala
// build.sbt
run / javaOptions += s"-javaagent:${target.value}/opentelemetry-javaagent.jar"
run / fork := true
```

Setting `fork := true` is required — sbt runs tasks in-process by default, and the javaagent must attach at JVM startup.

**Via `JAVA_TOOL_OPTIONS` environment variable:**
```sh
export JAVA_TOOL_OPTIONS="-javaagent:path/to/opentelemetry-javaagent.jar"
```
**Note**: The `JAVA_TOOL_OPTIONS` approach applies the agent to every JVM process started in that shell session, including sbt itself and any compilation forked by sbt.
It should be used only when you are sure all the JVM processes that will be affected need to be instrumented.

**Via `-javaagent` JVM parameter (fat JAR or assembly):**
```sh
java -javaagent:path/to/opentelemetry-javaagent.jar -jar myapp-assembly.jar
```

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

## Complete setup

### Using environment variables

```sh
# Service identification
export OTEL_SERVICE_NAME="my-service"

# Configure endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"

# Run with sbt (requires fork := true and javaOptions configured)
sbt run
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

java -jar myapp-assembly.jar
```

### Using JVM system properties

```sh
java \
  -javaagent:path/to/opentelemetry-javaagent.jar \
  -Dotel.service.name=my-service \
  -Dotel.exporter.otlp.endpoint=https://<OTLP_ENDPOINT> \
  -Dotel.exporter.otlp.headers="Authorization=Bearer YOUR_AUTH_TOKEN" \
  -jar myapp-assembly.jar
```

## Local development

### Console exporter

For development without a collector, use the console exporter to see telemetry in your terminal:

```sh
export OTEL_SERVICE_NAME="my-service"
export OTEL_TRACES_EXPORTER="console"
export OTEL_METRICS_EXPORTER="console"
export OTEL_LOGS_EXPORTER="console"

sbt run
```

This prints spans, metrics, and logs directly to stdout—useful for verifying instrumentation works before configuring a remote backend.

### Without a collector

If you set `OTEL_EXPORTER_OTLP_ENDPOINT` but have no collector running, you will see connection errors.
This is expected behavior.

**Options:**
1. Use `console` exporter during development (recommended for quick testing)
2. Run a local OpenTelemetry Collector
3. Point directly to your observability backend

## Resource configuration

Set `service.name`, `service.version`, and `deployment.environment.name` for every deployment.
See [resource attributes](../resources.md) for the full list of required and recommended attributes.

## Kubernetes setup

See [Kubernetes deployment](../platforms/k8s.md) for pod metadata injection, resource attributes, and Dash0 Kubernetes Operator guidance.

## Supported libraries

The auto-instrumentation agent automatically instruments Scala libraries that are built on Java frameworks.

| Category | Libraries |
|----------|-----------|
| HTTP frameworks | Akka HTTP, Play Framework, http4s (Blaze/Ember backends), Finatra |
| HTTP clients | Akka HTTP client, sttp (backed by OkHttp, Apache HttpClient, or Java HttpClient), http4s client |
| Database | Slick (via JDBC), Doobie (via JDBC), Quill (via JDBC), ScalikeJDBC |
| Redis | Lettuce, Jedis (via Java instrumentation) |
| Messaging | Kafka (via Java client), Alpakka Kafka, RabbitMQ (via Java client), ZIO Kafka |
| gRPC | ScalaPB (via grpc-java), Akka gRPC |
| Logging | Logback, Log4j2, SLF4J (all via Java instrumentation) |
| AWS | AWS SDK v1, AWS SDK v2 (via Java instrumentation) |

The agent instruments at the JVM bytecode level.
Any Scala library that delegates to an instrumented Java library (e.g., JDBC, Netty, gRPC-Java) is automatically covered.

Refer to the [OpenTelemetry registry](https://opentelemetry.io/ecosystem/registry/?language=java) for the complete list.

## Custom spans

Add business context to auto-instrumented traces.
The OpenTelemetry Java API is used directly from Scala.

### Basic pattern

```scala
import io.opentelemetry.api.GlobalOpenTelemetry
import io.opentelemetry.api.trace.{Span, StatusCode}

private def stackTraceAsString(t: Throwable): String = {
  val sw = new java.io.StringWriter()
  t.printStackTrace(new java.io.PrintWriter(sw))
  sw.toString
}

object OrderService {

  private val tracer = GlobalOpenTelemetry.getTracer("my-service")

  def processOrder(order: Order): Order = {
    val span = tracer.spanBuilder("order.process").startSpan()
    val scope = span.makeCurrent()
    try {
      span.setAttribute("order.id", order.id)
      span.setAttribute("order.total", order.total)
      saveOrder(order)
    } catch {
      case e: Exception =>
        span.setStatus(StatusCode.ERROR, e.getMessage)
        // Record exception as a log record — see spans.md#recording-exceptions
        val ctx = span.getSpanContext
        logger.atError()
          .addKeyValue("trace_id", ctx.getTraceId)
          .addKeyValue("span_id", ctx.getSpanId)
          .addKeyValue("exception.type", e.getClass.getName)
          .addKeyValue("exception.message", e.getMessage)
          .addKeyValue("exception.stacktrace", stackTraceAsString(e))
          .log("order.process.failed")
        throw e
    } finally {
      scope.close()
      span.end()
    }
  }
}
```

### Using `scala.util.Using` (Scala 2.13+)

`scala.util.Using` manages scope closure automatically.
Wrap the scope in a `Using.resource` block to avoid forgetting `scope.close()`:

```scala
import io.opentelemetry.api.GlobalOpenTelemetry
import io.opentelemetry.api.trace.{Span, StatusCode}
import scala.util.Using

object OrderService {

  private val tracer = GlobalOpenTelemetry.getTracer("my-service")

  def processOrder(order: Order): Order = {
    val span = tracer.spanBuilder("order.process").startSpan()
    try {
      Using.resource(span.makeCurrent()) { _ =>
        span.setAttribute("order.id", order.id)
        span.setAttribute("order.total", order.total)
        saveOrder(order)
      }
    } catch {
      case e: Exception =>
        span.setStatus(StatusCode.ERROR, e.getMessage)
        // Record exception as a log record — see spans.md#recording-exceptions
        val ctx = span.getSpanContext
        logger.atError()
          .addKeyValue("trace_id", ctx.getTraceId)
          .addKeyValue("span_id", ctx.getSpanId)
          .addKeyValue("exception.type", e.getClass.getName)
          .addKeyValue("exception.message", e.getMessage)
          .addKeyValue("exception.stacktrace", stackTraceAsString(e))
          .log("order.process.failed")
        throw e
    } finally {
      span.end()
    }
  }
}
```

### Tracing with `Future`

When working with `Future`, the span must be ended after the `Future` completes, not when `startSpan()` returns.
Failing to do this produces zero-duration spans that end before the async work finishes.

```scala
import io.opentelemetry.api.GlobalOpenTelemetry
import io.opentelemetry.api.trace.{Span, StatusCode}
import io.opentelemetry.context.Context
import scala.concurrent.{ExecutionContext, Future}

object OrderService {

  private val tracer = GlobalOpenTelemetry.getTracer("my-service")

  def processOrderAsync(order: Order)(implicit ec: ExecutionContext): Future[Order] = {
    val span = tracer.spanBuilder("order.process").startSpan()
    val scope = span.makeCurrent()

    // Capture the OTel context before entering the Future.
    val otelContext = Context.current()

    // Close the scope on the calling thread; the span stays open.
    scope.close()

    Future {
      // Re-attach the context inside the Future's execution thread.
      val innerScope = otelContext.makeCurrent()
      try {
        span.setAttribute("order.id", order.id)
        span.setAttribute("order.total", order.total)
        saveOrder(order)
      } finally {
        innerScope.close()
      }
    }.transform(
      result => { span.end(); result },
      error  => {
        span.setStatus(StatusCode.ERROR, error.getMessage)
        // Record exception as a log record — see spans.md#recording-exceptions
        val ctx = span.getSpanContext
        logger.atError()
          .addKeyValue("trace_id", ctx.getTraceId)
          .addKeyValue("span_id", ctx.getSpanId)
          .addKeyValue("exception.type", error.getClass.getName)
          .addKeyValue("exception.message", error.getMessage)
          .addKeyValue("exception.stacktrace", stackTraceAsString(error))
          .log("order.process.failed")
        span.end()
        error
      }
    )
  }
}
```

Key points:
- Close the scope on the calling thread immediately — scopes are thread-local and must not cross thread boundaries.
- Capture `Context.current()` before the `Future` block, then call `otelContext.makeCurrent()` inside it so that child spans created within the `Future` are linked to the correct parent.
- End the span in the `transform` callback to ensure it covers the full async execution.

### Retrieving the active span

Auto-instrumentation creates spans you do not control directly (e.g., the `SERVER` span for an HTTP request).
To enrich these spans with business context or set their status, retrieve the active span from the current context.
See [adding attributes to auto-instrumented spans](../spans.md#adding-attributes-to-auto-instrumented-spans) for when to use this pattern.

```scala
import io.opentelemetry.api.trace.Span

def createOrder(request: OrderRequest): Order = {
  val span = Span.current()
  span.setAttribute("order.id", request.orderId)
  span.setAttribute("tenant.id", request.tenantId)
  // ... handler logic
}
```

`Span.current()` returns a non-recording span if no span is active.
Calling `setAttribute` or `setStatus` on a non-recording span is a no-op, so no null check is needed.

In Go-style context-passing code (e.g., when using `Context` explicitly), use `Span.fromContext` instead:

```scala
import io.opentelemetry.api.trace.Span
import io.opentelemetry.context.Context

val span = Span.fromContext(Context.current())
```

### Span status rules

See [span status code](../spans.md#span-status-code) for the full rules.
This section shows how to apply them in Scala.

#### Always include a status message with `ERROR`

The second argument to `setStatus` is the status message.
It must contain the error type and a short explanation — enough to understand the failure without opening the full trace.

```scala
// BAD: no status message
span.setStatus(StatusCode.ERROR)

// BAD: generic message with no diagnostic value
span.setStatus(StatusCode.ERROR, "something went wrong")

// GOOD: specific message with error type and context
span.setStatus(StatusCode.ERROR, s"${e.getClass.getSimpleName}: ${e.getMessage}")
```

Do not include stack traces in the status message.
Record those in a log record with `exception.stacktrace` instead.

```scala
import java.io.{PrintWriter, StringWriter}

// BAD: stack trace in the status message
val sw = new StringWriter()
e.printStackTrace(new PrintWriter(sw))
span.setStatus(StatusCode.ERROR, sw.toString)

// GOOD: short message only
span.setStatus(StatusCode.ERROR, e.getMessage)
```

#### Use `OK` only for confirmed success

Set status to `OK` when application logic has explicitly verified the operation succeeded.
Leave status `UNSET` if the code simply did not encounter an error.

```scala
// GOOD: explicit confirmation from downstream
val response = httpClient.send(request, HttpResponse.BodyHandlers.ofString())
if (response.statusCode() == 200) {
  span.setStatus(StatusCode.OK)
}

// BAD: setting OK speculatively
span.setStatus(StatusCode.OK)
someMethod() // might still fail after this point
```

## Structured logging

Configure your logging framework to serialize exceptions into a single structured field so that stack traces do not break the one-line-per-record contract.
See [logs](../logs.md) for general guidance on structured logging and exception stack traces.

Scala applications typically use SLF4J-based loggers.
The same Logback and Log4j2 configurations from the [Java instrumentation guide](./java.md#structured-logging) apply.

### Logback with logstash-logback-encoder

The [logstash-logback-encoder](https://github.com/logfellow/logstash-logback-encoder) produces single-line JSON with stack traces serialized into a `stack_trace` field.

Add the dependency:

```scala
// build.sbt
libraryDependencies += "net.logstash.logback" % "logstash-logback-encoder" % "8.0" % Runtime
```

```xml
<!-- src/main/resources/logback.xml -->
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

### scala-logging

[scala-logging](https://github.com/lightbend-labs/scala-logging) is a thin Scala wrapper around SLF4J.
It uses the same underlying Logback or Log4j2 configuration, so the JSON formatting rules above apply.

```scala
import com.typesafe.scalalogging.LazyLogging

object OrderService extends LazyLogging {
  def processOrder(order: Order): Unit = {
    try {
      saveOrder(order)
    } catch {
      case e: Exception =>
        logger.error("order.failed", e)
    }
  }
}
```

Pass the exception as the second argument to `logger.error` so that the JSON encoder captures it as a structured field.
Do not call `e.getStackTrace` or `e.printStackTrace` and log the result as a string — this produces multi-line output.

## Graceful shutdown

Scala applications use the same Java agent as Java.
The agent registers a JVM shutdown hook automatically.
When the JVM receives `SIGTERM` or `sys.exit()` is called, the hook flushes all pending spans, metrics, and log records before the process terminates.
No additional code is needed.

If you use a programmatic SDK setup (without the agent), register a shutdown hook manually:

```scala
sys.addShutdownHook {
  tracerProvider.close()
  meterProvider.close()
  loggerProvider.close()
}
```

`close()` calls `shutdown()` internally, which flushes pending batches and releases resources.

## Troubleshooting

### No telemetry appearing

**Verify the agent is loaded:**
```sh
ps aux | grep opentelemetry-javaagent
```

Look for `opentelemetry-javaagent` in the JVM arguments.
If it is missing, the agent is not attached to the JVM process.

**Verify sbt is forking:**
If using sbt, ensure `run / fork := true` is set.
Without forking, `javaOptions` are ignored and the agent is not attached.

**Enable debug logging:**
```sh
export OTEL_LOG_LEVEL="debug"
```

Or via JVM system property:
```sh
java -Dotel.javaagent.debug=true -javaagent:path/to/opentelemetry-javaagent.jar -jar myapp-assembly.jar
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

**Symptom**: Agent loads but specific libraries are not instrumented.

**Fix**: Verify the library version is supported by checking the [OpenTelemetry registry](https://opentelemetry.io/ecosystem/registry/?language=java).
Some very old library versions may not be covered by the auto-instrumentation.

### Zero-duration spans with `Future`

**Symptom**: Spans appear in traces but have zero or near-zero duration, and all the actual work shows up outside the span.

**Fix**: The span is being ended on the calling thread before the `Future` completes.
End the span in a `transform` or `onComplete` callback — see [tracing with Future](#tracing-with-future).

### Conflicts with other Java agents

Running multiple Java agents (e.g., APM agents) alongside the OpenTelemetry agent can cause conflicts.
Remove other Java agents before attaching the OpenTelemetry agent.

## Resources

- [OpenTelemetry Java Documentation](https://opentelemetry.io/docs/languages/java/)
- [Java Agent Releases](https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases)
- [Environment Variable Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)

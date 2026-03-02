---
title: "Go Instrumentation"
impact: HIGH
tags:
  - go
  - backend
  - server
---

# Go Instrumentation

Instrument Go applications to generate traces, logs, and metrics for deep insights into behavior and performance.

## Use cases

- **HTTP Request Monitoring**: Understand outgoing and incoming HTTP requests through traces and metrics, with drill-downs to database level
- **Database Performance**: Observe which database statements execute and measure their duration for optimization
- **Error Detection**: Reveal uncaught errors and the context in which they happened

---

## Installation

Go does not have a single auto-instrumentation package.
Instead, you install individual instrumentation libraries for each framework and library you use, along with the core SDK and exporter packages.

```bash
# Core SDK and API
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/sdk

# gRPC exporters
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc
```

Install instrumentation packages for the libraries you use from the [OpenTelemetry Registry](https://opentelemetry.io/ecosystem/registry/?language=go).

**Note**: Installing the packages alone is insufficient—you must write initialization code to activate the SDK AND enable exporters.

---

## Environment variables

All environment variables that control the SDK behavior:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_SERVICE_NAME` | Yes | `unknown_service` | Identifies your service in telemetry data |
| `OTEL_TRACES_EXPORTER` | Yes | `none` | **Must set to `otlp`** to export traces |
| `OTEL_METRICS_EXPORTER` | No | `none` | Set to `otlp` to export metrics |
| `OTEL_LOGS_EXPORTER` | No | `none` | Set to `otlp` to export logs |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Yes | `http://localhost:4317` | OTLP collector endpoint |
| `OTEL_EXPORTER_OTLP_HEADERS` | No | - | Headers for authentication (e.g., `Authorization=Bearer TOKEN`) |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | No | `grpc` | Protocol: `grpc`, `http/protobuf`, or `http/json` |
| `OTEL_RESOURCE_ATTRIBUTES` | No | - | Additional resource attributes (e.g., `deployment.environment=production`) |

**Critical**: The gRPC exporters read these environment variables automatically, but you must initialize the exporters in code for the variables to take effect.

### Where to get configuration values

1. **OTLP Endpoint**: Your observability platform's OTLP endpoint
   - In Dash0: [Settings → Organization → Endpoints](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Db29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
   - Format: `https://<region>.your-platform.com`
2. **Auth Token**: API token for telemetry ingestion
   - In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)
3. **Service Name**: Choose a descriptive name (e.g., `order-api`, `checkout-service`)

---

## Configuration

### 1. Activate the SDK

Unlike Node.js, Go requires explicit initialization code.
Create an initialization function that sets up the trace, metric, and log providers:

```go
package main

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func initTelemetry(ctx context.Context) (func(), error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("my-service"),
		),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, err
	}

	// Trace exporter
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Metric exporter
	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// Log exporter
	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	shutdown := func() {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		_ = lp.Shutdown(ctx)
	}

	return shutdown, nil
}

func main() {
	ctx := context.Background()
	shutdown, err := initTelemetry(ctx)
	if err != nil {
		log.Fatalf("failed to initialize telemetry: %v", err)
	}
	defer shutdown()

	// Your application code here
}
```

The gRPC exporters automatically read `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`, and other environment variables.

### 2. Set service name

```bash
export OTEL_SERVICE_NAME="my-service"
```

### 3. Enable exporters

**This step is required** — without it, no telemetry is sent:

```bash
# Required for traces
export OTEL_TRACES_EXPORTER="otlp"

# Optional: also export metrics and logs
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"
```

### 4. Configure endpoint

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"
```

### 5. Optional: target specific dataset

```bash
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN,Dash0-Dataset=my-dataset"
```

---

## Complete setup

### Using environment variables

```bash
# Service identification
export OTEL_SERVICE_NAME="my-service"

# Enable exporters (required!)
export OTEL_TRACES_EXPORTER="otlp"
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"

# Configure endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"

go run .
```

### Using a .env file with a wrapper

Go does not natively load `.env` files.
Use a library like [godotenv](https://github.com/joho/godotenv) or source the file before running:

**.env.local:**
```bash
OTEL_SERVICE_NAME=my-service
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=https://<OTLP_ENDPOINT>
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer YOUR_AUTH_TOKEN
```

**Run with:**
```bash
source .env.local && go run .
```

### Using a Makefile

Add instrumented targets to your `Makefile`:

```makefile
.PHONY: run run-otel run-otel-console

run:
	go run .

run-otel:
	source .env.local && go run .

run-otel-console:
	OTEL_SERVICE_NAME=my-service \
	OTEL_TRACES_EXPORTER=console \
	go run .
```

**Usage:**
```bash
make run-otel          # Run with OTLP export to backend
make run-otel-console  # Run with console output (no collector needed)
```

---

## Local development

### Console exporter

For development without a collector, use the console exporter to see telemetry in your terminal.
Replace the gRPC exporters with stdout exporters in your initialization code:

```go
import (
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
)

traceExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
metricExporter, err := stdoutmetric.New()
```

Install the stdout exporter packages:
```bash
go get go.opentelemetry.io/otel/exporters/stdout/stdouttrace
go get go.opentelemetry.io/otel/exporters/stdout/stdoutmetric
```

This prints spans and metrics directly to stdout—useful for verifying instrumentation works before configuring a remote backend.

### Without a collector

If you configure the gRPC exporter but have no collector running, you will see connection errors.
This is expected behavior:

```
rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing: dial tcp 127.0.0.1:4317: connect: connection refused"
```

**Options:**
1. Use stdout exporters during development (recommended for quick testing)
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

Go uses individual instrumentation packages from the [OpenTelemetry Registry](https://opentelemetry.io/ecosystem/registry/?language=go).
Install only the packages you need for the frameworks and libraries your application uses:

| Category | Libraries |
|----------|-----------|
| HTTP | net/http, gin, echo, fiber, chi |
| Database | database/sql, pgx, go-sql-driver/mysql, mongo-driver |
| gRPC | google.golang.org/grpc |
| Messaging | sarama (Kafka), amqp091-go |
| AWS | aws-sdk-go-v2 |
| Logging | slog (via bridges) |
| Runtime | runtime metrics (automatic with SDK) |

Refer to the [OpenTelemetry Go instrumentation registry](https://opentelemetry.io/ecosystem/registry/?language=go) for the complete list.

### Example: instrumenting net/http

```bash
go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
```

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// Wrap an HTTP handler
handler := otelhttp.NewHandler(mux, "server")

// Wrap an HTTP client transport
client := &http.Client{
	Transport: otelhttp.NewTransport(http.DefaultTransport),
}
```

---

## Custom spans

Add business context to instrumented traces:

```go
import "go.opentelemetry.io/otel"

var tracer = otel.Tracer("my-service")

func processOrder(ctx context.Context, order Order) error {
	ctx, span := tracer.Start(ctx, "order.process")
	defer span.End()

	span.SetAttributes(
		attribute.String("order.id", order.ID),
		attribute.Float64("order.total", order.Total),
	)

	if err := saveOrder(ctx, order); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}
```

---

## Structured logging

Configure your logging framework to serialize errors into a single structured field so that stack traces do not break the one-line-per-record contract.
See [logs](../logs.md) for general guidance on structured logging and exception stack traces.

### slog with JSON handler

The standard library `slog` package with `slog.NewJSONHandler` produces single-line JSON output.
Errors logged as attributes are serialized inline.

```go
import (
	"log/slog"
	"os"
)

logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

if err != nil {
	logger.Error("order.failed",
		"error", err.Error(),
		"order_id", order.ID,
	)
}
```

Go errors do not include stack traces by default.
If you use a library that adds stack traces (e.g., `pkg/errors` or `cockroachdb/errors`), format the error with `fmt.Sprintf("%+v", err)` and log it as a single string field to avoid multi-line output.

### zerolog

[zerolog](https://github.com/rs/zerolog) produces single-line JSON by default and handles errors as structured fields.

```go
import "github.com/rs/zerolog/log"

if err != nil {
	log.Error().
		Err(err).
		Str("order_id", order.ID).
		Msg("order.failed")
}
```

zerolog serializes the error into an `"error"` field as a single string value.

---

## Troubleshooting

### No telemetry appearing

**Check exporters are enabled:**
```bash
echo $OTEL_TRACES_EXPORTER  # Should be "otlp" or "console", not empty
```

The SDK defaults `OTEL_TRACES_EXPORTER` to `none`, which silently discards all telemetry.

**Verify SDK is initialized:**
Ensure `initTelemetry()` (or equivalent) is called at the start of `main()` before any instrumented code runs.

### Enable debug logging

Set the `OTEL_LOG_LEVEL` environment variable or enable verbose logging in your exporter configuration:

```go
traceExporter, err := otlptracegrpc.New(ctx,
	otlptracegrpc.WithInsecure(), // For local development only
)
```

Use Go's standard `log` package to verify that spans are created and exported.

### Connection refused errors

```
rpc error: code = Unavailable desc = connection error: desc = "transport: Error while dialing: dial tcp 127.0.0.1:4317: connect: connection refused"
```

This means the SDK is working but cannot reach the collector:
- **No collector running**: Start a local collector or use stdout exporters
- **Wrong endpoint**: Check `OTEL_EXPORTER_OTLP_ENDPOINT` is correct
- **Port mismatch**: gRPC uses 4317, HTTP uses 4318

### Spans not appearing for a specific library

**Symptom**: SDK initializes but no spans appear for HTTP, database, or other calls.

**Fix**: Ensure you have installed and registered the correct instrumentation package for that library.
Each library requires its own instrumentation wrapper from `go.opentelemetry.io/contrib/instrumentation/`.

### Context propagation issues

**Symptom**: Spans are created but not connected into traces.

**Fix**: Always pass `context.Context` through your call chain.
Go instrumentation relies on context propagation to link parent and child spans.

---

## Resources

- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/languages/go/getting-started/)
- [OpenTelemetry Go Instrumentation Registry](https://opentelemetry.io/ecosystem/registry/?language=go)
- [Environment Variable Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)

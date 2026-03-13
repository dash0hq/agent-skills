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

## Resource configuration

Set `service.name`, `service.version`, and `deployment.environment.name` for every deployment.
See [resource attributes](../resources.md) for the full list of required and recommended attributes.

## Kubernetes setup

See [Kubernetes deployment](../platforms/k8s.md) for pod metadata injection, resource attributes, and Dash0 Kubernetes Operator guidance.

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
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "order.process.failed",
			"trace_id", span.SpanContext().TraceID().String(),
			"span_id", span.SpanContext().SpanID().String(),
			"exception.type", fmt.Sprintf("%T", err),
			"exception.message", err.Error(),
		)
		return err
	}

	return nil
}
```

### Retrieving the active span

Auto-instrumentation creates spans you do not control directly (e.g., the `SERVER` span created by `otelhttp`).
To enrich these spans with business context or set their status, retrieve the span from the request context.
See [adding attributes to auto-instrumented spans](../spans.md#adding-attributes-to-auto-instrumented-spans) for when to use this pattern.

Go does not have a global "current span" — the span is always carried in a `context.Context`.
Use `trace.SpanFromContext` to retrieve it:

```go
import "go.opentelemetry.io/otel/trace"

func handleOrder(w http.ResponseWriter, r *http.Request) {
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		attribute.String("order.id", order.ID),
		attribute.String("tenant.id", r.Header.Get("X-Tenant-Id")),
	)
	// ... handler logic
}
```

`trace.SpanFromContext` returns a non-recording span if no span is in the context.
Calling `SetAttributes` or `SetStatus` on a non-recording span is a no-op, so no nil check is needed.

### Span status rules

See [span status code](../spans.md#span-status-code) for the full rules.
This section shows how to apply them in Go.

#### Always include a status message with `ERROR`

The second argument to `SetStatus` is the status message.
It must contain the error type and a short explanation — enough to understand the failure without opening the full trace.

```go
// BAD: no status message
span.SetStatus(codes.Error, "")

// BAD: generic message with no diagnostic value
span.SetStatus(codes.Error, "something went wrong")

// GOOD: specific message with error type and context
span.SetStatus(codes.Error, fmt.Sprintf("*net.OpError: dial tcp %s: connection refused", addr))
```

For wrapped errors, use the outermost message.
Do not call `fmt.Sprintf("%+v", err)` in the status message — stack traces belong in a log record with `exception.stacktrace`, not in the status message.

```go
// BAD: stack trace in the status message
span.SetStatus(codes.Error, fmt.Sprintf("%+v", err))

// GOOD: short message only
span.SetStatus(codes.Error, err.Error())
```

#### Set the status message on the server span from `otelhttp`

`otelhttp` sets the SERVER span status to `ERROR` for 5xx responses, but it cannot populate the status message because it only sees the HTTP status code, not the application error.
Without an explicit `SetStatus` call in the handler, the root span of every error trace has no diagnostic information.

Always set the status message on the server span inside the handler when returning a 5xx response.
Use `trace.SpanFromContext` to retrieve the span that `otelhttp` created:

```go
import (
	"net/http"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func handleOrder(w http.ResponseWriter, r *http.Request) {
	order, err := decodeOrder(r)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := processOrder(r.Context(), order); err != nil {
		// Set the status message on the SERVER span created by otelhttp.
		trace.SpanFromContext(r.Context()).SetStatus(codes.Error, err.Error())
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
```

```go
// BAD: relies on otelhttp alone — root span says "Error" with no message
func handleOrder(w http.ResponseWriter, r *http.Request) {
	if err := processOrder(r.Context(), order); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
}
```

#### Use `OK` only for confirmed success

Set status to `OK` when application logic has explicitly verified the operation succeeded.
Leave status `UNSET` if the code simply did not encounter an error.

```go
// GOOD: explicit confirmation from downstream
resp, err := client.Do(req)
if err != nil {
	span.SetStatus(codes.Error, err.Error())
	return err
}
if resp.StatusCode == http.StatusOK {
	span.SetStatus(codes.Ok, "")
}

// BAD: setting OK speculatively
span.SetStatus(codes.Ok, "")
return someFunction(ctx) // might still fail after this point
```

## Context propagation

**This section applies only to distributed-traces instrumentation.**
If the application uses only logs and/or metrics, context propagation is not required.

Go carries the active span inside a `context.Context` value.
Every function in a call chain that should participate in a trace must accept a `context.Context` as its first parameter and pass it to downstream calls.
If any function in the chain drops or ignores the context, the trace breaks at that point and child spans become orphaned roots.

### Ensuring every function accepts a context

When adding tracing to an existing codebase, audit every function on the request path.
Any function that does not already take a `context.Context` must be refactored before it can carry trace context.

Add `ctx context.Context` as the **first parameter** (the standard Go convention):

```go
// BEFORE: no context — trace breaks here
func getUser(id string) (*User, error) {
	return db.QueryUser(id)
}

// AFTER: context flows through — child spans link to the parent
func getUser(ctx context.Context, id string) (*User, error) {
	return db.QueryUser(ctx, id)
}
```

Update every call site to pass the context:

```go
// BEFORE
user, err := getUser(order.UserID)

// AFTER
user, err := getUser(ctx, order.UserID)
```

### Common context-propagation breaks

Apply the following rules when the code matches one of these patterns.

#### Goroutines

Pass the parent context (or a derived context) to goroutines explicitly.
Do **not** rely on closure capture of a `ctx` variable that may be cancelled before the goroutine runs.

```go
// GOOD: pass context explicitly
go func(ctx context.Context) {
	processAsync(ctx, item)
}(ctx)

// BAD: closure captures ctx that may be cancelled by the caller
go func() {
	processAsync(ctx, item)
}()
```

If the goroutine must outlive the request (e.g., background work), create a new root context with `context.Background()` and link it to the original span:

```go
asyncCtx := context.Background()
asyncCtx, span := tracer.Start(asyncCtx, "async.process",
	trace.WithLinks(trace.LinkFromContext(ctx)),
)
go func() {
	defer span.End()
	processAsync(asyncCtx, item)
}()
```

#### Callbacks and interface implementations

When a framework or library defines a callback or interface method without a `context.Context` parameter, the trace context cannot flow through it.
Check whether the framework offers a context-aware variant (e.g., `http.Handler` carries context in `*http.Request`).

If no context-aware API exists, store the context before the callback and retrieve it inside:

```go
// Store context in a struct field before the callback
type handler struct {
	ctx context.Context
}

func (h *handler) OnMessage(msg Message) {
	ctx, span := tracer.Start(h.ctx, "message.process")
	defer span.End()
	// ...
}
```

#### Channel consumers

When reading from a channel, the producing side must send the context alongside the data.
Define a wrapper struct that pairs the payload with its context:

```go
type work struct {
	ctx  context.Context
	item Item
}

// Producer
ch <- work{ctx: ctx, item: item}

// Consumer
w := <-ch
ctx, span := tracer.Start(w.ctx, "consume.item")
defer span.End()
process(ctx, w.item)
```

### Verifying context propagation

After refactoring, verify that all spans in a request are connected into a single trace.
Export to a backend or use the console exporter and confirm that every span shares the same `TraceID` and has the expected `ParentSpanID`.
Orphaned root spans (spans with no parent that should have one) indicate a broken context chain.

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

## Graceful shutdown

Go uses a programmatic SDK setup, so the application must shut down providers explicitly.
The `initTelemetry` function in the [configuration section](#activate-the-sdk) returns a `shutdown` closure that flushes and shuts down all providers.

`os.Exit`, `log.Fatal`, and unhandled signals bypass `defer` — so relying on `defer shutdown()` alone loses telemetry in most real shutdown scenarios.
Call `shutdown()` explicitly in the signal handler, before the process exits:

```go
func main() {
	ctx := context.Background()
	shutdown, err := initTelemetry(ctx)
	if err != nil {
		log.Fatalf("failed to initialize telemetry: %v", err)
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	srv := &http.Server{Addr: ":8080", Handler: handler}
	go func() { _ = srv.ListenAndServe() }()

	<-ctx.Done()
	_ = srv.Shutdown(context.Background())
	shutdown()
}
```

Each provider's `Shutdown` method flushes pending batches and releases resources.
The call blocks until export completes or the context deadline expires.

For short-lived programs (CLI tools, batch jobs) that return from `main` normally, `defer shutdown()` is sufficient.

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

**Symptom**: Spans are created but not connected into traces (orphaned root spans).

**Fix**: Every function on the request path must accept and forward a `context.Context` struct.
See [context propagation](#context-propagation) for refactoring patterns covering goroutines, callbacks, and channel consumers.

## Resources

- [OpenTelemetry Go Documentation](https://opentelemetry.io/docs/languages/go/getting-started/)
- [OpenTelemetry Go Instrumentation Registry](https://opentelemetry.io/ecosystem/registry/?language=go)
- [Environment Variable Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)

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
```

Install instrumentation packages for the libraries you use from the [OpenTelemetry Registry](https://opentelemetry.io/ecosystem/registry/?language=go).

**Note**: Installing the packages alone is insufficient—you must write initialization code to activate the SDK AND enable exporters.

### Verifying dependencies

Never hand-write a `require` line in `go.mod` with a version from memory; verify the module against the module proxy first, per [verify-dependencies](../verify-dependencies.md):

```bash
go list -m -versions go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
```

The module proxy lists every published version; a module it does not know is not a published instrumentation.
Add modules with `go get <module>@latest` followed by `go mod tidy`, which resolves real versions and prunes the manifest.

The newest published release is not always usable: a module version's `go` directive must be satisfied by the project's toolchain — the `go` line in `go.mod` **and** the Go version of the build environment (Go container base images set `GOTOOLCHAIN=local`, so a builder like `golang:1.24` cannot auto-download the newer toolchain a dependency demands, and `go mod download` fails with `requires go >= ...`).
A version's directive is one request away:

```bash
curl -s https://proxy.golang.org/go.opentelemetry.io/otel/@v/v1.45.0.mod | grep '^go '
```

When the newest release requires a newer toolchain than the project has, walk `go list -m -versions` down to the newest release whose directive the toolchain satisfies and pin that — or upgrade the toolchain deliberately (go.mod directive, builder images, CI) as its own visible change.

`go.sum` must move with `go.mod`: its hashes cannot be hand-written, so after any `go.mod` edit, regenerate it with `go mod tidy`.
A `go.mod` that requires modules missing from `go.sum` fails every subsequent build:

```text
main.go:23:2: missing go.sum entry for module providing package go.opentelemetry.io/otel/sdk/resource
```

When the `go.mod` edit happens somewhere the Go toolchain cannot run — per [verify-dependencies](../verify-dependencies.md#keeping-the-lockfile-in-step) — and the project builds in a container, make the builder stage regenerate the module metadata itself before compiling:

```dockerfile
FROM golang:1.24 AS builder
WORKDIR /src
COPY . .
RUN go mod tidy
RUN go build -o /service .
```

## Environment variables

All environment variables that control the SDK behavior:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_SERVICE_NAME` | Yes | `unknown_service` | Identifies your service in telemetry data |
| `OTEL_TRACES_EXPORTER` | Yes | `none` | **Must set to `otlp`** to export traces |
| `OTEL_METRICS_EXPORTER` | No | `none` | Set to `otlp` to export metrics |
| `OTEL_LOGS_EXPORTER` | No | `none` | Leave unset: application logs stay on stdout per [logs](../logs.md) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Yes | `http://localhost:4317` | OTLP collector endpoint |
| `OTEL_EXPORTER_OTLP_HEADERS` | No | - | Headers for authentication (e.g., `Authorization=Bearer TOKEN`) |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | No | `grpc` | Protocol: `grpc`, `http/protobuf`, or `http/json` |
| `OTEL_RESOURCE_ATTRIBUTES` | No | - | Additional resource attributes (e.g., `deployment.environment.name=production`) |

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
Create an initialization function that sets up the trace and metric providers.
Application logs are not part of this setup: they stay on stdout as structured JSON per [logs](../logs.md), correlated with traces through a context-aware handler (see [Structured logging](#structured-logging)) — do not initialize an OTLP log provider or wire a logging bridge for them.

```go
package main

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
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

	shutdown := func() {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
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

# Optional: also export metrics
export OTEL_METRICS_EXPORTER="otlp"
```

Leave `OTEL_LOGS_EXPORTER` unset: application logs stay on stdout per [logs](../logs.md).

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

<!-- eval:skip -->
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
| Logging | log/slog (context-aware JSON handler to stdout; see [Structured logging](#structured-logging)) |
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

## Database query parameters

Neither `go.opentelemetry.io/contrib/instrumentation/database/sql/otelsql` nor `github.com/XSAM/otelsql` capture prepared-statement parameter values by default.
Read [capturing database query parameters](../capture-database-query-parameters.md) first — it covers the cross-language risks and the Collector-side defence-in-depth that must be in place before enabling capture.

For `github.com/XSAM/otelsql`, use the `WithAttributesGetter` option to emit `db.query.parameter.<key>` attributes:

```go
import (
    "context"
    "database/sql/driver"
    "fmt"

    "github.com/XSAM/otelsql"
    "go.opentelemetry.io/otel/attribute"
)

func captureQueryParameters(_ context.Context, _ otelsql.Method, _ string, args []driver.NamedValue) []attribute.KeyValue {
    attrs := make([]attribute.KeyValue, 0, len(args))
    for i, a := range args {
        switch a.Value.(type) {
        case bool, int64, float64, string:
            // allowed types
        default:
            // skip binary, time.Time conversion is up to the caller, etc.
            continue
        }
        key := fmt.Sprintf("db.query.parameter.%d", i)
        if a.Name != "" {
            key = fmt.Sprintf("db.query.parameter.%s", a.Name)
        }
        attrs = append(attrs, attribute.String(key, fmt.Sprintf("%v", a.Value)))
    }
    return attrs
}

db, err := otelsql.Open("postgres", dsn,
    otelsql.WithAttributesGetter(captureQueryParameters),
)
```

For database drivers that bypass `database/sql` (e.g., `pgx` used directly), `trace.SpanFromContext(ctx).SetAttributes(...)` after the query returns is a no-op — the auto-instrumentation span is created and ended inside the driver tracer, so it is no longer recording by the time control returns to application code.
Use a library-level option that emits parameter values onto the driver's own span.
`github.com/exaring/otelpgx` exposes `otelpgx.WithIncludeQueryParameters()`:

```go
tracer := otelpgx.NewTracer(otelpgx.WithIncludeQueryParameters())
config.ConnConfig.Tracer = tracer
```

This emits the library-specific attribute shape, not `db.query.parameter.<key>` — verify the resulting attribute keys against your downstream consumers.
If the driver in use exposes no such option, parameter capture is not possible without modifying the driver's tracer implementation.

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

### Trace correlation with a context-aware slog handler

Per [logs](../logs.md), every record emitted inside an active span must carry `trace_id` and `span_id`, and stdout stays the only delivery channel for application logs — no OTLP log export, no logging bridge.
In Go, wrap the JSON handler in a `slog.Handler` that reads the span context from the record's context and stamps the ids:

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// traceContextHandler stamps every record emitted inside an active span with
// the span's trace_id and span_id, so stdout logs correlate with traces
// without exporting the logs over OTLP.
type traceContextHandler struct {
	slog.Handler
}

func (h traceContextHandler) Handle(ctx context.Context, record slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		record = record.Clone()
		record.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, record)
}

// WithAttrs and WithGroup must re-wrap, or the derived handler loses the
// trace-context stamping.
func (h traceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h traceContextHandler) WithGroup(name string) slog.Handler {
	return traceContextHandler{Handler: h.Handler.WithGroup(name)}
}

func main() {
	slog.SetDefault(slog.New(traceContextHandler{
		Handler: slog.NewJSONHandler(os.Stdout, nil),
	}))

	http.HandleFunc("GET /checkout", func(w http.ResponseWriter, r *http.Request) {
		// The request context carries the active span (for example started
		// by otelhttp); the Context logging variant hands it to the handler.
		slog.InfoContext(r.Context(), "checkout completed", "order_id", "TEST-0001")
		w.WriteHeader(http.StatusOK)
	})
}
```

Two rules make the correlation work:

1. **Log through the `Context` variants** (`slog.InfoContext`, `logger.ErrorContext`, and so on), passing the request's context.
   A bare `logger.Info` call has no context, so the handler cannot see the active span and the record carries no ids.
2. **Stamp the ids per record, inside the handler.**
   Reading the span context once at startup (or hardcoding ids) stamps every record with the same ids and correlates nothing.

<!-- eval:bad -->
```go
// BAD: no context — the handler cannot see the active span, so the record
// carries no trace_id/span_id even though a span is active in ctx.
logger.Info("checkout completed", "order_id", "TEST-0001")
```

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

<!-- eval:skip -->
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

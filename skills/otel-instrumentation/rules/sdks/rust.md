---
title: "Rust Instrumentation"
impact: HIGH
tags:
  - rust
  - backend
  - server
---

# Rust instrumentation

Instrument Rust applications to generate traces and metrics, and to correlate logs with traces.

## Use cases

- **HTTP request monitoring**: trace incoming and outgoing HTTP requests, and connect them across services.
- **Error detection**: mark failed operations with an `ERROR` status and a message that explains the failure.
- **Latency analysis**: measure where time goes inside a request with nested spans.

For Rust functions running on AWS Lambda, follow this file for the provider setup and [AWS Lambda deployment](../platforms/aws-lambda.md#rust) for the handler wrapping and the end-of-invocation flush.

## How Rust instrumentation works

Rust has no auto-instrumentation agent.
Instrument with the [`tracing`](https://docs.rs/tracing) crate, which most of the async ecosystem (Tokio, `tower`, `axum`, `hyper`, `reqwest`) already uses, and bridge its spans to OpenTelemetry with [`tracing-opentelemetry`](https://docs.rs/tracing-opentelemetry).
Use the OpenTelemetry API directly only where `tracing` has no equivalent: setting the span status, extracting and injecting context, and recording metrics.

The OpenTelemetry crates are pre-1.0 and every minor release breaks the API.
Pin `opentelemetry`, `opentelemetry_sdk`, `opentelemetry-otlp`, and `opentelemetry-http` to the same version, and pick the `tracing-opentelemetry` release built against it.

| `opentelemetry` crates | `tracing-opentelemetry` |
|------------------------|-------------------------|
| `0.33` | `0.34` |
| `0.32` | `0.33` |
| `0.31` | `0.32` |

Two copies of `opentelemetry` in the dependency graph compile, but spans from one copy never reach the provider of the other.
Run `cargo tree -d -i opentelemetry` and resolve every duplicate before shipping.

## Installation

Add the crates with exact versions.
Resolve current versions per [verify dependencies](../verify-dependencies.md); the versions below are an example of a matching set.

```toml
[dependencies]
opentelemetry = "=0.33.1"
opentelemetry_sdk = "=0.33.1"
# HTTP/protobuf with a blocking client: the batch processors export from
# their own thread, so the exporter needs no async runtime.
opentelemetry-otlp = { version = "=0.33.1", default-features = false, features = ["http-proto", "reqwest-blocking-client", "trace", "metrics"] }
opentelemetry-http = "=0.33.1"
tracing = "=0.1.44"
tracing-opentelemetry = "=0.34.0"
tracing-subscriber = { version = "=0.3.23", features = ["env-filter", "json", "registry"] }
```

Use `http-proto` with `reqwest-blocking-client`.
Do not enable `grpc-tonic` as well: with both transports compiled in, `SpanExporter::builder().build()` picks one from `OTEL_EXPORTER_OTLP_PROTOCOL`, and an unset variable silently selects HTTP.

## Environment variables

The Rust SDK reads a subset of the standard variables.
Exporters are selected in code, so `OTEL_TRACES_EXPORTER`, `OTEL_METRICS_EXPORTER`, `OTEL_LOGS_EXPORTER`, and `OTEL_SDK_DISABLED` have no effect.

| Variable | Read by | Default | Description |
|----------|---------|---------|-------------|
| `OTEL_SERVICE_NAME` | `Resource::builder()` | `unknown_service` | **Set it**; identifies the service |
| `OTEL_RESOURCE_ATTRIBUTES` | `Resource::builder()` | — | Additional resource attributes, for example `deployment.environment.name=production` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Exporters | `http://localhost:4318` with HTTP | Base URL; the HTTP exporter appends `/v1/traces` and `/v1/metrics` |
| `OTEL_EXPORTER_OTLP_HEADERS` | Exporters | — | For example `Authorization=Bearer <token>` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `SpanExporter::builder().build()` | The compiled-in transport | `http/protobuf` with the features above |
| `OTEL_TRACES_SAMPLER` | `SdkTracerProvider` | `parentbased_always_on` | Overridden by `.with_sampler(...)` in code |
| `OTEL_BSP_SCHEDULE_DELAY`, `OTEL_METRIC_EXPORT_INTERVAL` | Batch and periodic processors | 5000 ms, 60000 ms | Export intervals |

Get the endpoint from [Settings → Endpoints](https://app.dash0.com/goto/settings/endpoints?endpoint_type=otlp_http) and the token from [Settings → Auth Tokens](https://app.dash0.com/settings/auth-tokens).

## Setup

Initialize telemetry once, at the top of `main`, before any instrumented code runs.
The function below returns a handle whose `shutdown` flushes and stops the providers.

```rust
// src/telemetry.rs
use opentelemetry::{KeyValue, global, trace::TracerProvider as _};
use opentelemetry_otlp::{MetricExporter, SpanExporter};
use opentelemetry_sdk::{
    Resource,
    metrics::SdkMeterProvider,
    propagation::TraceContextPropagator,
    trace::{Sampler, SdkTracerProvider},
};
use tracing_subscriber::{EnvFilter, prelude::*};

/// Owns the providers so `main` can flush and shut them down before exiting.
pub struct Telemetry {
    tracer_provider: SdkTracerProvider,
    meter_provider: SdkMeterProvider,
}

pub fn init() -> Result<Telemetry, Box<dyn std::error::Error + Send + Sync>> {
    // Resource::builder() reads OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES.
    let resource = Resource::builder()
        .with_attribute(KeyValue::new("service.version", env!("CARGO_PKG_VERSION")))
        .build();

    // The exporters read OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_EXPORTER_OTLP_HEADERS,
    // and OTEL_EXPORTER_OTLP_PROTOCOL.
    let tracer_provider = SdkTracerProvider::builder()
        .with_sampler(Sampler::AlwaysOn)
        .with_batch_exporter(SpanExporter::builder().build()?)
        .with_resource(resource.clone())
        .build();
    let meter_provider = SdkMeterProvider::builder()
        .with_periodic_exporter(MetricExporter::builder().build()?)
        .with_resource(resource)
        .build();

    global::set_text_map_propagator(TraceContextPropagator::new());
    global::set_tracer_provider(tracer_provider.clone());
    global::set_meter_provider(meter_provider.clone());

    // Bridge `tracing` spans to OpenTelemetry, and write logs as one JSON
    // object per line on stdout.
    let tracer = tracer_provider.tracer(env!("CARGO_PKG_NAME"));
    tracing_subscriber::registry()
        .with(EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info")))
        .with(tracing_opentelemetry::layer().with_tracer(tracer))
        .with(tracing_subscriber::fmt::layer().json().with_span_list(true))
        .try_init()?;

    Ok(Telemetry { tracer_provider, meter_provider })
}

impl Telemetry {
    /// Flushes buffered telemetry. Call it before the process exits.
    pub fn shutdown(self) {
        if let Err(err) = self.tracer_provider.shutdown() {
            eprintln!("tracer provider shutdown failed: {err}");
        }
        if let Err(err) = self.meter_provider.shutdown() {
            eprintln!("meter provider shutdown failed: {err}");
        }
    }
}
```

Apply these rules to the setup:

1. **Set `Sampler::AlwaysOn` in code.**
   The default, `ParentBased(AlwaysOn)`, lets callers switch tracing off through the `traceparent` header, see [sampling](../spans.md#sampling).
2. **Use `with_batch_exporter`, never `with_simple_exporter`, with the blocking HTTP client.**
   The simple processor exports on the thread that ends the span, and the blocking client panics when it runs on a Tokio worker thread.
3. **Install the global propagator.**
   Without `set_text_map_propagator`, extraction and injection are no-ops and every service starts its own trace.
4. **Register exactly one subscriber.**
   A second `init()` or `set_global_default` fails, and code that ignores that error runs without the OpenTelemetry layer.
   Use `try_init()?` and remove any `tracing_subscriber::fmt::init()` or framework-provided default subscriber.
5. **Keep the `EnvFilter` at `info` or lower for the crates you instrument.**
   The filter applies to spans as well as to log events: a span filtered out by level is never created, and its children attach to the next enabled ancestor.

## Spans

Create spans with `#[tracing::instrument]` or `tracing::info_span!`.
`tracing-opentelemetry` maps these special fields onto the OpenTelemetry span:

| Field | Effect |
|-------|--------|
| `otel.name` | Span name; without it, the span is named after the function or the first macro argument |
| `otel.kind` | Span kind: `server`, `client`, `producer`, `consumer`, or `internal` (the default) |
| `otel.status_code` | `ok` or `error`, without a message |
| `otel.status_description` | Sets the status to `ERROR` with this message |

Every other field becomes a span attribute.
Name attribute fields after the [semantic conventions](https://opentelemetry.io/docs/specs/semconv/), with dots, for example `http.request.method`.

```rust
use tracing::instrument;

#[instrument(skip(db), fields(order.id = %order_id, otel.kind = "internal"))]
async fn load_order(db: &Database, order_id: &str) -> Result<Order, Error> {
    db.fetch(order_id).await
}
```

Use `skip(...)` for every argument that is large, not `Debug`, or sensitive.
`#[instrument]` records all arguments by default, which leaks request bodies and credentials into span attributes; see [sensitive data](../sensitive-data.md).

### Recording values after the span starts

`tracing` silently drops `Span::record` calls for fields the span did not declare.
Declare such fields up front with `field::Empty`, or set them through `OpenTelemetrySpanExt`, which accepts any key:

```rust
use opentelemetry::trace::Status;
use tracing::{Span, field, info_span};
use tracing_opentelemetry::OpenTelemetrySpanExt;

let span = info_span!("charge payment", payment.provider = "TEST-PSP", payment.outcome = field::Empty);
span.record("payment.outcome", "declined"); // declared above: recorded
span.record("payment.retry", true); // BAD: not declared, silently dropped
span.set_attribute("payment.retry", true); // any key: recorded
span.set_status(Status::error("payment declined by provider"));
```

### Span status

Set the status through `OpenTelemetrySpanExt::set_status`, and follow the [span status rules](../spans.md):

- Set `Status::error(message)` only when the operation failed, and always with a message that explains the failure.
- Set `Status::Ok` only for a confirmed success that a caller could otherwise misread; leave the status `UNSET` otherwise.
- Do not rely on `tracing::error!` events to set the status.
  The layer does turn an `error` event into an `ERROR` status, but the message becomes the event's `Debug` output, and handled errors that the code recovers from end up marked as failures.

```rust
use opentelemetry::trace::Status;
use tracing::Span;
use tracing_opentelemetry::OpenTelemetrySpanExt;

if let Err(err) = client.charge(&payment).await {
    Span::current().set_status(Status::error(format!("charge failed: {err}")));
    return Err(err.into());
}
```

## HTTP servers

There is no general-purpose OpenTelemetry middleware for Rust HTTP servers.
Build the SERVER span in `tower_http::trace::TraceLayer`, which works with `axum`, `tonic`, and any other `tower`-based server:

1. Name the span `{method} {route}` with the route template, never the raw path, so span names stay low-cardinality.
2. Set `otel.kind = "server"` and the HTTP semantic-convention attributes.
3. Extract the caller's context from the request headers and make it the parent.
4. Record the response status code, and set `ERROR` for 5xx responses only.

```toml
axum = "=0.8.9"
tower-http = { version = "=0.7.1", features = ["trace"] }
http = "1"
```

```rust
use axum::{Router, extract::MatchedPath, http::Request, routing::get};
use opentelemetry::{global, trace::{Status, TraceContextExt as _}};
use opentelemetry_http::HeaderExtractor;
use tower_http::trace::TraceLayer;
use tracing::{Span, field, info_span};
use tracing_opentelemetry::OpenTelemetrySpanExt;

let app = Router::new()
    .route("/orders/{id}", get(get_order))
    .layer(
        TraceLayer::new_for_http()
            .make_span_with(|req: &Request<_>| {
                let route = req
                    .extensions()
                    .get::<MatchedPath>()
                    .map(MatchedPath::as_str)
                    .unwrap_or("");
                let span = info_span!(
                    "http request",
                    otel.name = format!("{} {}", req.method(), route),
                    otel.kind = "server",
                    http.request.method = %req.method(),
                    http.route = route,
                    url.path = req.uri().path(),
                    http.response.status_code = field::Empty,
                    trace_id = field::Empty,
                );
                let parent = global::get_text_map_propagator(|p| p.extract(&HeaderExtractor(req.headers())));
                let _ = span.set_parent(parent);
                // Correlates every log line of the request with its trace.
                span.record("trace_id", span.context().span().span_context().trace_id().to_string());
                span
            })
            .on_response(|res: &axum::http::Response<_>, _latency, span: &Span| {
                span.record("http.response.status_code", res.status().as_u16());
                if res.status().is_server_error() {
                    span.set_status(Status::error(format!("HTTP {}", res.status().as_u16())));
                }
            }),
    );
```

Add the layer with `Router::layer`, after the routes.
`MatchedPath` is only present on a route-level layer; added in front of the router, the span name loses the route.

## HTTP clients

Wrap `reqwest` with [`reqwest-middleware`](https://docs.rs/reqwest-middleware) and [`reqwest-tracing`](https://docs.rs/reqwest-tracing), which create a CLIENT span per request.
Check that the `reqwest-tracing` release has an `opentelemetry_0_XX` feature matching your `opentelemetry` version.
When it has none, it still creates the CLIENT span but does not inject `traceparent`, so add the injecting middleware below after `TracingMiddleware`, where the current span is the CLIENT span:

```toml
reqwest = { version = "=0.13.5", default-features = false, features = ["rustls"] }
reqwest-middleware = "=0.5.2"
reqwest-tracing = "=0.7.1"
async-trait = "0.1"
```

```rust
use opentelemetry::global;
use opentelemetry_http::HeaderInjector;
use reqwest_middleware::{ClientBuilder, Middleware, Next};
use reqwest_tracing::TracingMiddleware;
use tracing::Span;
use tracing_opentelemetry::OpenTelemetrySpanExt;

/// Injects the current span's context into outgoing requests.
pub struct InjectTraceContext;

#[async_trait::async_trait]
impl Middleware for InjectTraceContext {
    async fn handle(
        &self,
        mut req: reqwest::Request,
        extensions: &mut http::Extensions,
        next: Next<'_>,
    ) -> reqwest_middleware::Result<reqwest::Response> {
        let cx = Span::current().context();
        global::get_text_map_propagator(|p| p.inject_context(&cx, &mut HeaderInjector(req.headers_mut())));
        next.run(req, extensions).await
    }
}

let client = ClientBuilder::new(reqwest::Client::new())
    .with(TracingMiddleware::default())
    .with(InjectTraceContext)
    .build();
```

Build the client once and share it.
A plain `reqwest::Client` bypasses the middleware and produces no CLIENT span.

## Context propagation

`tracing` carries the current span in task-local state, so context follows `.await` within a task but not across `tokio::spawn`.
Instrument spawned futures with the span they belong to:

```rust
use tracing::{Instrument, Span};

// BAD: the spawned task has no parent span and starts a new trace.
tokio::spawn(async move { send_receipt(order).await });

// GOOD: the task's spans are children of the current span.
tokio::spawn(async move { send_receipt(order).await }.instrument(Span::current()));
```

Do the same for `std::thread::spawn` and `spawn_blocking`: capture `Span::current()` before spawning and `let _guard = span.enter();` inside the closure.
Never hold an `enter()` guard across an `.await`; use `.instrument(span)` for futures.

## Metrics

Create instruments once, from the global meter, and reuse them:

```rust
use opentelemetry::{KeyValue, global};

let meter = global::meter("orders");
let orders_placed = meter
    .u64_counter("orders.placed")
    .with_description("Orders accepted for fulfillment")
    .build();

orders_placed.add(1, &[KeyValue::new("payment.method", "card")]);
```

Follow [metrics](../metrics.md) for instrument selection, naming, and attribute cardinality.

## Logs

Write logs to stdout as one JSON object per line, per [logs](../logs.md).
The setup above does this with `fmt::layer().json().with_span_list(true)`, which prints the fields of every enclosing span, including the `trace_id` that the [HTTP server span](#http-servers) records.
Use `tracing` events with structured fields, never interpolated strings:

```rust
// BAD: unstructured
tracing::info!("order {} placed by {}", order_id, user_id);

// GOOD: structured
tracing::info!(order.id = %order_id, user.id = %user_id, "order placed");
```

Do not also export logs over OTLP when stdout is collected, or every log record arrives twice; see [stdout vs OTLP](../logs.md#stdout-vs-otlp-for-log-delivery).

## Graceful shutdown

The batch processors hold up to 5 seconds of spans in memory.
Call `shutdown()` on every exit path, after the server stops accepting requests:

```rust
mod telemetry;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let telemetry = telemetry::init()?;

    let listener = tokio::net::TcpListener::bind("0.0.0.0:8080").await?;
    axum::serve(listener, app())
        .with_graceful_shutdown(async {
            let _ = tokio::signal::ctrl_c().await;
        })
        .await?;

    telemetry.shutdown();
    Ok(())
}
```

`std::process::exit` and a panic with `panic = "abort"` skip `shutdown()`, and the buffered spans are lost.
Return from `main` instead of calling `exit`.

## Troubleshooting

### No telemetry appearing

1. Check that `telemetry::init()` runs before the first span is created, and that its `Result` is not discarded.
2. Check `cargo tree -d -i opentelemetry` for duplicate versions; a duplicate makes the bridge send spans to a provider that is never installed.
3. Check that `OTEL_EXPORTER_OTLP_ENDPOINT` is the base URL, without `/v1/traces`; the HTTP exporter appends the signal path itself.
4. Check that the process calls `shutdown()` before exiting.

### Spans exist but are disconnected

The child spans run on a spawned task or thread without the parent span; see [context propagation](#context-propagation).
For traces broken between services, check that the global propagator is installed and that the client injects `traceparent`.

### Debug the exporter

Set `RUST_LOG=info,opentelemetry=debug` to print the SDK's internal diagnostics, including export errors, through the `tracing` subscriber.
The directive matches every crate whose name starts with `opentelemetry`.
Remove it once the problem is found: at `debug`, the SDK logs every export.

## Resources

- [OpenTelemetry Rust](https://opentelemetry.io/docs/languages/rust/)
- [`tracing-opentelemetry`](https://docs.rs/tracing-opentelemetry)
- [Environment variable specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)

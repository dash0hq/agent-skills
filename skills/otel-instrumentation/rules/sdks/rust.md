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
Run `cargo tree -i opentelemetry` before shipping: it prints one dependency tree when there is a single copy, and fails with ``specification `opentelemetry` is ambiguous``, listing every version, when there are two.

## Installation

Add the crates with exact versions.
Resolve current versions per [verify dependencies](../verify-dependencies.md); the versions below are an example of a matching set.

```toml
[dependencies]
opentelemetry = "=0.33.1"
opentelemetry_sdk = "=0.33.1"
# HTTP/protobuf with a blocking client: the batch processors export from
# their own thread, so the exporter needs no async runtime.
opentelemetry-otlp = { version = "=0.33.1", default-features = false, features = ["http-proto", "reqwest-blocking-client", "reqwest-rustls", "trace", "metrics"] }
opentelemetry-http = "=0.33.1"
opentelemetry-resource-detectors = "=0.13.1"
tracing = "=0.1.44"
tracing-opentelemetry = "=0.34.0"
tracing-subscriber = { version = "=0.3.23", features = ["env-filter", "registry"] }
tokio = { version = "=1.54.1", features = ["macros", "net", "rt-multi-thread", "signal"] }
# The JSON log layer in the setup below.
serde_json = "=1.0.145"
```

Apply these rules to the exporter features:

1. **Use `http-proto` with `reqwest-blocking-client`.**
   Do not enable `grpc-tonic` as well: with both transports compiled in, `SpanExporter::builder().build()` picks one from `OTEL_EXPORTER_OTLP_PROTOCOL`, and an unset variable silently selects HTTP.
2. **Enable `reqwest-rustls` when the endpoint is `https://`.**
   `opentelemetry-otlp` builds `reqwest` without TLS, so without this feature the exporter cannot reach an HTTPS endpoint such as Dash0's.
   It only appears to work when another dependency of the application happens to enable `reqwest`'s `rustls` feature.
   A Collector on plain `http://` loopback, such as a Lambda exporter layer, does not need it.
3. **Install `ca-certificates` in the runtime image.**
   `reqwest`'s `rustls` feature verifies certificates against the system trust store, which `debian:*-slim` images do not ship; see [using a Dockerfile](#using-a-dockerfile).

### Verifying dependencies

Never hand-write a `[dependencies]` entry in `Cargo.toml` with a version from memory; verify the crate against crates.io first, per [verify-dependencies](../verify-dependencies.md):

```bash
cargo info opentelemetry-otlp
cargo info opentelemetry-otlp@0.33.1
```

The first command prints the latest published version, its `rust-version`, and its features; the second fails when that version was never published.
Check the reported `rust-version` against the project's `rust-version` in `Cargo.toml`, its `rust-toolchain.toml`, and the `rust:` base image in its Dockerfile.

Add crates with `cargo add <crate>@=<version>`, which writes the exact pin and updates `Cargo.lock` in one step.
After editing `Cargo.toml` by hand, run `cargo update --workspace` to bring `Cargo.lock` in step without upgrading unrelated crates, then run `cargo tree -i opentelemetry` to confirm that no second copy entered the graph.

For an instrumentation crate, also check which `opentelemetry` version it depends on before adding it:

```bash
cargo add opentelemetry-instrumentation-tower@=0.19.0 --features axum
cargo tree -i opentelemetry
```

When `cargo tree` reports the specification as ambiguous, the crate targets a different release than the project; pick the crate release that matches, or leave the crate out.

## Environment variables

The Rust SDK reads a subset of the standard variables.
Exporters are selected in code, so `OTEL_TRACES_EXPORTER`, `OTEL_METRICS_EXPORTER`, `OTEL_LOGS_EXPORTER`, and `OTEL_SDK_DISABLED` have no effect.

| Variable | Read by | Default | Description |
|----------|---------|---------|-------------|
| `OTEL_SERVICE_NAME` | `SdkProvidedResourceDetector` | `unknown_service:<executable name>` | **Set it**; identifies the service |
| `OTEL_RESOURCE_ATTRIBUTES` | `EnvResourceDetector` | — | Additional resource attributes, for example `deployment.environment.name=production` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Exporters | `http://localhost:4318` with HTTP | Base URL; the HTTP exporter appends `/v1/traces` and `/v1/metrics` |
| `OTEL_EXPORTER_OTLP_HEADERS` | Exporters | — | For example `Authorization=Bearer <token>` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `SpanExporter::builder().build()` | The compiled-in transport | `http/protobuf` with the features above |
| `OTEL_TRACES_SAMPLER` | `SdkTracerProvider` | `parentbased_always_on` | Overridden by `.with_sampler(...)` in code |
| `OTEL_BSP_SCHEDULE_DELAY`, `OTEL_METRIC_EXPORT_INTERVAL` | Batch and periodic processors | 5000 ms, 60000 ms | Export intervals |

### Where to get configuration values

1. **OTLP endpoint**: in Dash0, [Settings → Endpoints](https://app.dash0.com/goto/settings/endpoints?endpoint_type=otlp_http), the OTLP/HTTP endpoint.
2. **Auth token**: in Dash0, [Settings → Auth Tokens](https://app.dash0.com/settings/auth-tokens).
3. **Service name, version, and environment**: resolve them from the project per [resolving values](../resolve-values.md).

## Configuration

### 1. Activate the SDK

Initialize telemetry once, at the top of `main`, before any instrumented code runs.
The function below returns a handle whose `shutdown` flushes and stops the providers.
Application logs are not part of the OpenTelemetry export: they stay on stdout as JSON lines, correlated through the `JsonStdoutLogs` layer in [structured logging](#structured-logging).

```rust
// src/telemetry.rs
use opentelemetry::{KeyValue, global, trace::TracerProvider as _};
use opentelemetry_otlp::{MetricExporter, SpanExporter};
use opentelemetry_resource_detectors::{
    ContainerResourceDetector, HostResourceDetector, OsResourceDetector, ServiceInstanceIdResourceDetector,
};
use opentelemetry_sdk::{
    Resource,
    metrics::SdkMeterProvider,
    propagation::TraceContextPropagator,
    resource::{EnvResourceDetector, SdkProvidedResourceDetector, TelemetryResourceDetector},
    trace::{Sampler, SdkTracerProvider},
};
use tracing_subscriber::{EnvFilter, prelude::*};

use crate::logs::JsonStdoutLogs;

/// Owns the providers so `main` can flush and shut them down before exiting.
pub struct Telemetry {
    tracer_provider: SdkTracerProvider,
    meter_provider: SdkMeterProvider,
}

pub fn init() -> Result<Telemetry, Box<dyn std::error::Error + Send + Sync>> {
    let resource = Resource::builder_empty()
        .with_detectors(&[
            Box::new(HostResourceDetector::default()),
            Box::new(OsResourceDetector),
            Box::new(ContainerResourceDetector),
            Box::new(ServiceInstanceIdResourceDetector),
        ])
        .with_attribute(KeyValue::new("service.version", env!("CARGO_PKG_VERSION")))
        // Last, so OTEL_RESOURCE_ATTRIBUTES and OTEL_SERVICE_NAME override
        // every value above.
        .with_detectors(&[
            Box::new(TelemetryResourceDetector),
            Box::new(EnvResourceDetector::new()),
            Box::new(SdkProvidedResourceDetector),
        ])
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

    let tracer = tracer_provider.tracer(env!("CARGO_PKG_NAME"));
    tracing_subscriber::registry()
        .with(EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info")))
        // Bridge `tracing` spans to OpenTelemetry, without the bridge's own
        // attributes (busy_ns, idle_ns, thread.*, code.*, target), which are
        // not semantic conventions.
        .with(
            tracing_opentelemetry::layer()
                .with_tracer(tracer)
                .with_tracked_inactivity(false)
                .with_threads(false)
                .with_location(false)
                .with_target(false),
        )
        // Logs: one JSON object per line on stdout, see Structured logging.
        .with(JsonStdoutLogs)
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
6. **Add the environment detectors last.**
   `with_detectors` and `with_attribute` override the attributes added before them, so detectors added after `Resource::builder()` silently replace the values from `OTEL_RESOURCE_ATTRIBUTES`, such as the pod UID that [Kubernetes deployment](../platforms/k8s.md) sets as `service.instance.id`.
   Build from `Resource::builder_empty()` in the order above instead.
7. **Do not add `ProcessResourceDetector`.**
   It records `process.command_args`, which carries every command-line argument, credentials included; see [sensitive data](../sensitive-data.md).

### 2. Set service name

```bash
export OTEL_SERVICE_NAME="order-api"
```

### 3. Select exporters

The Rust SDK ignores `OTEL_TRACES_EXPORTER` and `OTEL_METRICS_EXPORTER`.
Traces and metrics are exported because the setup above installs the OTLP exporters; to switch exporters, change the code, see [console exporter](#console-exporter).
Do not install a log exporter: application logs stay on stdout per [logs](../logs.md).

### 4. Configure endpoint

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"
```

Set the base URL only; the HTTP exporter appends `/v1/traces` and `/v1/metrics`.

### 5. Optional: target specific dataset

```bash
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN,Dash0-Dataset=my-dataset"
```

## Complete setup

### Using environment variables

```bash
export OTEL_SERVICE_NAME="order-api"
export OTEL_RESOURCE_ATTRIBUTES="deployment.environment.name=production"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"

cargo run --release
```

### Using a Dockerfile

Bake only non-secret configuration into the image, and pass the endpoint and the token when the container starts:

```dockerfile
FROM rust:1.99-slim-trixie AS build
WORKDIR /src
COPY . .
RUN cargo build --release --locked

FROM debian:trixie-slim
# The rustls certificate verifier reads the system trust store.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /src/target/release/order-api /usr/local/bin/order-api
ENV OTEL_SERVICE_NAME=order-api
ENTRYPOINT ["/usr/local/bin/order-api"]
```

```bash
docker run \
  -e OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>" \
  -e OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN" \
  order-api
```

`cargo build --locked` fails when `Cargo.lock` is out of step with `Cargo.toml`; regenerate the lockfile per [verifying dependencies](#verifying-dependencies) instead of dropping the flag.

## Local development

### Console exporter

To see spans and metrics without a Collector or backend, swap the OTLP exporters for the stdout exporters of [`opentelemetry-stdout`](https://docs.rs/opentelemetry-stdout), at the same version as the other `opentelemetry` crates:

```toml
opentelemetry-stdout = { version = "=0.33.1", features = ["trace", "metrics"] }
```

```rust
// In telemetry::init(), in place of the OTLP exporters.
let tracer_provider = SdkTracerProvider::builder()
    .with_sampler(Sampler::AlwaysOn)
    .with_batch_exporter(opentelemetry_stdout::SpanExporter::default())
    .with_resource(resource.clone())
    .build();
let meter_provider = SdkMeterProvider::builder()
    .with_periodic_exporter(opentelemetry_stdout::MetricExporter::default())
    .with_resource(resource)
    .build();
```

The stdout exporters print to the same stream as the JSON log lines, so use them only on a developer machine, never in a deployed service whose stdout is collected.

### Without a collector

With no Collector listening on `OTEL_EXPORTER_OTLP_ENDPOINT`, the application keeps running and every export fails.
The SDK reports the failures through `tracing` at the `opentelemetry` target; see [debug the exporter](#debug-the-exporter) to print them.

**Options:**
1. Use the console exporter during development.
2. Run a local OpenTelemetry Collector.
3. Point `OTEL_EXPORTER_OTLP_ENDPOINT` directly at the observability backend.

## Resource configuration

Set `service.name`, `service.version`, and `deployment.environment.name` for every deployment.
The setup above takes `service.version` from `Cargo.toml` at compile time and lets `OTEL_RESOURCE_ATTRIBUTES` override it.
See [resource attributes](../resources.md) for the full list of required and recommended attributes.

## Kubernetes setup

See [Kubernetes deployment](../platforms/k8s.md) for pod metadata injection, resource attributes, and Dash0 Kubernetes Operator guidance.
The pod spec sets `service.instance.id` and the `k8s.*` attributes through `OTEL_RESOURCE_ATTRIBUTES`; keep the environment detectors last, per rule 6 of the [setup](#1-activate-the-sdk), so those values win.

## Supported libraries

Rust instrumentation comes from individual crates, not from an agent.
Use the crate in this table for each library the application uses, and check its `opentelemetry` dependency per [verifying dependencies](#verifying-dependencies) before adding it:

| Library | Crate | Produces | Guidance |
|---------|-------|----------|----------|
| `axum`, `tonic`, any `tower` HTTP server | `tower-http` `TraceLayer` | SERVER span through `tracing` | [HTTP servers](#http-servers) |
| `axum`, `tonic`, any `tower` HTTP server | `opentelemetry-instrumentation-tower` | `http.server.request.duration`, `http.server.active_requests`, request and response body sizes | [HTTP servers](#http-servers) |
| `actix-web` | `opentelemetry-instrumentation-actix-web` | SERVER span through the OpenTelemetry API, and HTTP server metrics | [actix-web](#actix-web) |
| `reqwest` | `reqwest-middleware` with `reqwest-tracing` | CLIENT span through `tracing` | [HTTP clients](#http-clients) |
| `tonic` (gRPC server and client) | `tonic-tracing-opentelemetry` | SERVER and CLIENT spans through `tracing` | [gRPC with tonic](#grpc-with-tonic) |
| `sqlx`, `diesel`, `tokio-postgres` | none | — | [Database spans](#database-spans) |
| `rdkafka`, `lapin`, other message clients | none | — | [Messaging](#messaging) |

Do not add `actix-web-opentelemetry`: its last release, 0.22, depends on `opentelemetry` 0.29.

## HTTP servers

Build the SERVER span in `tower_http::trace::TraceLayer`, which works with `axum`, `tonic`, and any other `tower`-based server, and record the HTTP server metrics with `opentelemetry-instrumentation-tower`:

1. Name the span `{method} {route}` with the route template, never the raw path, so span names stay low-cardinality.
2. Set `otel.kind = "server"` and the HTTP semantic-convention attributes.
3. Extract the caller's context from the request headers and make it the parent.
4. Record the response status code, and set `ERROR` for 5xx responses only.
5. Add `opentelemetry-instrumentation-tower` with `with_tracing(false)`, so it records metrics but no second SERVER span.

```toml
axum = "=0.8.9"
tower-http = { version = "=0.7.1", features = ["trace"] }
opentelemetry-instrumentation-tower = { version = "=0.19.0", features = ["axum"] }
http = "1"
```

```rust
// src/routes.rs
use axum::{
    Router,
    extract::{MatchedPath, Path},
    http::Request,
    routing::get,
};
use opentelemetry::{global, trace::Status};
use opentelemetry_http::HeaderExtractor;
use opentelemetry_instrumentation_tower::http::server::LayerBuilder;
use tower_http::trace::TraceLayer;
use tracing::{Span, field, info_span};
use tracing_opentelemetry::OpenTelemetrySpanExt;

/// Builds the router. Call it after `telemetry::init()`: the metrics layer
/// takes its meter from the global meter provider when it is built.
pub fn app() -> Router {
    let http_metrics = LayerBuilder::builder()
        .with_tracing(false)
        .build()
        .expect("HTTP server metrics layer");

    Router::new()
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
                    );
                    let parent = global::get_text_map_propagator(|p| p.extract(&HeaderExtractor(req.headers())));
                    let _ = span.set_parent(parent);
                    span
                })
                .on_response(|res: &axum::http::Response<_>, _latency, span: &Span| {
                    span.record("http.response.status_code", res.status().as_u16());
                    if res.status().is_server_error() {
                        span.set_status(Status::error(format!("HTTP {}", res.status().as_u16())));
                    }
                }),
        )
        .layer(http_metrics)
}

async fn get_order(Path(id): Path<String>) -> String {
    id
}
```

Add both layers with `Router::layer`, after the routes.
`MatchedPath` is only present on a route-level layer; added in front of the router, the span name and the `http.route` metric attribute lose the route.
Without the `axum` feature, `opentelemetry-instrumentation-tower` records no `http.route` at all.

Do not use `opentelemetry-instrumentation-tower` for the SERVER span.
It creates the span through the OpenTelemetry API, not `tracing`, so `Span::current()` in a handler is not that span and the [attribute pattern below](#adding-attributes-to-the-server-span) silently does nothing.

### Adding attributes to the SERVER span

Inside a handler without its own `#[instrument]`, `Span::current()` is the SERVER span of the request.
Add request-level attributes to it with `set_attribute`; do not create a child span only to carry them:

```rust
use axum::response::IntoResponse;
use tracing::Span;
use tracing_opentelemetry::OpenTelemetrySpanExt;

async fn checkout(/* extractors */) -> impl IntoResponse {
    Span::current().set_attribute("order.id", "TEST-0001");
    // ...
}
```

`#[instrument]` on the handler makes its own span the current one.
Then add the attribute through the `#[instrument]` fields instead, or drop `#[instrument]` from the handler.

### actix-web

For `actix-web`, wrap the app in the middleware of `opentelemetry-instrumentation-actix-web`, which creates the SERVER span and records the HTTP server metrics:

```toml
actix-web = "4"
opentelemetry-instrumentation-actix-web = { version = "=0.25.0", features = ["metrics"] }
```

```rust
use actix_web::{App, HttpServer, web};
use opentelemetry::{KeyValue, trace::get_active_span};
use opentelemetry_instrumentation_actix_web::{RequestMetrics, RequestTracing};

async fn checkout() -> &'static str {
    // The SERVER span is an OpenTelemetry API span, not a `tracing` span.
    get_active_span(|span| span.set_attribute(KeyValue::new("order.id", "TEST-0001")));
    "ok"
}

async fn serve() -> std::io::Result<()> {
    HttpServer::new(|| {
        App::new()
            .wrap(RequestTracing::new())
            .wrap(RequestMetrics::default())
            .route("/checkout", web::get().to(checkout))
    })
    .bind(("0.0.0.0", 8080))?
    .run()
    .await
}
```

The middleware attaches the SERVER span to the OpenTelemetry context while the handler runs, so `tracing` spans created in the handler become its children.
Add attributes to the SERVER span with `get_active_span`, not `Span::current()`, which here is not the SERVER span.

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

### Recording client request duration

No `reqwest` middleware records `http.client.request.duration`.
When a dashboard or alert needs outbound latency, record it in one more middleware, with the semantic-convention unit, buckets, and attributes:

```rust
use std::time::Instant;

use opentelemetry::{KeyValue, global, metrics::Histogram};
use reqwest_middleware::{Middleware, Next};

/// Records http.client.request.duration for every request.
pub struct ClientRequestDuration(Histogram<f64>);

impl ClientRequestDuration {
    pub fn new() -> Self {
        Self(
            global::meter("http-client")
                .f64_histogram("http.client.request.duration")
                .with_unit("s")
                .with_boundaries(vec![0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1.0, 2.5, 5.0, 7.5, 10.0])
                .build(),
        )
    }
}

#[async_trait::async_trait]
impl Middleware for ClientRequestDuration {
    async fn handle(
        &self,
        req: reqwest::Request,
        extensions: &mut http::Extensions,
        next: Next<'_>,
    ) -> reqwest_middleware::Result<reqwest::Response> {
        let mut attributes = vec![
            KeyValue::new("http.request.method", req.method().as_str().to_owned()),
            KeyValue::new("server.address", req.url().host_str().unwrap_or_default().to_owned()),
        ];
        if let Some(port) = req.url().port_or_known_default() {
            attributes.push(KeyValue::new("server.port", i64::from(port)));
        }
        let start = Instant::now();
        let result = next.run(req, extensions).await;
        match &result {
            Ok(res) => {
                let status = res.status().as_u16();
                attributes.push(KeyValue::new("http.response.status_code", i64::from(status)));
                if status >= 400 {
                    attributes.push(KeyValue::new("error.type", status.to_string()));
                }
            }
            Err(_) => attributes.push(KeyValue::new("error.type", "_OTHER")),
        }
        self.0.record(start.elapsed().as_secs_f64(), &attributes);
        result
    }
}
```

Add it with `.with(ClientRequestDuration::new())` after `InjectTraceContext`, and build the client after `telemetry::init()`, or the histogram records into a no-op meter.
Never add `url.full` or the path as an attribute: it makes the metric's cardinality unbounded, see [metrics](../metrics.md).

## gRPC with tonic

Add `OtelGrpcLayer` from [`tonic-tracing-opentelemetry`](https://docs.rs/tonic-tracing-opentelemetry) to the server and to every client channel.
The server layer extracts the caller's context and creates the SERVER span; the client layer creates the CLIENT span and injects `traceparent`.

```toml
tonic = "=0.14.9"
tonic-tracing-opentelemetry = { version = "=0.42.1", features = ["tracing_level_info"] }
tower = "0.5"
```

```rust
use tonic::transport::{Channel, Server};
use tonic_tracing_opentelemetry::middleware::{client, filters, server};
use tower::ServiceBuilder;

// Server: one SERVER span per call, health checks excluded.
Server::builder()
    .layer(server::OtelGrpcLayer::default().filter(filters::reject_healthcheck))
    .add_service(OrdersServer::new(orders))
    .serve(addr)
    .await?;

// Client: one CLIENT span per call, with traceparent injected.
let channel = Channel::from_static("http://inventory.example.test:50051").connect().await?;
let channel = ServiceBuilder::new().layer(client::OtelGrpcLayer).service(channel);
let mut inventory = InventoryClient::new(channel);
```

Enable the `tracing_level_info` feature.
Without it, the crate creates its spans at `TRACE` level on the `otel::tracing` target, and the `info` filter of the [setup](#1-activate-the-sdk) drops every gRPC span.
When the feature cannot be enabled, add the directive `otel::tracing=trace` to the `EnvFilter` instead.

The crate records the older RPC attributes `rpc.system`, `rpc.service`, `rpc.method`, and `rpc.grpc.status_code`; query them under those names.

## Database spans

`sqlx`, `diesel`, and `tokio-postgres` emit no OpenTelemetry database CLIENT spans; `sqlx` only logs each query as a `tracing` event on the `sqlx::query` target.
Wrap each query in a CLIENT span with the database semantic-convention attributes, as in the example under [database query parameters](#database-query-parameters):

1. Name the span `{db.operation.name} {db.collection.name}`, for example `SELECT orders`.
2. Set `otel.kind = "client"`, `db.system.name`, `db.operation.name`, `db.collection.name`, and `db.query.text` with placeholders, never with literal values.
3. Attach the span to the query future with `.instrument(span)`, so it ends when the query completes.

## Database query parameters

No Rust database crate captures prepared-statement parameter values for you.
Read [capturing database query parameters](../capture-database-query-parameters.md) first: it covers the cross-language risks and the Collector-side redaction that must be in place before enabling capture.

Record each parameter on the CLIENT span as `db.query.parameter.<index>`, with the 0-based index of its bind position (`$1` is index `0`).
Gate the capture behind a setting that is off by default, and record only parameters that are not on the [never-instrument list](../sensitive-data.md#never-instrument-list):

```rust
use tracing::{Instrument, info_span};
use tracing_opentelemetry::OpenTelemetrySpanExt;

const FIND_ORDER: &str = "SELECT id, status FROM orders WHERE id = $1";

async fn find_order(pool: &sqlx::PgPool, order_id: &str, capture_parameters: bool) -> Result<Order, sqlx::Error> {
    let span = info_span!(
        "SELECT orders",
        otel.kind = "client",
        db.system.name = "postgresql",
        db.operation.name = "SELECT",
        db.collection.name = "orders",
        db.query.text = FIND_ORDER,
    );
    if capture_parameters {
        span.set_attribute("db.query.parameter.0", order_id.to_owned());
    }
    sqlx::query_as::<_, Order>(FIND_ORDER)
        .bind(order_id)
        .fetch_one(pool)
        .instrument(span)
        .await
}
```

Read `capture_parameters` once at startup, for example from an `APP_CAPTURE_DB_QUERY_PARAMETERS=true` environment variable, and never enable it by default in production.

## Messaging

No Rust message-broker client injects or extracts trace context.
Carry it in the message headers yourself: inject on the PRODUCER span when sending, and extract and set the parent of the CONSUMER span when processing.

```rust
use std::collections::HashMap;

use opentelemetry::global;
use tracing::{Instrument, info_span};
use tracing_opentelemetry::OpenTelemetrySpanExt;

async fn publish(payload: &[u8]) {
    let span = info_span!(
        "send orders",
        otel.kind = "producer",
        messaging.system = "kafka",
        messaging.destination.name = "orders",
        messaging.operation.type = "send",
    );
    let mut headers: HashMap<String, String> = HashMap::new();
    global::get_text_map_propagator(|p| p.inject_context(&span.context(), &mut headers));
    // Copy `headers` into the message headers of the client, then send.
    send_with_headers(payload, &headers).instrument(span).await;
}

async fn process(payload: &[u8], headers: &HashMap<String, String>) {
    let parent = global::get_text_map_propagator(|p| p.extract(headers));
    let span = info_span!(
        "process orders",
        otel.kind = "consumer",
        messaging.system = "kafka",
        messaging.destination.name = "orders",
        messaging.operation.type = "process",
    );
    let _ = span.set_parent(parent);
    handle_order(payload).instrument(span).await;
}
```

Convert between the client's header type and `HashMap<String, String>` at the call site; the propagator reads and writes the `traceparent` and `tracestate` keys.

## Custom spans

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

### Retrieving the active span

Pick the accessor by the API that created the span:

| Span created by | Accessor |
|-----------------|----------|
| `tracing`: `#[instrument]`, `info_span!`, `TraceLayer`, `reqwest-tracing`, `tonic-tracing-opentelemetry` | `tracing::Span::current()` with `OpenTelemetrySpanExt` |
| The OpenTelemetry API: `opentelemetry-instrumentation-actix-web` | `opentelemetry::trace::get_active_span` |

```rust
use opentelemetry::{KeyValue, trace::get_active_span};
use tracing::Span;
use tracing_opentelemetry::OpenTelemetrySpanExt;

// A `tracing` span is current.
Span::current().set_attribute("order.id", "TEST-0001");

// An OpenTelemetry API span is active.
get_active_span(|span| span.set_attribute(KeyValue::new("order.id", "TEST-0001")));
```

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

### Span status rules

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

Do not create the HTTP metrics by hand on the server side: `opentelemetry-instrumentation-tower` and `opentelemetry-instrumentation-actix-web` record them, see [HTTP servers](#http-servers).
On the client side, record `http.client.request.duration` per [recording client request duration](#recording-client-request-duration).
Follow [metrics](../metrics.md) for instrument selection, naming, and attribute cardinality.

## Structured logging

Write logs to stdout as one JSON object per line, per [logs](../logs.md), and put the `trace_id` and `span_id` of the active span on every line as top-level fields.
`tracing-subscriber`'s built-in JSON formatter cannot do this: it knows nothing about OpenTelemetry IDs, and with `with_span_list` it nests span fields in an array.
Use this layer, registered in the [setup](#1-activate-the-sdk):

```rust
// src/logs.rs
use std::io::Write as _;

use opentelemetry::trace::TraceContextExt as _;
use serde_json::{Map, Value};
use tracing::{Event, Subscriber, field::{Field, Visit}};
use tracing_opentelemetry::OpenTelemetrySpanExt as _;
use tracing_subscriber::{Layer, layer::Context};

/// Writes every event as one JSON object per line on stdout, with the
/// `trace_id` and `span_id` of the active span as top-level fields.
pub struct JsonStdoutLogs;

impl<S: Subscriber> Layer<S> for JsonStdoutLogs {
    fn on_event(&self, event: &Event<'_>, _ctx: Context<'_, S>) {
        let mut record = Map::new();
        record.insert("level".into(), event.metadata().level().as_str().into());
        record.insert("target".into(), event.metadata().target().into());
        event.record(&mut JsonFields(&mut record));

        let cx = tracing::Span::current().context();
        let span_context = cx.span().span_context().clone();
        if span_context.is_valid() {
            record.insert("trace_id".into(), span_context.trace_id().to_string().into());
            record.insert("span_id".into(), span_context.span_id().to_string().into());
        }

        let mut line = Value::Object(record).to_string();
        line.push('\n');
        let _ = std::io::stdout().lock().write_all(line.as_bytes());
    }
}

struct JsonFields<'a>(&'a mut Map<String, Value>);

impl Visit for JsonFields<'_> {
    fn record_str(&mut self, field: &Field, value: &str) {
        self.0.insert(field.name().into(), value.into());
    }
    fn record_i64(&mut self, field: &Field, value: i64) {
        self.0.insert(field.name().into(), value.into());
    }
    fn record_u64(&mut self, field: &Field, value: u64) {
        self.0.insert(field.name().into(), value.into());
    }
    fn record_bool(&mut self, field: &Field, value: bool) {
        self.0.insert(field.name().into(), value.into());
    }
    fn record_debug(&mut self, field: &Field, value: &dyn std::fmt::Debug) {
        self.0.insert(field.name().into(), format!("{value:?}").into());
    }
}
```

The event message lands in the `message` field.
Register the layer **after** the OpenTelemetry layer, and remove any other `fmt` layer that writes to stdout, or every event is printed twice.

Use `tracing` events with structured fields, never interpolated strings:

```rust
// BAD: unstructured
tracing::info!("order {} placed by {}", order_id, user_id);

// GOOD: structured
tracing::info!(order.id = %order_id, user.id = %user_id, "order placed");
```

Do not also export logs over OTLP when stdout is collected, or every log record arrives twice; see [stdout vs OTLP](../logs.md#stdout-vs-otlp-for-log-delivery).
Do not add `opentelemetry-appender-tracing` for the same reason.

## Graceful shutdown

The batch processors hold up to 5 seconds of spans in memory.
Call `shutdown()` on every exit path, after the server stops accepting requests:

```rust
// src/main.rs
mod logs;
mod routes;
mod telemetry;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let telemetry = telemetry::init()?;

    let listener = tokio::net::TcpListener::bind("0.0.0.0:8080").await?;
    axum::serve(listener, routes::app())
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
2. Check that `cargo tree -i opentelemetry` does not fail as ambiguous; a duplicate makes the bridge send spans to a provider that is never installed.
3. Check that `OTEL_EXPORTER_OTLP_ENDPOINT` is the base URL, without `/v1/traces`; the HTTP exporter appends the signal path itself.
4. Check that the process calls `shutdown()` before exiting.

### Connection errors

1. For an `https://` endpoint, check that `opentelemetry-otlp` has the `reqwest-rustls` feature, per [installation](#installation).
2. In a container, check that the runtime image has `ca-certificates` installed.
3. Check that `OTEL_EXPORTER_OTLP_ENDPOINT` uses the OTLP/HTTP port, `4318` on a Collector, not the gRPC port `4317`.

### Spans missing for a library

1. Check that the library's spans pass the `EnvFilter`: an instrumentation crate that creates spans at `TRACE` or `DEBUG` level is filtered out at `info`, as `tonic-tracing-opentelemetry` is without its `tracing_level_info` feature.
2. Check that the crate's `opentelemetry` version matches the project's with `cargo tree -i opentelemetry`.
3. Check that the layer or middleware is applied to the service or client that handles the request, for example the shared `reqwest` client and not a plain `reqwest::Client`.

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
- [OpenTelemetry Rust contrib instrumentation](https://github.com/open-telemetry/opentelemetry-rust-contrib)
- [Environment variable specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)

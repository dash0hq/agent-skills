//! A Rust Lambda function instrumented with the OpenTelemetry Rust SDK in code.
//!
//! There is no upstream OpenTelemetry auto-instrumentation layer for Rust, so the
//! provider setup, the invocation span, and the end-of-invocation flush all live
//! here. The upstream Collector layer runs as a Lambda extension in the same
//! execution environment and receives OTLP over loopback.

use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;

use lambda_runtime::{Error, LambdaEvent, service_fn};
use opentelemetry::{
    KeyValue, global,
    metrics::{Counter, MeterProvider as _},
    trace::{Status, TraceContextExt, TracerProvider as _},
};
use opentelemetry_aws::detector::LambdaResourceDetector;
use opentelemetry_http::HeaderInjector;
use opentelemetry_otlp::{MetricExporter, SpanExporter};
use opentelemetry_sdk::{
    Resource,
    metrics::SdkMeterProvider,
    propagation::TraceContextPropagator,
    resource::{EnvResourceDetector, SdkProvidedResourceDetector, TelemetryResourceDetector},
    trace::{Sampler, SdkTracerProvider},
};
use reqwest_middleware::{ClientBuilder, ClientWithMiddleware, Middleware, Next};
use reqwest_tracing::TracingMiddleware;
use serde_json::{Value, json};
use tracing::{Instrument, Span, info_span};
use tracing_opentelemetry::OpenTelemetrySpanExt;
use tracing_subscriber::prelude::*;

/// Set until the first invocation of this execution environment has started.
static COLD_START: AtomicBool = AtomicBool::new(true);

/// Deliberately unresolvable: example.test is a reserved domain, so DNS fails and
/// the CLIENT span ends with status ERROR.
const UNREACHABLE_URL: &str = "https://example.test/";

/// Reachable: produces a successful CLIENT span.
const REACHABLE_URL: &str = "https://aws.amazon.com/";

/// Injects the current span's context into outgoing requests.
///
/// reqwest-tracing 0.7.1 has no `opentelemetry_0_33` feature, so it creates the
/// CLIENT span without injecting `traceparent`. Added after `TracingMiddleware`,
/// the current span here is that CLIENT span.
struct InjectTraceContext;

#[async_trait::async_trait]
impl Middleware for InjectTraceContext {
    async fn handle(
        &self,
        mut req: reqwest::Request,
        extensions: &mut http::Extensions,
        next: Next<'_>,
    ) -> reqwest_middleware::Result<reqwest::Response> {
        let cx = Span::current().context();
        global::get_text_map_propagator(|p| {
            p.inject_context(&cx, &mut HeaderInjector(req.headers_mut()))
        });
        next.run(req, extensions).await
    }
}

/// Everything the handler needs, built once per execution environment.
struct Demo {
    client: ClientWithMiddleware,
    invocations: Counter<u64>,
}

#[tokio::main]
async fn main() -> Result<(), Error> {
    // Built once per execution environment, during the init phase, so the cost
    // is not paid on every invocation.
    //
    // Lambda resource attributes first, then OTEL_RESOURCE_ATTRIBUTES and
    // OTEL_SERVICE_NAME, so the function's environment overrides the detector.
    // The OTLP endpoint and protocol come from the OTEL_* environment variables
    // set by the CDK stack; the exporters read them.
    let resource = Resource::builder_empty()
        .with_detector(Box::new(LambdaResourceDetector))
        .with_detectors(&[
            Box::new(TelemetryResourceDetector),
            Box::new(EnvResourceDetector::new()),
            Box::new(SdkProvidedResourceDetector),
        ])
        .build();
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

    let tracer = tracer_provider.tracer(env!("CARGO_PKG_NAME"));
    tracing_subscriber::registry()
        .with(tracing_opentelemetry::layer().with_tracer(tracer))
        .try_init()?;

    let demo = Demo {
        client: ClientBuilder::new(
            reqwest::Client::builder()
                .timeout(Duration::from_secs(5))
                .build()?,
        )
        .with(TracingMiddleware::default())
        .with(InjectTraceContext)
        .build(),
        invocations: meter_provider
            .meter(env!("CARGO_PKG_NAME"))
            .u64_counter("demo.invocations")
            .with_unit("{invocation}")
            .build(),
    };
    let (demo, tracer_provider, meter_provider) = (&demo, &tracer_provider, &meter_provider);

    lambda_runtime::run(service_fn(move |event: LambdaEvent<Value>| async move {
        // lambda_runtime's own tracing span is INTERNAL and already entered, so
        // create the invocation span here, with kind SERVER.
        let span = info_span!(
            "invocation",
            otel.name = %event.context.env_config.function_name,
            otel.kind = "server",
            faas.trigger = "http",
            faas.invocation_id = %event.context.request_id,
            faas.coldstart = COLD_START.swap(false, Ordering::Relaxed),
        );
        let result = handler(demo, event).instrument(span.clone()).await;
        if let Err(err) = &result {
            span.set_status(Status::error(err.to_string()));
        }

        // THE LAMBDA FLUSH. Lambda freezes the execution environment as soon as
        // the handler returns, so anything still buffered at that moment is lost.
        // End the span first: an open span is not exported. The meter provider
        // needs its own flush; its periodic reader would otherwise export only
        // when a later invocation happens to reuse this environment.
        drop(span);
        if let Err(err) = tracer_provider.force_flush() {
            eprintln!("trace flush failed: {err}");
        }
        if let Err(err) = meter_provider.force_flush() {
            eprintln!("metric flush failed: {err}");
        }
        result
    }))
    .await
}

/// The real handler: a little CPU work, one outbound call that fails, one that
/// succeeds, and a small JSON body.
async fn handler(demo: &Demo, event: LambdaEvent<Value>) -> Result<Value, Error> {
    demo.invocations.add(1, &[]);

    let request_id = format!("TEST-{}", event.context.request_id);
    println!("demo.invocation.start request_id={request_id}");

    let work = info_span!("demo.work", demo.request.id = %request_id, demo.checksum = tracing::field::Empty);
    let (checksum, probes) = async {
        let checksum = compute_checksum();
        Span::current().record("demo.checksum", checksum);
        let unreachable = probe(&demo.client, UNREACHABLE_URL).await;
        let reachable = probe(&demo.client, REACHABLE_URL).await;
        (checksum, [unreachable, reachable])
    }
    .instrument(work.clone())
    .await;
    work.set_status(Status::Ok);
    let trace_id = work.context().span().span_context().trace_id().to_string();
    drop(work);

    println!("demo.invocation.end request_id={request_id} checksum={checksum}");
    Ok(json!({
        "requestId": request_id,
        "checksum": checksum,
        "traceId": trace_id,
        "probes": probes,
    }))
}

/// A small amount of CPU work, so the invocation span has measurable duration.
fn compute_checksum() -> i64 {
    let _span = info_span!("demo.cpu", demo.iterations = 2_000_000).entered();
    (1..=2_000_000i64).fold(0, |acc, i| (acc + i * 31) % 1_000_003)
}

/// Issues one outbound HTTPS request and reports the outcome. Failures are
/// returned rather than propagated so the invocation keeps going; the failed
/// CLIENT span is the point of the exercise.
async fn probe(client: &ClientWithMiddleware, url: &str) -> Value {
    match client.get(url).send().await {
        Ok(res) => json!({ "url": url, "outcome": "ok", "statusCode": res.status().as_u16() }),
        Err(err) => {
            // reqwest-tracing already marked its own CLIENT span ERROR. Record the
            // failure on the enclosing custom span as an event, and carry on.
            Span::current().add_event(
                "exception",
                vec![KeyValue::new("exception.message", err.to_string())],
            );
            json!({ "url": url, "outcome": "failed", "error": err.to_string() })
        }
    }
}

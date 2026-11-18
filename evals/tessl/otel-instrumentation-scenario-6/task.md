# Order Service Observability in Rust

## Problem Description

Tidewater Supply runs `order-service`, a Rust HTTP service built with axum 0.8 on Tokio.
Its `POST /orders` handler reserves stock by calling the internal `inventory.v1.Inventory/Reserve` gRPC method with a tonic 0.14 client, then answers with a small JSON body such as `{"order_id": "TEST-0001", "status": "confirmed"}`.
The service currently logs with `tracing_subscriber::fmt::init()` and has no OpenTelemetry instrumentation.

The platform team wants every request to show up as a trace that includes the gRPC call, request latency and error rate per route on a dashboard, and log lines that can be correlated with those traces.
The service runs on Kubernetes, where the deployment sets `service.instance.id` to the pod UID and `deployment.environment.name` through `OTEL_RESOURCE_ATTRIBUTES`; those values must be what the backend shows.

Telemetry goes straight to the backend:

- OTLP via HTTP endpoint: `https://ingress.example.test`
- Auth token: `TEST-INGEST-0001`
- Required `service.name`: `order-service`

The runtime image is built from `debian:trixie-slim`.

## Output Specification

Produce the following files:

1. **`Cargo.toml`** — The `[dependencies]` section with every crate the instrumentation needs.
2. **`src/telemetry.rs`** — The OpenTelemetry setup, called once from `main`.
3. **`src/main.rs`** — The axum router, the gRPC client wiring, and the startup and shutdown sequence.
4. **`Dockerfile`** — The multi-stage build of the service.
5. **`NOTES.md`** — A short handover note listing the environment variables the deployment must set and anything the team must verify before shipping.

Keep the endpoint and token values above as literal placeholders, and read them from environment variables rather than hardcoding them in the code.

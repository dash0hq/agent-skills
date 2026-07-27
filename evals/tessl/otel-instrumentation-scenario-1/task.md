# Payment API Observability Setup

## Problem Description

FinPay Ltd. is launching a payment processing API built with Node.js and Express. The service is a CommonJS application that handles credit card payments on behalf of merchants. Before going to production, the engineering team needs to add OpenTelemetry instrumentation so the operations team can monitor request latency, detect errors, and correlate logs to traces during incidents.

The service already has a basic Express app (`app.js`) and runs behind an OpenTelemetry Collector that listens on gRPC at port 4317. The team wants traces, metrics, and logs all exported via OTLP. The collector endpoint is `http://otel-collector:4317`. Authentication is handled by a pre-existing environment variable `OTEL_AUTH_TOKEN`.

Because FinPay processes credit card payments, the team is especially concerned about accidentally leaking sensitive data (card numbers, auth tokens, user emails) into the telemetry pipeline. Request URLs may contain session tokens as query parameters, and log statements must not expose raw authorization headers or card data.

The codebase contains three HTTP routes that need proper instrumentation:
- `POST /api/payments` — initiates a payment; may return 400 (invalid request), 402 (insufficient funds), or 500 (processor unavailable)
- `GET /api/payments/:paymentId` — retrieves a payment record; returns 404 if not found
- `DELETE /api/payments/:paymentId/cancel` — cancels a pending payment

The team also needs proper error handling so that if the Node.js process crashes unexpectedly, any in-flight telemetry is flushed to the backend before the process exits.

## Output Specification

Produce the following files in your working directory:

1. **`instrumentation.js`** — OpenTelemetry SDK setup and initialization code that can be loaded before the app
2. **`.env.local`** — Environment variable configuration for running the instrumented service locally
3. **`payment-handlers.js`** — Example implementation of the three route handlers above, with correct span status handling, exception recording (using structured log records), sensitive data sanitization, and trace-log correlation
4. **`package.json`** — Package manifest listing the required OTel dependencies and npm scripts to run the instrumented app

The files should be self-consistent and runnable (or near-runnable) without additional input. Placeholder values are acceptable for the OTLP endpoint and auth token where real values are not available.


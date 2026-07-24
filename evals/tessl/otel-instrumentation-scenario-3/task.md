# Inventory Sync Worker Instrumentation

## Problem Description

DataBridge Inc. runs a nightly inventory synchronization worker that pulls updated stock levels from a third-party supplier API, reconciles them against an internal database, and flags discrepancies for review. The worker runs as a cron job (no inbound HTTP requests) on a schedule and is implemented in Node.js.

The team recently added basic OpenTelemetry tracing to the worker but noticed that all their traces appear as disconnected CLIENT spans in the backend — making it impossible to see the full picture of what the worker is doing per run. They also have a retry mechanism for transient supplier API failures, but the traces show errors even for runs that ultimately succeed after a retry.

The worker processes a batch of product SKUs per run (typically 200–500 items). A previous developer wrapped each individual SKU reconciliation in its own span, which generated thousands of tiny spans per run and caused the observability backend to reject the data as too high-cardinality.

The worker structure is as follows:
1. Fetch a batch of SKU updates from the supplier REST API (with up to 3 retries on 5xx or network errors)
2. For each SKU in the batch, compare against the database record
3. Write a summary of discrepancies to the database
4. Log a completion event

Occasionally the supplier API returns a 500 error or times out. When all retries are exhausted, the run should be marked as failed. If a retry succeeds, the run continues normally and should not be marked as failed.

## Output Specification

Produce the following files:

1. **`worker.js`** — The main inventory sync worker implementation with full OpenTelemetry instrumentation. Include all tracing logic: root span setup, outbound call spans, retry handling, exception recording, and graceful shutdown.

2. **`otel-setup.js`** — SDK initialization code that sets up the tracer provider and exporter. Include resource attributes and any crash handler registration.

3. **`README.md`** — A short explanation (3–10 bullet points) of the key tracing decisions made in the implementation: why certain span kinds were chosen, how sampling is configured, and how the batch processing is instrumented.

The files should be self-consistent. Use placeholder values for endpoint URLs and auth tokens. No real external services need to be called.


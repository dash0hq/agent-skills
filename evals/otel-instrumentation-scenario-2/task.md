# Order Fulfillment Service Metrics

## Problem Description

ShopStream is a mid-size e-commerce platform whose order fulfillment service processes thousands of orders per day. The service is a Node.js application that already has `@opentelemetry/auto-instrumentations-node` installed and running (which handles HTTP server and HTTP client spans and metrics automatically).

The engineering team has noticed that their observability dashboards show generic infrastructure metrics but nothing about the business operations that actually matter for SLA compliance. They want to add three custom metrics to the fulfillment service:

1. **Order processing duration** — how long (in wall-clock time) it takes to fully process an order from receipt to completion. This needs percentile analysis (p50, p95, p99) to catch long-tail slowness.

2. **Orders processed** — a running count of orders that have completed processing, broken down by outcome (fulfilled, cancelled, failed). This will be used for throughput rate calculation on dashboards.

3. **Fulfillment queue depth** — the current number of orders waiting in the queue at any point in time. This can go up (when orders arrive) and down (when orders are processed or cancelled). Used for autoscaling decisions.

The team's previous attempt added a metric called `http.server.orders.processed` that appeared to duplicate data the existing instrumentation already captured, and the unit was expressed as `orders.count.total` in the name itself. These problems caused backend cost issues and confusing dashboards.

## Output Specification

Produce the following files:

1. **`metrics.js`** — A Node.js module that creates and exports the three metrics described above. Include the meter initialization and metric creation with all necessary configuration.

2. **`fulfillment-service.js`** — A short example showing how the metrics are recorded in a realistic order processing workflow. Demonstrate incrementing/decrementing, recording durations, and what attributes are attached to each metric.

3. **`metrics.test.js`** — Integration tests (using any test framework or plain Node.js assertions) that verify the shape of each custom metric: name, instrument type, unit, and attribute keys.

The files should work with standard OpenTelemetry Node.js packages available on npm. Use placeholder values for any endpoint configuration.


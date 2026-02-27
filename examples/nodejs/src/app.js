/**
 * OpenTelemetry Node.js Example
 *
 * This example demonstrates proper OpenTelemetry instrumentation:
 *
 * 1. AUTO-INSTRUMENTATION
 *    - HTTP requests (Express) via @opentelemetry/auto-instrumentations-node
 *    - Automatic trace propagation
 *
 * 2. CUSTOM SPANS
 *    - Business logic spans (order.process, payment.process)
 *    - Proper error handling with recordException()
 *    - Meaningful span names (noun.verb format)
 *
 * 3. GOLDEN SIGNAL METRICS
 *    - Latency: http.server.duration histogram
 *    - Traffic: http.server.requests counter
 *    - Errors: http.server.errors counter
 *    - Saturation: http.server.active_connections gauge
 *
 * 4. STRUCTURED LOGGING
 *    - Trace correlation (trace_id, span_id)
 *    - Structured attributes (not string interpolation)
 *    - Appropriate log levels by environment
 *
 * 5. CARDINALITY MANAGEMENT
 *    - Normalized paths on metrics
 *    - Bucketed status codes on metrics
 *    - High-cardinality attributes only on spans
 */

import express from "express";
import { metricsMiddleware } from "./middleware/metrics.js";
import { processOrder, getOrder, processBatch } from "./services/order-service.js";
import { tracer, withSpan } from "./telemetry.js";
import logger from "./logger.js";

const app = express();
app.use(express.json());

// Apply metrics middleware to all routes
app.use(metricsMiddleware);

// ============================================================================
// ROUTES
// ============================================================================

/**
 * Health check endpoint
 * Minimal instrumentation - auto-instrumented only
 */
app.get("/health", (req, res) => {
  res.json({ status: "healthy" });
});

/**
 * Create order
 * Demonstrates: custom spans, business metrics, error handling
 */
app.post("/orders", async (req, res) => {
  try {
    const order = {
      id: generateOrderId(),
      items: req.body.items || [{ name: "Default Item", price: 10 }],
      total: req.body.total || 99.99,
      currency: req.body.currency || "usd",
    };

    const result = await processOrder(order);

    if (!result.success) {
      return res.status(400).json({
        error: "Order processing failed",
        reason: result.reason,
      });
    }

    res.status(201).json({
      message: "Order created",
      orderId: result.orderId,
    });
  } catch (error) {
    logger.error("order.creation_failed", {
      error_type: error.name,
      error_message: error.message,
    });

    res.status(500).json({
      error: "Internal server error",
    });
  }
});

/**
 * Get order by ID
 * Demonstrates: path parameter normalization in metrics
 */
app.get("/orders/:id", async (req, res) => {
  const order = await getOrder(req.params.id);

  if (!order) {
    return res.status(404).json({ error: "Order not found" });
  }

  res.json(order);
});

/**
 * Batch processing endpoint
 * Demonstrates: GOOD pattern - single span for batch, not per-item
 */
app.post("/batch", async (req, res) => {
  try {
    const items = req.body.items || [
      { id: 1, data: "item1" },
      { id: 2, data: "item2" },
      { id: 3, data: "item3" },
    ];

    const results = await processBatch(items);

    res.json({
      message: "Batch processed",
      count: results.length,
      results,
    });
  } catch (error) {
    logger.error("batch.processing_failed", {
      error_type: error.name,
      error_message: error.message,
    });

    res.status(500).json({ error: "Batch processing failed" });
  }
});

/**
 * Simulate slow endpoint
 * Demonstrates: latency tracking in metrics
 */
app.get("/slow", async (req, res) => {
  await withSpan("slow.operation", { "delay.ms": 500 }, async () => {
    await new Promise((resolve) => setTimeout(resolve, 500));
    logger.debug("slow.operation.completed");
  });

  res.json({ message: "Slow operation completed" });
});

/**
 * Simulate error endpoint
 * Demonstrates: error recording in spans and metrics
 */
app.get("/error", async (req, res) => {
  try {
    await withSpan("error.simulation", {}, async () => {
      throw new Error("Simulated error for testing");
    });
  } catch (error) {
    logger.error("simulated.error", {
      error_type: error.name,
      error_message: error.message,
    });

    res.status(500).json({
      error: "Simulated error",
      message: error.message,
    });
  }
});

/**
 * Nested spans example
 * Demonstrates: parent-child span relationships
 */
app.get("/nested", async (req, res) => {
  await withSpan("parent.operation", {}, async () => {
    logger.info("parent.started");

    await withSpan("child.operation.1", {}, async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
      logger.debug("child.1.completed");
    });

    await withSpan("child.operation.2", {}, async () => {
      await new Promise((resolve) => setTimeout(resolve, 30));
      logger.debug("child.2.completed");
    });

    logger.info("parent.completed");
  });

  res.json({ message: "Nested operations completed" });
});

// ============================================================================
// STARTUP
// ============================================================================

const PORT = process.env.PORT || 3001;

app.listen(PORT, () => {
  logger.info("server.started", {
    port: PORT,
    node_env: process.env.NODE_ENV || "development",
    otel_service: process.env.OTEL_SERVICE_NAME || "otel-nodejs-example",
  });

  console.log(`
===========================================
 OpenTelemetry Node.js Example
===========================================

Server running on http://localhost:${PORT}

Available endpoints:
  GET  /health     - Health check
  POST /orders     - Create order (custom spans + metrics)
  GET  /orders/:id - Get order by ID
  POST /batch      - Batch processing example
  GET  /slow       - Simulate slow operation
  GET  /error      - Simulate error
  GET  /nested     - Nested spans example

Test with:
  curl http://localhost:${PORT}/health
  curl -X POST http://localhost:${PORT}/orders -H "Content-Type: application/json" -d '{"total": 49.99}'
  curl http://localhost:${PORT}/slow
  curl http://localhost:${PORT}/error
  curl http://localhost:${PORT}/nested

===========================================
  `);
});

// ============================================================================
// HELPERS
// ============================================================================

function generateOrderId() {
  return `ord_${Date.now()}_${Math.random().toString(36).substring(2, 8)}`;
}

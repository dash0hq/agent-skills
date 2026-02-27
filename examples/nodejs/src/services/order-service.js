/**
 * Order Service - Business logic with proper instrumentation
 *
 * Demonstrates:
 * - Custom spans for business operations
 * - Proper attribute placement (high-cardinality on spans, not metrics)
 * - Structured logging with trace correlation
 * - Business metrics with bounded attributes
 */

import {
  withSpan,
  ordersProcessed,
  orderValue,
  SpanStatusCode,
} from "../telemetry.js";
import logger from "../logger.js";

// Simulated database
const orders = new Map();

/**
 * Process a batch of items
 *
 * GOOD PATTERN: Single span for batch operations
 * BAD PATTERN: Creating a span per item (cardinality explosion)
 */
export async function processBatch(items) {
  return withSpan("batch.process", { "batch.size": items.length }, async () => {
    logger.info("batch.started", { item_count: items.length });

    // Process all items in the batch
    // Note: We do NOT create a span per item!
    const results = items.map((item) => ({
      id: item.id,
      processed: true,
    }));

    logger.info("batch.completed", {
      item_count: items.length,
      success_count: results.length,
    });

    return results;
  });
}

/**
 * Process an order
 *
 * Demonstrates:
 * - Custom spans with business attributes
 * - Metrics with bounded attributes only
 * - Structured logging with trace context
 */
export async function processOrder(order) {
  return withSpan(
    "order.process",
    {
      // OK on spans: order.id (high cardinality but bounded per trace)
      "order.id": order.id,
      "order.total": order.total,
      "order.item_count": order.items?.length || 0,
    },
    async (span) => {
      logger.info("order.processing", {
        order_id: order.id,
        total: order.total,
      });

      // Simulate validation
      await validateOrder(order);

      // Simulate payment processing
      const paymentResult = await processPayment(order);

      if (!paymentResult.success) {
        // Record metric with BOUNDED attributes only
        ordersProcessed.add(1, {
          status: "failed",
          failure_reason: paymentResult.reason, // Bounded: "insufficient_funds", "card_declined", etc.
        });

        span.setStatus({
          code: SpanStatusCode.ERROR,
          message: paymentResult.reason,
        });

        logger.warn("order.payment_failed", {
          order_id: order.id,
          reason: paymentResult.reason,
        });

        return { success: false, reason: paymentResult.reason };
      }

      // Save order
      orders.set(order.id, { ...order, status: "completed" });

      // Record success metrics
      // Note: NO user_id or order_id on metrics (unbounded!)
      ordersProcessed.add(1, { status: "completed" });
      orderValue.record(order.total, {
        currency: order.currency || "usd",
      });

      logger.info("order.completed", {
        order_id: order.id,
        total: order.total,
      });

      return { success: true, orderId: order.id };
    }
  );
}

/**
 * Validate order
 * Uses custom span for significant business operation
 */
async function validateOrder(order) {
  return withSpan("order.validate", { "order.id": order.id }, async () => {
    // Simulate validation delay
    await sleep(10);

    if (!order.items || order.items.length === 0) {
      throw new Error("Order must have at least one item");
    }

    if (order.total <= 0) {
      throw new Error("Order total must be positive");
    }

    logger.debug("order.validated", { order_id: order.id });
    return true;
  });
}

/**
 * Process payment
 * Uses custom span for external service call
 */
async function processPayment(order) {
  return withSpan(
    "payment.process",
    {
      "order.id": order.id,
      "payment.amount": order.total,
      "payment.currency": order.currency || "usd",
    },
    async (span) => {
      // Simulate payment processing delay
      await sleep(50);

      // Simulate occasional payment failures (10% chance)
      if (Math.random() < 0.1) {
        const reason = "card_declined";
        span.addEvent("payment.declined", { reason });
        return { success: false, reason };
      }

      span.addEvent("payment.authorized");
      logger.info("payment.authorized", {
        order_id: order.id,
        amount: order.total,
      });

      return { success: true };
    }
  );
}

/**
 * Get order by ID
 */
export async function getOrder(orderId) {
  return withSpan("order.get", { "order.id": orderId }, async () => {
    const order = orders.get(orderId);

    if (!order) {
      logger.warn("order.not_found", { order_id: orderId });
      return null;
    }

    return order;
  });
}

// Helper
function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

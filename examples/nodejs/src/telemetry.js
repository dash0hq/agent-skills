/**
 * Telemetry utilities for OpenTelemetry instrumentation
 *
 * This module provides:
 * - Custom tracer for creating spans
 * - Custom metrics (golden signals)
 * - Trace context extraction for log correlation
 */

import { trace, context, metrics, SpanStatusCode } from "@opentelemetry/api";

// Service name should match OTEL_SERVICE_NAME
const SERVICE_NAME = process.env.OTEL_SERVICE_NAME || "otel-nodejs-example";

// Get tracer for creating custom spans
export const tracer = trace.getTracer(SERVICE_NAME);

// Get meter for creating custom metrics
export const meter = metrics.getMeter(SERVICE_NAME);

// ============================================================================
// GOLDEN SIGNAL METRICS
// ============================================================================

/**
 * Latency histogram - measures request duration
 * Use for: SLO tracking, performance monitoring
 */
export const httpDuration = meter.createHistogram("http.server.duration", {
  description: "HTTP request duration in milliseconds",
  unit: "ms",
});

/**
 * Traffic counter - counts total requests
 * Use for: Traffic monitoring, rate calculations
 */
export const httpRequests = meter.createCounter("http.server.requests", {
  description: "Total number of HTTP requests",
});

/**
 * Error counter - counts failed requests
 * Use for: Error rate SLOs, alerting
 */
export const httpErrors = meter.createCounter("http.server.errors", {
  description: "Total number of HTTP errors",
});

/**
 * Saturation gauge - tracks active connections
 * Use for: Capacity planning, load monitoring
 */
export const activeConnections = meter.createUpDownCounter(
  "http.server.active_connections",
  {
    description: "Number of active HTTP connections",
  }
);

// ============================================================================
// BUSINESS METRICS
// ============================================================================

/**
 * Order counter - tracks orders by status
 * Note: Only use bounded attributes (status), never user_id
 */
export const ordersProcessed = meter.createCounter("orders.processed", {
  description: "Total orders processed",
});

/**
 * Order value histogram - tracks order values for revenue analysis
 */
export const orderValue = meter.createHistogram("orders.value", {
  description: "Order value distribution",
  unit: "usd",
});

// ============================================================================
// TRACE CONTEXT HELPERS
// ============================================================================

/**
 * Extract trace context for log correlation
 *
 * Usage:
 *   logger.info("order.placed", { ...getTraceContext(), order_id: orderId });
 *
 * This enables navigating from traces to related log entries
 */
export function getTraceContext() {
  const span = trace.getSpan(context.active());
  if (!span) return {};

  const ctx = span.spanContext();
  return {
    trace_id: ctx.traceId,
    span_id: ctx.spanId,
  };
}

// ============================================================================
// SPAN HELPERS
// ============================================================================

/**
 * Normalize URL paths for use as metric attributes
 * Prevents cardinality explosion from dynamic path segments
 *
 * Example: /users/123/orders/456 -> /users/{id}/orders/{id}
 */
export function normalizePath(path) {
  return path
    .replace(/\/\d+/g, "/{id}") // Replace numeric IDs
    .replace(/\/[a-f0-9-]{36}/gi, "/{uuid}"); // Replace UUIDs
}

/**
 * Bucket HTTP status codes to reduce cardinality
 * Use for metric attributes, not span attributes
 */
export function bucketStatusCode(code) {
  if (code >= 200 && code < 300) return "2xx";
  if (code >= 300 && code < 400) return "3xx";
  if (code >= 400 && code < 500) return "4xx";
  if (code >= 500) return "5xx";
  return "unknown";
}

/**
 * Create a span with proper error handling
 * Ensures span.end() is always called and errors are recorded
 */
export async function withSpan(name, attributes, fn) {
  return tracer.startActiveSpan(name, async (span) => {
    try {
      // Set initial attributes
      for (const [key, value] of Object.entries(attributes || {})) {
        span.setAttribute(key, value);
      }

      const result = await fn(span);
      return result;
    } catch (error) {
      // Record exception and set error status
      span.recordException(error);
      span.setStatus({
        code: SpanStatusCode.ERROR,
        message: error.message,
      });
      throw error;
    } finally {
      span.end();
    }
  });
}

// Re-export SpanStatusCode for convenience
export { SpanStatusCode };

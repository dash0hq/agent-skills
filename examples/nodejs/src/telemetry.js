/**
 * Telemetry utilities for OpenTelemetry instrumentation
 *
 * This module provides:
 * - Custom tracer for creating spans
 * - Custom metrics (golden signals)
 * - Trace context extraction for log correlation
 */

import { trace, context, metrics, SpanStatusCode } from "@opentelemetry/api";
import { logs, SeverityNumber } from "@opentelemetry/api-logs";

// Service name should match OTEL_SERVICE_NAME
const SERVICE_NAME = process.env.OTEL_SERVICE_NAME || "otel-nodejs-example";

// Get tracer for creating custom spans
export const tracer = trace.getTracer(SERVICE_NAME);

// Get meter for creating custom metrics
export const meter = metrics.getMeter(SERVICE_NAME);

// ============================================================================
// BUSINESS METRICS
// ============================================================================
// HTTP golden signal metrics (http.server.request.duration, http.server.active_requests)
// are created in middleware/metrics.js using the stable HTTP semantic conventions.
// The installed @opentelemetry/instrumentation-http (0.53.x) still uses old
// semconv names, so the middleware creates the correct metrics manually.

/**
 * Order counter - tracks orders by status
 * Note: Only use bounded attributes (status), never user_id
 */
export const ordersProcessed = meter.createCounter("orders.processed", {
  description: "Total orders processed",
  unit: "1",
});

/**
 * Order value histogram - tracks order values for revenue analysis
 */
export const orderValue = meter.createHistogram("orders.value", {
  description: "Order value distribution",
  unit: "{USD}",
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
      // Record exception as a structured log record (not span.recordException,
      // which uses the deprecated Span Event API)
      const spanContext = span.spanContext();
      logs.getLogger(SERVICE_NAME).emit({
        severityNumber: SeverityNumber.ERROR,
        severityText: "ERROR",
        body: "exception",
        attributes: {
          trace_id: spanContext.traceId,
          span_id: spanContext.spanId,
          "exception.type": error.name,
          "exception.message": error.message,
          "exception.stacktrace": error.stack,
        },
      });
      span.setStatus({
        code: SpanStatusCode.ERROR,
        message: `${error.name}: ${error.message}`,
      });
      throw error;
    } finally {
      span.end();
    }
  });
}

import { trace, context, SpanStatusCode, metrics } from "@opentelemetry/api";
import { logs, SeverityNumber } from "@opentelemetry/api-logs";

// Get tracer for custom spans
export function getTracer(name = "nextjs-demo") {
  return trace.getTracer(name);
}

// Get meter for custom metrics
export function getMeter(name = "nextjs-demo") {
  return metrics.getMeter(name);
}

// Get logger for structured logs
export function getLogger(name = "nextjs-demo") {
  return logs.getLogger(name);
}

// Extract trace context for correlation
export function getTraceContext() {
  const span = trace.getSpan(context.active());
  if (!span) return {};
  const ctx = span.spanContext();
  return {
    trace_id: ctx.traceId,
    span_id: ctx.spanId,
  };
}

// Structured logger that includes trace context
export const logger = {
  info(message: string, attributes: Record<string, unknown> = {}) {
    const otelLogger = getLogger();
    otelLogger.emit({
      severityNumber: SeverityNumber.INFO,
      severityText: "INFO",
      body: message,
      attributes: {
        ...getTraceContext(),
        ...attributes,
      },
    });
  },

  warn(message: string, attributes: Record<string, unknown> = {}) {
    const otelLogger = getLogger();
    otelLogger.emit({
      severityNumber: SeverityNumber.WARN,
      severityText: "WARN",
      body: message,
      attributes: {
        ...getTraceContext(),
        ...attributes,
      },
    });
  },

  error(message: string, attributes: Record<string, unknown> = {}) {
    const otelLogger = getLogger();
    otelLogger.emit({
      severityNumber: SeverityNumber.ERROR,
      severityText: "ERROR",
      body: message,
      attributes: {
        ...getTraceContext(),
        ...attributes,
      },
    });
  },
};

// Helper to wrap async functions with spans
export async function withSpan<T>(
  name: string,
  attributes: Record<string, string | number | boolean>,
  fn: () => Promise<T>
): Promise<T> {
  const tracer = getTracer();
  return tracer.startActiveSpan(name, async (span) => {
    try {
      Object.entries(attributes).forEach(([key, value]) => {
        span.setAttribute(key, value);
      });
      const result = await fn();
      return result;
    } catch (error) {
      const err = error as Error;
      // Record exception as a structured log record (not span.recordException,
      // which uses the deprecated Span Event API)
      const spanContext = span.spanContext();
      getLogger().emit({
        severityNumber: SeverityNumber.ERROR,
        severityText: "ERROR",
        body: "exception",
        attributes: {
          trace_id: spanContext.traceId,
          span_id: spanContext.spanId,
          "exception.type": err.name,
          "exception.message": err.message,
          "exception.stacktrace": err.stack,
        },
      });
      span.setStatus({
        code: SpanStatusCode.ERROR,
        message: `${err.name}: ${err.message}`,
      });
      throw error;
    } finally {
      span.end();
    }
  });
}

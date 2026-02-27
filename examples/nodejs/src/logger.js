/**
 * Structured logging with trace correlation
 *
 * Key principles:
 * - Always use structured logging (objects, not string interpolation)
 * - Include trace context for correlation with spans
 * - Use appropriate log levels by environment
 */

import pino from "pino";
import { getTraceContext } from "./telemetry.js";

const isDev = process.env.NODE_ENV !== "production";

// Create base logger with appropriate settings
const baseLogger = pino({
  level: isDev ? "debug" : "info",
  // Pretty print in development
  transport: isDev
    ? {
        target: "pino-pretty",
        options: {
          colorize: true,
        },
      }
    : undefined,
});

/**
 * Logger wrapper that automatically includes trace context
 *
 * Usage:
 *   logger.info("order.placed", { order_id: orderId, amount: 99.99 });
 *
 * Output includes trace_id and span_id for correlation
 */
export const logger = {
  /**
   * Error level - always logged in all environments
   * Use for: Exceptions, failed operations, unrecoverable errors
   */
  error(message, data = {}) {
    baseLogger.error({ ...getTraceContext(), ...data }, message);
  },

  /**
   * Warn level - always logged in all environments
   * Use for: Degraded performance, recoverable errors, deprecations
   */
  warn(message, data = {}) {
    baseLogger.warn({ ...getTraceContext(), ...data }, message);
  },

  /**
   * Info level - sampled in production (10%), always in development
   * Use for: Business events, state transitions, key operations
   */
  info(message, data = {}) {
    baseLogger.info({ ...getTraceContext(), ...data }, message);
  },

  /**
   * Debug level - never in production, always in development
   * Use for: Detailed debugging, variable inspection
   */
  debug(message, data = {}) {
    baseLogger.debug({ ...getTraceContext(), ...data }, message);
  },
};

export default logger;

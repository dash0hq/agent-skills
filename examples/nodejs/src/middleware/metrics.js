/**
 * HTTP Metrics Middleware
 *
 * Records golden signal metrics for all HTTP requests:
 * - Latency (histogram)
 * - Traffic (counter)
 * - Errors (counter)
 * - Saturation (active connections)
 *
 * Key principles:
 * - Normalize paths to prevent cardinality explosion
 * - Bucket status codes for metrics (exact codes on spans)
 * - Never include unbounded attributes (user_id, request_id)
 */

import {
  httpDuration,
  httpRequests,
  httpErrors,
  activeConnections,
  normalizePath,
  bucketStatusCode,
} from "../telemetry.js";
import logger from "../logger.js";

/**
 * Express middleware for HTTP metrics
 */
export function metricsMiddleware(req, res, next) {
  const startTime = Date.now();

  // Track active connections (saturation)
  activeConnections.add(1);

  // Capture response finish
  res.on("finish", () => {
    const duration = Date.now() - startTime;
    const normalizedPath = normalizePath(req.path);
    const statusBucket = bucketStatusCode(res.statusCode);

    // Metric attributes - BOUNDED only!
    const metricAttributes = {
      method: req.method,
      route: normalizedPath,
      status: statusBucket,
    };

    // Record latency
    httpDuration.record(duration, metricAttributes);

    // Record request count
    httpRequests.add(1, metricAttributes);

    // Record errors (4xx and 5xx)
    if (res.statusCode >= 400) {
      httpErrors.add(1, metricAttributes);
    }

    // Release active connection
    activeConnections.add(-1);

    // Structured log with request details
    // Logs can have higher cardinality than metrics
    logger.info("http.request", {
      method: req.method,
      path: req.path, // Full path OK in logs
      status: res.statusCode, // Exact code OK in logs
      duration_ms: duration,
      user_agent: req.get("user-agent"),
    });
  });

  next();
}

export default metricsMiddleware;

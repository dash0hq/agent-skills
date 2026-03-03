/**
 * HTTP Metrics Middleware
 *
 * Records golden signal metrics for all HTTP requests using semconv names:
 * - Latency: http.server.request.duration histogram (traffic count derived from it)
 * - Errors: derived from the histogram filtered by http.response.status_code
 * - Saturation: http.server.active_requests gauge
 *
 * Key principles:
 * - Use semantic convention metric names and attribute keys whenever applicable
 * - Use bounded attributes only on metrics (e.g. status_code, not user_id)
 * - Normalize paths to prevent cardinality explosion
 * - Never include unbounded attributes (user_id, request_id)
 */

import {
  httpDuration,
  activeRequests,
  normalizePath,
} from "../telemetry.js";
import logger from "../logger.js";

/**
 * Express middleware for HTTP metrics
 */
export function metricsMiddleware(req, res, next) {
  const startTime = performance.now();

  // Track active requests (saturation)
  activeRequests.add(1);

  // Capture response finish
  res.on("finish", () => {
    const durationS = (performance.now() - startTime) / 1000;
    const normalizedPath = normalizePath(req.path);

    // Metric attributes - use semconv attribute keys, BOUNDED only!
    const metricAttributes = {
      "http.request.method": req.method,
      "http.route": normalizedPath,
      "http.response.status_code": res.statusCode,
    };

    // Record duration — traffic count and error rate are derived from this histogram
    httpDuration.record(durationS, metricAttributes);

    // Release active request
    activeRequests.add(-1);

    // Structured log with request details
    // Logs can have higher cardinality than metrics
    logger.info("http.request", {
      method: req.method,
      path: req.path, // Full path OK in logs
      status: res.statusCode, // Exact code OK in logs
      duration_ms: Math.round(durationS * 1000),
      user_agent: req.get("user-agent"),
    });
  });

  next();
}

export default metricsMiddleware;

/**
 * HTTP metrics middleware
 *
 * Creates http.server.request.duration and http.server.active_requests
 * following stable HTTP semantic conventions.
 *
 * Why manual instead of auto-instrumented?
 * The installed @opentelemetry/instrumentation-http (0.53.x) emits metrics
 * under old semconv names (http.server.duration, unit ms) and does not
 * support the semconvStabilityOptIn option. This middleware creates the
 * correct stable semconv metrics (http.server.request.duration, unit s)
 * so that dashboards and alerts use consistent, forward-compatible names.
 * The outdated metrics from instrumentation-http are dropped in the
 * Collector with a filter processor — see otel-collector configuration.
 *
 * When @opentelemetry/instrumentation-http ships stable HTTP semconv
 * support, remove this middleware and the Collector filter, and rely on
 * auto-instrumentation instead.
 */

import { meter } from "../telemetry.js";

const requestDuration = meter.createHistogram("http.server.request.duration", {
  description: "Duration of HTTP server requests",
  unit: "s",
});

const activeRequests = meter.createUpDownCounter("http.server.active_requests", {
  description: "Number of active HTTP server requests",
  unit: "1",
});

/**
 * Express middleware for HTTP server metrics
 */
export function metricsMiddleware(req, res, next) {
  const startTime = performance.now();

  activeRequests.add(1, { "http.request.method": req.method });

  res.on("finish", () => {
    const durationS = (performance.now() - startTime) / 1000;
    const route = req.route?.path
      ? req.baseUrl + req.route.path
      : req.path;

    requestDuration.record(durationS, {
      "http.request.method": req.method,
      "http.route": route,
      "http.response.status_code": res.statusCode,
    });

    activeRequests.add(-1, { "http.request.method": req.method });
  });

  next();
}

export default metricsMiddleware;

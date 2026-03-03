/**
 * Telemetry integration tests
 *
 * Verifies that the application produces the expected metrics and spans,
 * following the testing patterns from otel-instrumentation skill rules.
 *
 * Uses in-memory exporters so no collector is needed.
 */

import { describe, it, after, afterEach, before } from "node:test";
import assert from "node:assert/strict";
import http from "node:http";

import { SpanKind, context } from "@opentelemetry/api";
import { suppressTracing } from "@opentelemetry/core";
import { NodeSDK } from "@opentelemetry/sdk-node";
import {
  InMemorySpanExporter,
  SimpleSpanProcessor,
} from "@opentelemetry/sdk-trace-base";
import {
  InMemoryMetricExporter,
  PeriodicExportingMetricReader,
  AggregationTemporality,
} from "@opentelemetry/sdk-metrics";
import { HttpInstrumentation } from "@opentelemetry/instrumentation-http";
import { ExpressInstrumentation } from "@opentelemetry/instrumentation-express";

// ---------------------------------------------------------------------------
// SDK setup — must happen before the app is imported
// ---------------------------------------------------------------------------

const spanExporter = new InMemorySpanExporter();
const metricExporter = new InMemoryMetricExporter(
  AggregationTemporality.CUMULATIVE,
);
const metricReader = new PeriodicExportingMetricReader({
  exporter: metricExporter,
  exportIntervalMillis: 100,
});

const sdk = new NodeSDK({
  spanProcessors: [new SimpleSpanProcessor(spanExporter)],
  metricReader,
  instrumentations: [new HttpInstrumentation(), new ExpressInstrumentation()],
});
sdk.start();

// Import app after SDK is initialised so instrumentation hooks are in place
const { app } = await import("../src/app.js");

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

let server;
let baseUrl;

/**
 * Make an HTTP request to the test server.
 * Tracing is suppressed so the test client does not produce CLIENT spans
 * that would pollute span hygiene assertions.
 */
function request(method, path, body) {
  return context.with(suppressTracing(context.active()), () => {
    return new Promise((resolve, reject) => {
      const url = new URL(path, baseUrl);
      const opts = {
        method,
        hostname: url.hostname,
        port: url.port,
        path: url.pathname,
      };
      const headers = {};
      let payload;

      if (body !== undefined) {
        payload = JSON.stringify(body);
        headers["Content-Type"] = "application/json";
        headers["Content-Length"] = Buffer.byteLength(payload);
      }
      opts.headers = headers;

      const req = http.request(opts, (res) => {
        const chunks = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () => {
          resolve({
            status: res.statusCode,
            body: Buffer.concat(chunks).toString(),
          });
        });
      });
      req.on("error", reject);
      if (payload) req.write(payload);
      req.end();
    });
  });
}

async function collectMetrics() {
  await metricReader.forceFlush();
  return metricExporter.getMetrics();
}

function resetMetrics() {
  metricExporter.reset();
}

function getSpans() {
  return spanExporter.getFinishedSpans();
}

function resetSpans() {
  spanExporter.reset();
}

function findMetric(name) {
  for (const rm of metricExporter.getMetrics()) {
    for (const sm of rm.scopeMetrics) {
      for (const metric of sm.metrics) {
        if (metric.descriptor.name === name) {
          const attrKeys = new Set();
          for (const dp of metric.dataPoints) {
            for (const key of Object.keys(dp.attributes)) {
              attrKeys.add(key);
            }
          }
          return {
            name: metric.descriptor.name,
            type: metric.descriptor.type,
            unit: metric.descriptor.unit,
            attributeKeys: [...attrKeys].sort(),
          };
        }
      }
    }
  }
  return undefined;
}

// ---------------------------------------------------------------------------
// Span hygiene assertions (from spans.md)
// ---------------------------------------------------------------------------

function assertNoParentlessOutboundSpans() {
  const parentless = getSpans().filter(
    (s) =>
      (s.kind === SpanKind.CLIENT || s.kind === SpanKind.PRODUCER) &&
      !s.parentSpanId,
  );
  if (parentless.length > 0) {
    const names = parentless.map((s) => s.name).join(", ");
    assert.fail(`CLIENT/PRODUCER root spans detected: ${names}`);
  }
}

function assertNoOrphanSpans() {
  const spans = getSpans();
  const spanIds = new Set(spans.map((s) => s.spanContext().spanId));
  const orphans = spans.filter(
    (s) => s.parentSpanId && !spanIds.has(s.parentSpanId),
  );
  if (orphans.length > 0) {
    const names = orphans.map((s) => s.name).join(", ");
    assert.fail(`Orphan spans detected (parent not found): ${names}`);
  }
}

function assertInternalSpanLimit(maxPerTrace = 10) {
  const spans = getSpans();
  const counts = new Map();
  for (const s of spans) {
    if (s.kind === SpanKind.INTERNAL) {
      const traceId = s.spanContext().traceId;
      counts.set(traceId, (counts.get(traceId) ?? 0) + 1);
    }
  }
  for (const [traceId, count] of counts) {
    if (count > maxPerTrace) {
      assert.fail(
        `Trace ${traceId} has ${count} INTERNAL spans (limit: ${maxPerTrace})`,
      );
    }
  }
}

function assertErrorSpansHaveMessages() {
  // Only check application-created spans. Auto-instrumented HTTP SERVER spans
  // set ERROR status on 5xx without a message — we cannot control that.
  const missing = getSpans().filter(
    (s) =>
      s.status.code === 2 /* ERROR */ &&
      !s.status.message?.trim() &&
      s.kind === SpanKind.INTERNAL,
  );
  if (missing.length > 0) {
    const names = missing.map((s) => s.name).join(", ");
    assert.fail(`ERROR spans without a status message: ${names}`);
  }
}

// ---------------------------------------------------------------------------
// Metric hygiene assertions (from metrics.md)
// ---------------------------------------------------------------------------

async function assertAllMetricsHaveUnits() {
  const resourceMetrics = await collectMetrics();
  const missing = [];
  for (const rm of resourceMetrics) {
    for (const sm of rm.scopeMetrics) {
      for (const metric of sm.metrics) {
        if (!metric.descriptor.unit) {
          missing.push(metric.descriptor.name);
        }
      }
    }
  }
  if (missing.length > 0) {
    assert.fail(`Metrics without a unit: ${missing.join(", ")}`);
  }
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

before(async () => {
  await new Promise((resolve) => {
    server = app.listen(0, () => {
      const addr = server.address();
      baseUrl = `http://127.0.0.1:${addr.port}`;
      resolve();
    });
  });
});

afterEach(async () => {
  try {
    // Span hygiene
    assertNoParentlessOutboundSpans();
    assertNoOrphanSpans();
    assertInternalSpanLimit();
    assertErrorSpansHaveMessages();

    // Metric hygiene
    await assertAllMetricsHaveUnits();
  } finally {
    resetSpans();
    resetMetrics();
  }
});

after(async () => {
  await new Promise((resolve) => server.close(resolve));
  await sdk.shutdown();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("HTTP server metrics", () => {
  it("http.server.request.duration exists with expected shape after a request", async () => {
    await request("GET", "/health");
    await collectMetrics();

    const metric = findMetric("http.server.request.duration");
    assert.ok(metric, "http.server.request.duration metric not found");
    assert.equal(metric.unit, "s");
    assert.equal(metric.type, "HISTOGRAM");
    for (const key of [
      "http.request.method",
      "http.route",
      "http.response.status_code",
    ]) {
      assert.ok(
        metric.attributeKeys.includes(key),
        `Missing attribute key: ${key}`,
      );
    }
  });
});

describe("custom business metrics", () => {
  it("orders.processed has the expected shape", async () => {
    await request("POST", "/orders", {
      items: [{ name: "Widget", price: 25 }],
      total: 25,
      currency: "usd",
    });
    await collectMetrics();

    const metric = findMetric("orders.processed");
    assert.ok(metric, "orders.processed metric not found");
    assert.equal(metric.type, "COUNTER");
    assert.equal(metric.unit, "1");
    assert.ok(
      metric.attributeKeys.includes("status"),
      "Missing attribute key: status",
    );
  });

  it("orders.value has the expected shape", async () => {
    await request("POST", "/orders", {
      items: [{ name: "Widget", price: 25 }],
      total: 25,
      currency: "usd",
    });
    await collectMetrics();

    const metric = findMetric("orders.value");
    assert.ok(metric, "orders.value metric not found");
    assert.equal(metric.type, "HISTOGRAM");
    assert.equal(metric.unit, "{USD}");
    assert.ok(
      metric.attributeKeys.includes("currency"),
      "Missing attribute key: currency",
    );
  });
});

describe("span hygiene", () => {
  it("POST /orders produces well-formed spans", async () => {
    await request("POST", "/orders", {
      items: [{ name: "Widget", price: 25 }],
      total: 25,
      currency: "usd",
    });
    // Hygiene assertions run in afterEach
    assert.ok(getSpans().length > 0, "Expected spans to be recorded");
  });

  it("GET /error records ERROR status with a message on application spans", async () => {
    await request("GET", "/error");
    const appErrorSpans = getSpans().filter(
      (s) => s.status.code === 2 && s.kind === SpanKind.INTERNAL,
    );
    assert.ok(appErrorSpans.length > 0, "Expected at least one application ERROR span");
    for (const s of appErrorSpans) {
      assert.ok(
        s.status.message?.trim(),
        `ERROR span "${s.name}" has no status message`,
      );
    }
  });
});

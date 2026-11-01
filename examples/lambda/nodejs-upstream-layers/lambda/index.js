'use strict';

// CommonJS on purpose. Layer-based auto-instrumentation patches `require()`
// directly; ESM handlers (`.mjs`, or `"type": "module"`) depend on the layer's
// ESM loader hook, which is newer and varies by layer and runtime version.
// CommonJS is instrumented identically by both paths, so it keeps the example
// about the layers rather than about module systems.
//
// There is deliberately no OpenTelemetry SDK code in this file: the layer owns
// the whole instrumentation lifecycle.

// Reserved example domain, so the DNS lookup always fails and the resulting
// client span carries an ERROR status.
const UNREACHABLE_URL = 'https://example.test/';
// Reachable target, so there is a successful client span in the same trace.
const REACHABLE_URL = 'https://aws.amazon.com/';

const CPU_ITERATIONS = 2_000_000;
const REQUEST_TIMEOUT_MS = 5_000;

function burnCpu(iterations) {
  let accumulator = 0;
  for (let i = 1; i <= iterations; i += 1) {
    accumulator += Math.sqrt(i) * Math.sin(i);
  }
  return Number(accumulator.toFixed(4));
}

async function callUpstream(url) {
  const startedAt = Date.now();
  try {
    const response = await fetch(url, {
      method: 'GET',
      signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
    });
    await response.arrayBuffer();
    return {
      url,
      ok: true,
      status: response.status,
      durationMs: Date.now() - startedAt,
    };
  } catch (error) {
    // Swallowed on purpose: the failing call is part of the demo payload.
    return {
      url,
      ok: false,
      error: error instanceof Error ? error.message : String(error),
      durationMs: Date.now() - startedAt,
    };
  }
}

exports.handler = async function handler(event) {
  const orderId = `TEST-${String(Math.floor(Math.random() * 100000)).padStart(5, '0')}`;
  console.log(JSON.stringify({ level: 'INFO', message: 'invocation started', orderId }));

  const checksum = burnCpu(CPU_ITERATIONS);
  const failing = await callUpstream(UNREACHABLE_URL);
  const succeeding = await callUpstream(REACHABLE_URL);

  console.log(
    JSON.stringify({
      level: failing.ok ? 'INFO' : 'WARN',
      message: 'upstream calls completed',
      orderId,
      failing,
      succeeding,
    }),
  );

  return {
    statusCode: 200,
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      orderId,
      checksum,
      method: event?.requestContext?.http?.method ?? 'unknown',
      upstreamCalls: [failing, succeeding],
    }),
  };
};

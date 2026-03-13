---
title: 'Next.js Instrumentation'
impact: HIGH
tags:
  - nextjs
  - react
  - fullstack
  - app-router
---

# Next.js Instrumentation

Full-stack OpenTelemetry setup for Next.js 13+ with App Router. Server-side uses Node SDK, client-side uses Dash0 SDK.

## Prerequisites

- Next.js 13+ with App Router
- **OTLP Endpoint**: Your observability platform's HTTP endpoint
  - In Dash0: [Settings → Organization → Endpoints → "OTLP via HTTP"](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Cb29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
  - Format: `https://<region>.your-platform.com`
- **Auth Token**: API token for telemetry ingestion
  - In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)
  - For browser telemetry, use a token with limited permissions (Ingesting only)

## Quick Start

### 1. Install Packages

```bash
npm install @opentelemetry/api \
  @opentelemetry/api-logs \
  @opentelemetry/sdk-node \
  @opentelemetry/sdk-logs \
  @opentelemetry/sdk-metrics \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-http \
  @opentelemetry/exporter-metrics-otlp-http \
  @opentelemetry/exporter-logs-otlp-http \
  @opentelemetry/resources \
  @opentelemetry/semantic-conventions \
  @dash0/sdk-web
```

### 2. Create `.env.local`

```bash
# Server-side (standard OTEL env vars)
OTEL_EXPORTER_OTLP_ENDPOINT=https://<OTLP_ENDPOINT>
OTEL_SERVICE_NAME=my-nextjs-app

# Client-side (NEXT_PUBLIC_ prefix required)
NEXT_PUBLIC_OTEL_ENDPOINT=https://<OTLP_ENDPOINT>
NEXT_PUBLIC_OTEL_AUTH_TOKEN=your-auth-token-here
```

### 3. Create `src/instrumentation.ts` (Server)

```typescript
import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-http';
import { OTLPLogExporter } from '@opentelemetry/exporter-logs-otlp-http';
import { PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';
import {
  BatchLogRecordProcessor,
  LoggerProvider,
} from '@opentelemetry/sdk-logs';
import { logs } from '@opentelemetry/api-logs';
import { resourceFromAttributes } from '@opentelemetry/resources';
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
} from '@opentelemetry/semantic-conventions';

export async function register() {
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    // CRITICAL: Read env vars inside register() to ensure .env.local is loaded
    const OTEL_ENDPOINT =
      process.env.OTEL_EXPORTER_OTLP_ENDPOINT || 'http://localhost:4318';
    const OTEL_AUTH_TOKEN = process.env.NEXT_PUBLIC_OTEL_AUTH_TOKEN;

    console.log('[OTel] Endpoint:', OTEL_ENDPOINT);
    console.log('[OTel] Auth:', OTEL_AUTH_TOKEN ? 'configured' : 'missing');

    const exporterHeaders: Record<string, string> = {};
    if (OTEL_AUTH_TOKEN) {
      exporterHeaders['Authorization'] = `Bearer ${OTEL_AUTH_TOKEN}`;
    }

    const resource = resourceFromAttributes({
      [ATTR_SERVICE_NAME]: process.env.OTEL_SERVICE_NAME || 'nextjs-app',
      [ATTR_SERVICE_VERSION]: '1.0.0',
      'deployment.environment': process.env.NODE_ENV || 'development',
    });

    // Initialize LoggerProvider for structured logging
    const logExporter = new OTLPLogExporter({
      url: `${OTEL_ENDPOINT}/v1/logs`,
      headers: exporterHeaders,
    });

    const loggerProvider = new LoggerProvider({
      resource,
      processors: [new BatchLogRecordProcessor(logExporter)],
    });
    logs.setGlobalLoggerProvider(loggerProvider);

    // Initialize NodeSDK for traces and metrics
    const sdk = new NodeSDK({
      resource,
      traceExporter: new OTLPTraceExporter({
        url: `${OTEL_ENDPOINT}/v1/traces`,
        headers: exporterHeaders,
      }),
      metricReader: new PeriodicExportingMetricReader({
        exporter: new OTLPMetricExporter({
          url: `${OTEL_ENDPOINT}/v1/metrics`,
          headers: exporterHeaders,
        }),
        exportIntervalMillis: 10000,
      }),
      instrumentations: [
        getNodeAutoInstrumentations({
          '@opentelemetry/instrumentation-fs': { enabled: false },
          '@opentelemetry/instrumentation-dns': { enabled: false },
        }),
      ],
    });

    sdk.start();

    // Graceful shutdown — flush all providers before exit
    async function shutdown() {
      await loggerProvider.forceFlush();
      await Promise.allSettled([
        sdk.shutdown(),
        loggerProvider.shutdown(),
      ]);
    }

    function logExceptionAndExit(message: string, error: Error, exitCode: number) {
      logs.getLogger('shutdown').emit({
        severityNumber: 17, // ERROR
        severityText: 'ERROR',
        body: message,
        attributes: {
          'exception.type': error.name,
          'exception.message': error.message,
          'exception.stacktrace': error.stack,
        },
      });
      shutdown().finally(() => process.exit(exitCode));
    }

    process.on('SIGTERM', () => shutdown().finally(() => process.exit(0)));
    process.on('SIGINT', () => shutdown().finally(() => process.exit(0)));

    process.on('uncaughtException', (error) => {
      logExceptionAndExit('uncaught.exception', error, 1);
    });
    process.on('unhandledRejection', (reason) => {
      const error = reason instanceof Error ? reason : new Error(String(reason));
      logExceptionAndExit('unhandled.rejection', error, 1);
    });
  }
}
```

### 4. Create `src/instrumentation-client.ts` (Client)

```typescript
import { init, addSignalAttribute, sendEvent } from '@dash0/sdk-web';

init({
  serviceName: 'my-nextjs-app-frontend',
  endpoint: {
    url: process.env.NEXT_PUBLIC_OTEL_ENDPOINT || 'http://localhost:4318',
    authToken: process.env.NEXT_PUBLIC_OTEL_AUTH_TOKEN || 'dev-token',
  },
  propagateTraceHeadersCorsURLs: [
    /\/api\/.*/, // Match your API routes
  ],
});

// Add default attributes for all telemetry
addSignalAttribute('app.version', '1.0.0');
addSignalAttribute('app.environment', process.env.NODE_ENV || 'development');

export { addSignalAttribute, sendEvent };
```

### 5. Add CORS Headers to `next.config.ts`

```typescript
import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  async headers() {
    return [
      {
        source: '/api/:path*',
        headers: [
          {
            key: 'Access-Control-Allow-Headers',
            value: 'Content-Type, Authorization, traceparent, tracestate',
          },
          {
            key: 'Access-Control-Expose-Headers',
            value: 'X-Trace-Id',
          },
        ],
      },
    ];
  },
};

export default nextConfig;
```

## Resource configuration

Set `service.name`, `service.version`, and `deployment.environment.name` for every deployment.
See [resource attributes](../resources.md) for the full list of required and recommended attributes.

## Common Gotchas

### ENV Vars Must Be Read Inside `register()`

```typescript
// BAD: reads at module load (before .env.local is loaded)
const ENDPOINT = process.env.OTEL_EXPORTER_OTLP_ENDPOINT;

export async function register() {
  // ENDPOINT is undefined here
}

// GOOD: reads inside register()
export async function register() {
  const ENDPOINT = process.env.OTEL_EXPORTER_OTLP_ENDPOINT;
  // ENDPOINT has the correct value
}
```

### Clear `.next` Cache After Changing `NEXT_PUBLIC_*` Vars

```bash
rm -rf .next && npm run dev
```

Client-side env vars are inlined at build time. Changes won't take effect without clearing the cache.

### Package API Notes

- Use `resourceFromAttributes` (not `new Resource()`)
- Use `addSignalAttribute` (not `addAttributes`) for Dash0 SDK

## Custom Telemetry Helper

Create `src/lib/telemetry.ts` for custom spans, metrics, and logs:

```typescript
import { trace, context, SpanStatusCode, metrics } from '@opentelemetry/api';
import { logs, SeverityNumber } from '@opentelemetry/api-logs';

export function getTracer(name = 'my-app') {
  return trace.getTracer(name);
}

export function getMeter(name = 'my-app') {
  return metrics.getMeter(name);
}

export function getLogger(name = 'my-app') {
  return logs.getLogger(name);
}

export function getTraceContext() {
  const span = trace.getSpan(context.active());
  if (!span) return {};
  const ctx = span.spanContext();
  return { traceId: ctx.traceId, spanId: ctx.spanId };
}

// Structured logger with trace correlation
export const logger = {
  info(message: string, attributes: Record<string, unknown> = {}) {
    getLogger().emit({
      severityNumber: SeverityNumber.INFO,
      severityText: 'INFO',
      body: message,
      attributes: { ...getTraceContext(), ...attributes },
    });
  },
  warn(message: string, attributes: Record<string, unknown> = {}) {
    getLogger().emit({
      severityNumber: SeverityNumber.WARN,
      severityText: 'WARN',
      body: message,
      attributes: { ...getTraceContext(), ...attributes },
    });
  },
  error(message: string, attributes: Record<string, unknown> = {}) {
    getLogger().emit({
      severityNumber: SeverityNumber.ERROR,
      severityText: 'ERROR',
      body: message,
      attributes: { ...getTraceContext(), ...attributes },
    });
  },
};

// Wrap async functions with spans
export async function withSpan<T>(
  name: string,
  attributes: Record<string, string | number | boolean>,
  fn: () => Promise<T>,
): Promise<T> {
  return getTracer().startActiveSpan(name, async (span) => {
    try {
      Object.entries(attributes).forEach(([key, value]) => {
        span.setAttribute(key, value);
      });
      return await fn();
    } catch (error) {
      span.setStatus({ code: SpanStatusCode.ERROR, message: (error as Error).message });
      const ctx = span.spanContext();
      logger.error(`${name}.failed`, {
        'trace_id': ctx.traceId,
        'span_id': ctx.spanId,
        'exception.type': (error as Error).name,
        'exception.message': (error as Error).message,
        'exception.stacktrace': (error as Error).stack,
      });
      throw error;
    } finally {
      span.end();
    }
  });
}
```

### Retrieving the active span

Auto-instrumentation creates spans you do not control directly (e.g., the `SERVER` span for an API route).
To enrich these spans with business context or set their status, retrieve the active span from the current context.
See [adding attributes to auto-instrumented spans](../spans.md#adding-attributes-to-auto-instrumented-spans) for when to use this pattern.

The API is the same as Node.js:

```typescript
import { trace } from "@opentelemetry/api";

export async function GET(request: NextRequest) {
  const span = trace.getActiveSpan();
  span?.setAttribute("order.id", request.nextUrl.searchParams.get("id") ?? "");
  span?.setAttribute("tenant.id", request.headers.get("x-tenant-id") ?? "");
  // ... handler logic
}
```

`trace.getActiveSpan()` returns `undefined` if no span is active.
Always use optional chaining (`?.`) when calling methods on the result.

### Span status rules

See [span status code](../spans.md#span-status-code) for the full rules.
This section shows how to apply them in Next.js (same API as Node.js).

#### Always include a status message with `ERROR`

The `message` field on the status object must contain the error class and a short explanation — enough to understand the failure without opening the full trace.

```typescript
// BAD: no status message
span.setStatus({ code: SpanStatusCode.ERROR });

// BAD: generic message with no diagnostic value
span.setStatus({ code: SpanStatusCode.ERROR, message: 'something went wrong' });

// GOOD: specific message with error class and context
span.setStatus({
  code: SpanStatusCode.ERROR,
  message: `TimeoutError: upstream payment service did not respond within 5s`,
});
```

Do not include stack traces in the status message.
Record those in a log record with `exception.stacktrace` instead.

```typescript
// BAD: stack trace in the status message
span.setStatus({ code: SpanStatusCode.ERROR, message: (error as Error).stack });

// GOOD: short message only
span.setStatus({ code: SpanStatusCode.ERROR, message: (error as Error).message });
```

#### Use `OK` only for confirmed success

Set status to `OK` when application logic has explicitly verified the operation succeeded.
Leave status `UNSET` if the code simply did not encounter an error.

## Demo: API Route with Full Telemetry

Create `src/app/api/demo/route.ts`:

```typescript
import { NextRequest, NextResponse } from 'next/server';
import {
  getTracer,
  getMeter,
  logger,
  withSpan,
  getTraceContext,
} from '@/lib/telemetry';
import { SpanStatusCode } from '@opentelemetry/api';

// Create metrics (do once at module level)
const meter = getMeter();
const requestCounter = meter.createCounter('demo.requests');
const requestDuration = meter.createHistogram('demo.request.duration', {
  unit: 'ms',
});

export async function GET(request: NextRequest) {
  const tracer = getTracer();
  const startTime = Date.now();

  return tracer.startActiveSpan('demo.api.get', async (span) => {
    try {
      const itemId = request.nextUrl.searchParams.get('id') || 'default';

      // SPANS: Add context for distributed tracing - helps locate WHERE in the request flow
      span.setAttribute('http.route', '/api/demo');
      span.setAttribute('item.id', itemId);

      // LOGS: Record discrete events with details - helps understand WHY something happened
      // Note: logger auto-injects traceId/spanId via getTraceContext() for span↔log correlation
      logger.info('api.request.received', {
        route: '/api/demo',
        item_id: itemId,
      });

      // METRICS: Aggregate counts for dashboards/alerts - helps detect WHAT is happening
      requestCounter.add(1, { method: 'GET', route: '/api/demo' });

      // Simulate work with custom span
      const result = await withSpan(
        'demo.process',
        { item_id: itemId },
        async () => {
          await new Promise((r) => setTimeout(r, 50));
          return { id: itemId, processed: true };
        },
      );

      requestDuration.record(Date.now() - startTime, {
        method: 'GET',
        status_code: '200',w
      });

      const traceContext = getTraceContext();
      return NextResponse.json(
        {
          success: true,
          data: result,
          _trace: traceContext,
        },
        {
          headers: { 'X-Trace-Id': traceContext.traceId || '' },
        },
      );
    } catch (error) {
      span.setStatus({ code: SpanStatusCode.ERROR, message: (error as Error).message });
      // Record exception as a log record, not a span event — see spans.md#recording-exceptions
      logger.error('api.request.failed', {
        'exception.type': (error as Error).name,
        'exception.message': (error as Error).message,
        'exception.stacktrace': (error as Error).stack,
      });
      return NextResponse.json({ success: false }, { status: 500 });
    } finally {
      span.end();
    }
  });
}
```

## Verification

### Check Console Output

On startup, you should see:

```
[OTel] Endpoint: https://<OTLP_ENDPOINT>
[OTel] Auth: configured
```

### Check Network Tab

Browser DevTools → Network → Filter by "v1/traces" or "v1/logs" to see OTLP exports.

### Check Dash0

Navigate to Dash0 → Explore → filter by `service.name = "my-nextjs-app"`.

## Resources

- [Next.js Instrumentation Docs](https://nextjs.org/docs/app/api-reference/file-conventions/instrumentation)
- [Dash0 SDK Web](https://github.com/dash0hq/dash0-sdk-web)
- [OpenTelemetry Node.js](https://opentelemetry.io/docs/languages/js/getting-started/nodejs/)

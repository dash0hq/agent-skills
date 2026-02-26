---
title: "Browser SDK Setup"
impact: HIGH
tags:
  - browser
  - sdk
  - web
  - frontend
  - react
---

# Browser SDK Setup Reference

## Quick Start with @dash0/sdk-web (Recommended)

The fastest way to instrument a browser application:

```bash
npm install @dash0/sdk-web
```

```typescript
import { init } from "@dash0/sdk-web";

init({
  serviceName: "my-frontend",
  endpoint: {
    url: "https://ingress.eu-west-1.dash0.com:4318",
    authToken: process.env.DASH0_AUTH_TOKEN
  }
});
```

That's it. This automatically includes:
- Document load instrumentation
- Fetch/XHR instrumentation with trace propagation
- Session tracking
- Web Vitals collection

---

## When to Use @dash0/sdk-web vs Manual Setup

| Use @dash0/sdk-web when... | Use manual setup when... |
|---------------------------|-------------------------|
| Starting a new project | Need custom samplers |
| Want sensible defaults | Require specific instrumentations only |
| Using Dash0 as backend | Using multiple backends |
| Prefer minimal config | Need fine-grained control |

---

## Manual Setup (Full Control)

### Official Documentation

- [Browser SDK](https://opentelemetry.io/docs/languages/js/getting-started/browser/)
- [WebTracerProvider](https://open-telemetry.github.io/opentelemetry-js/classes/_opentelemetry_sdk_trace_web.WebTracerProvider.html)

### Package Installation

```bash
npm install @opentelemetry/api \
  @opentelemetry/sdk-trace-web \
  @opentelemetry/sdk-trace-base \
  @opentelemetry/resources \
  @opentelemetry/semantic-conventions \
  @opentelemetry/exporter-trace-otlp-http \
  @opentelemetry/context-zone
```

### Basic Provider Setup

```typescript
// instrumentation.ts - import FIRST in your app entry point
import { WebTracerProvider } from "@opentelemetry/sdk-trace-web";
import { BatchSpanProcessor } from "@opentelemetry/sdk-trace-base";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-http";
import { ZoneContextManager } from "@opentelemetry/context-zone";
import { Resource } from "@opentelemetry/resources";
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT_NAME
} from "@opentelemetry/semantic-conventions";

const resource = new Resource({
  [ATTR_SERVICE_NAME]: "my-frontend",
  [ATTR_SERVICE_VERSION]: process.env.APP_VERSION || "0.0.0",
  [ATTR_DEPLOYMENT_ENVIRONMENT_NAME]: process.env.NODE_ENV || "development",
  "session.id": getOrCreateSessionId()
});

const provider = new WebTracerProvider({ resource });

provider.addSpanProcessor(new BatchSpanProcessor(
  new OTLPTraceExporter({
    url: "https://ingress.eu-west-1.dash0.com:4318/v1/traces",
    headers: { "Authorization": `Bearer ${process.env.DASH0_AUTH_TOKEN}` }
  }),
  { maxQueueSize: 100, maxExportBatchSize: 30, scheduledDelayMillis: 1000 }
));

provider.register({ contextManager: new ZoneContextManager() });

function getOrCreateSessionId(): string {
  let id = sessionStorage.getItem("otel_session_id");
  if (!id) {
    id = crypto.randomUUID();
    sessionStorage.setItem("otel_session_id", id);
  }
  return id;
}
```

### Adding Auto-Instrumentation

```typescript
import { registerInstrumentations } from "@opentelemetry/instrumentation";
import { DocumentLoadInstrumentation } from "@opentelemetry/instrumentation-document-load";
import { FetchInstrumentation } from "@opentelemetry/instrumentation-fetch";
import { UserInteractionInstrumentation } from "@opentelemetry/instrumentation-user-interaction";

registerInstrumentations({
  instrumentations: [
    new DocumentLoadInstrumentation(),
    new FetchInstrumentation({
      propagateTraceHeaderCorsUrls: [/api\.yoursite\.com/],
      ignoreUrls: [/analytics/, /sentry/]
    }),
    new UserInteractionInstrumentation({ eventNames: ["click", "submit"] })
  ]
});
```

---

## Context Managers

### ZoneContextManager (Recommended)

Uses Zone.js for reliable async context propagation.

```typescript
import { ZoneContextManager } from "@opentelemetry/context-zone";

provider.register({ contextManager: new ZoneContextManager() });
```

**Note**: Zone.js adds ~15KB gzipped. Angular apps already include it.

### StackContextManager (Lighter)

Smaller footprint but limited async support.

```typescript
import { StackContextManager } from "@opentelemetry/sdk-trace-web";

provider.register({ contextManager: new StackContextManager() });
```

---

## Sampling

Use parent-based sampling to respect backend decisions:

```typescript
import { ParentBasedSampler, TraceIdRatioBasedSampler } from "@opentelemetry/sdk-trace-base";

const provider = new WebTracerProvider({
  resource,
  sampler: new ParentBasedSampler({
    root: new TraceIdRatioBasedSampler(0.1) // 10% for new traces
  })
});
```

| Environment | Sample Rate | Rationale |
|------------|-------------|-----------|
| Development | 100% | See everything |
| Staging | 50% | Catch issues |
| Production | 1-10% | Cost control |

---

## Resource Attributes

### Essential Attributes

```typescript
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT_NAME,
  ATTR_SERVICE_NAMESPACE
} from "@opentelemetry/semantic-conventions";

const resource = new Resource({
  [ATTR_SERVICE_NAME]: "checkout-ui",           // REQUIRED
  [ATTR_SERVICE_VERSION]: "2.1.0",              // Recommended
  [ATTR_DEPLOYMENT_ENVIRONMENT_NAME]: "production", // Recommended
  [ATTR_SERVICE_NAMESPACE]: "ecommerce",        // Optional - logical grouping
  "session.id": getOrCreateSessionId()          // Browser-specific
});
```

---

## Initialization Timing

**Critical**: Initialize telemetry BEFORE your app code loads.

```typescript
// index.ts - FIRST import
import "./instrumentation";

// Then app code
import { createRoot } from "react-dom/client";
import App from "./App";

createRoot(document.getElementById("root")!).render(<App />);
```

---

## Shutdown Handling

Flush spans on page unload:

```typescript
window.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "hidden") {
    provider.forceFlush();
  }
});
```

---

## Pre-Deployment Checklist

- [ ] SDK init called before app code
- [ ] `serviceName` set correctly (not "unknown_service")
- [ ] Auth token configured and scoped to browser-only permissions
- [ ] CORS configured on backend for `traceparent` header
- [ ] `propagateTraceHeaderCorsUrls` includes your API domains
- [ ] Analytics/third-party URLs in `ignoreUrls`
- [ ] Session tracking verified in backend
- [ ] Test span sent and visible in Dash0

---

## Troubleshooting

| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| No spans in backend | SDK not initialized | Ensure instrumentation.ts is first import |
| Broken traces (browser → server) | Missing trace headers | Add CORS for `traceparent`, configure `propagateTraceHeaderCorsUrls` |
| Missing fetch spans | Wrong URL pattern | Check `propagateTraceHeaderCorsUrls` regex |
| High data volume | Sampling not configured | Add `TraceIdRatioBasedSampler` |
| Context lost in async | Wrong context manager | Use `ZoneContextManager` |
| Spans not exported on navigate | No flush handler | Add `visibilitychange` listener |

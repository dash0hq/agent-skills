---
title: "Browser Auto-Instrumentation"
impact: HIGH
tags:
  - browser
  - auto-instrumentation
  - fetch
  - document-load
  - user-interaction
---

# Browser Auto-Instrumentation Reference

## Quick Start

**Using @dash0/sdk-web?** These instrumentations are included automatically. Skip to [Configuration](#configuration) if you need to customize.

**Using manual setup?** Enable all instrumentations, ignore analytics URLs:

```typescript
import { registerInstrumentations } from "@opentelemetry/instrumentation";
import { DocumentLoadInstrumentation } from "@opentelemetry/instrumentation-document-load";
import { FetchInstrumentation } from "@opentelemetry/instrumentation-fetch";
import { UserInteractionInstrumentation } from "@opentelemetry/instrumentation-user-interaction";

const API_URLS = [/api\.yoursite\.com/];
const IGNORE_URLS = [/analytics/, /sentry/, /hotjar/, /google-analytics/];

registerInstrumentations({
  instrumentations: [
    new DocumentLoadInstrumentation(),
    new FetchInstrumentation({
      propagateTraceHeaderCorsUrls: API_URLS,
      ignoreUrls: IGNORE_URLS
    }),
    new UserInteractionInstrumentation({ eventNames: ["click", "submit"] })
  ]
});
```

---

## Available Instrumentations

| Package | What It Captures | Include? |
|---------|------------------|----------|
| `instrumentation-document-load` | Page load timing, resources | Yes |
| `instrumentation-fetch` | Fetch API requests | Yes |
| `instrumentation-xml-http-request` | XHR calls (legacy) | If using jQuery/XHR |
| `instrumentation-user-interaction` | Clicks, form submissions | Yes |
| `instrumentation-long-task` | Tasks > 50ms | Optional |

---

## Installation

```bash
npm install @opentelemetry/instrumentation \
  @opentelemetry/instrumentation-document-load \
  @opentelemetry/instrumentation-fetch \
  @opentelemetry/instrumentation-user-interaction
```

---

## Configuration

### Fetch Instrumentation (Most Important)

```typescript
new FetchInstrumentation({
  // CRITICAL: Enable trace propagation to your APIs
  propagateTraceHeaderCorsUrls: [
    /https:\/\/api\.yoursite\.com/,
    /https:\/\/.*\.yoursite\.com\/api/
  ],

  // Ignore third-party and analytics URLs
  ignoreUrls: [
    /google-analytics\.com/,
    /sentry\.io/,
    /hotjar\.com/,
    /segment\.com/,
    /\/health$/,
    /\/ping$/
  ]
})
```

**Backend CORS requirement** for trace propagation:
```
Access-Control-Allow-Headers: traceparent, tracestate
```

### Document Load Instrumentation

Defaults are usually fine. Optional customization:

```typescript
new DocumentLoadInstrumentation({
  applyCustomAttributesOnSpan: {
    documentLoad: (span) => {
      span.setAttribute("page.url", window.location.pathname);
    }
  },
  ignoreResourceUrls: [/\.gif$/, /analytics/]
})
```

### User Interaction Instrumentation

Track meaningful interactions only:

```typescript
new UserInteractionInstrumentation({
  eventNames: ["click", "submit"],
  shouldPreventSpanCreation: (eventType, element) => {
    // Only track elements with data-track attribute
    return !element.hasAttribute("data-track");
  }
})
```

```html
<!-- Tracked -->
<button data-track>Submit Order</button>

<!-- Not tracked -->
<button>Cancel</button>
```

---

## Official Documentation

- [Web Instrumentations](https://github.com/open-telemetry/opentelemetry-js-contrib/tree/main/plugins/web)
- [Fetch Instrumentation](https://github.com/open-telemetry/opentelemetry-js-contrib/tree/main/plugins/web/opentelemetry-instrumentation-fetch)

---

## What Auto-Instrumentation Does NOT Capture

| Scenario | Solution |
|----------|----------|
| SPA route changes | Use router hooks (see browser-manual-instrumentation.md) |
| React component renders | React Profiler |
| Custom business events | Manual spans |
| Web Vitals (LCP, CLS, INP) | Use web-vitals library |

---

## Troubleshooting

| Issue | Cause | Fix |
|-------|-------|-----|
| No fetch spans | URL not matching | Check `propagateTraceHeaderCorsUrls` regex |
| Broken traces | Missing CORS headers | Add `traceparent` to allowed headers |
| Too many spans | Tracking all interactions | Use `shouldPreventSpanCreation` filter |
| Missing page load | Init order wrong | Import instrumentation before app code |

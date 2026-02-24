# Browser Auto-Instrumentation Reference

Guide to automatic instrumentation available for browser applications.

## Official Documentation

- [Web Instrumentations](https://github.com/open-telemetry/opentelemetry-js-contrib/tree/main/plugins/web)
- [Document Load](https://github.com/open-telemetry/opentelemetry-js-contrib/tree/main/plugins/web/opentelemetry-instrumentation-document-load)
- [Fetch](https://github.com/open-telemetry/opentelemetry-js-contrib/tree/main/plugins/web/opentelemetry-instrumentation-fetch)
- [User Interaction](https://github.com/open-telemetry/opentelemetry-js-contrib/tree/main/plugins/web/opentelemetry-instrumentation-user-interaction)

## Available Instrumentations

| Package | What It Captures |
|---------|------------------|
| `instrumentation-document-load` | Page load timing, navigation, resources |
| `instrumentation-fetch` | Fetch API requests |
| `instrumentation-xml-http-request` | XMLHttpRequest calls |
| `instrumentation-user-interaction` | Clicks, form submissions |
| `instrumentation-long-task` | Long tasks (>50ms) |

## Installation

```bash
npm install @opentelemetry/instrumentation \
  @opentelemetry/instrumentation-document-load \
  @opentelemetry/instrumentation-fetch \
  @opentelemetry/instrumentation-xml-http-request \
  @opentelemetry/instrumentation-user-interaction
```

## Registration

```typescript
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { XMLHttpRequestInstrumentation } from '@opentelemetry/instrumentation-xml-http-request';
import { UserInteractionInstrumentation } from '@opentelemetry/instrumentation-user-interaction';

registerInstrumentations({
  instrumentations: [
    new DocumentLoadInstrumentation(),
    new FetchInstrumentation(),
    new XMLHttpRequestInstrumentation(),
    new UserInteractionInstrumentation()
  ]
});
```

---

## Document Load Instrumentation

Captures the full page load lifecycle using the Navigation Timing API.

### Spans Created

```
documentLoad ─────────────────────────────────────── total page load
  └─ documentFetch ───────────────────────────────── HTML fetch
  └─ resourceFetch (script.js) ───────────────────── each resource
  └─ resourceFetch (styles.css) ──────────────────── each resource
  └─ resourceFetch (image.png) ───────────────────── each resource
```

### Configuration

```typescript
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';

new DocumentLoadInstrumentation({
  // Add custom attributes to spans
  applyCustomAttributesOnSpan: {
    documentLoad: (span) => {
      span.setAttribute('page.title', document.title);
      span.setAttribute('page.url', window.location.href);
      span.setAttribute('page.referrer', document.referrer);
    },

    documentFetch: (span) => {
      span.setAttribute('document.cache', performance.getEntriesByType('navigation')[0]?.transferSize === 0);
    },

    resourceFetch: (span, resource) => {
      span.setAttribute('resource.initiator', resource.initiatorType);
      span.setAttribute('resource.size', resource.transferSize);
      span.setAttribute('resource.cached', resource.transferSize === 0);
    }
  },

  // Ignore certain resources
  ignoreResourceUrls: [
    /google-analytics\.com/,
    /\.gif$/
  ]
});
```

### Attributes Captured

**documentLoad span:**
- `document.readyState`
- Navigation timing metrics as events

**documentFetch span:**
- `http.url`
- `http.response_content_length`

**resourceFetch span:**
- `http.url`
- `resource.initiatorType` (script, link, img, etc.)

---

## Fetch Instrumentation

Captures all `fetch()` API calls.

### Spans Created

```
HTTP GET https://api.example.com/users ──────────── 150ms
  attributes:
    http.method: GET
    http.url: https://api.example.com/users
    http.status_code: 200
    http.response_content_length: 1234
```

### Configuration

```typescript
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';

new FetchInstrumentation({
  // CRITICAL: Configure which URLs to propagate trace headers to
  propagateTraceHeaderCorsUrls: [
    /https:\/\/api\.yoursite\.com.*/,      // Your API
    /https:\/\/.*\.yoursite\.com\/api.*/,  // Subdomains
    'https://partner-api.com/v1'           // Specific partner
  ],

  // Ignore URLs (no spans created)
  ignoreUrls: [
    /\/health$/,
    /\/ping$/,
    /google-analytics\.com/,
    /sentry\.io/,
    /hotjar\.com/
  ],

  // Clear performance entries after capture (recommended)
  clearTimingResources: true,

  // Add custom attributes
  applyCustomAttributesOnSpan: (span, request, response) => {
    // Add request ID from response header
    const requestId = response.headers.get('x-request-id');
    if (requestId) {
      span.setAttribute('http.request_id', requestId);
    }

    // Add custom header if present
    const customHeader = request.headers?.get('x-custom-header');
    if (customHeader) {
      span.setAttribute('custom.header', customHeader);
    }
  },

  // Ignore specific network errors (optional)
  ignoreNetworkEvents: false
});
```

### Context Propagation

**Critical**: For full-stack tracing, configure `propagateTraceHeaderCorsUrls`:

```typescript
propagateTraceHeaderCorsUrls: [
  /your-api\.com/
]
```

This adds `traceparent` and `tracestate` headers to outgoing requests:

```
traceparent: 00-abc123def456-789xyz-01
tracestate: dash0=...
```

**Backend CORS requirement:**
```
Access-Control-Allow-Headers: traceparent, tracestate
```

### Attributes Captured

| Attribute | Description |
|-----------|-------------|
| `http.method` | GET, POST, etc. |
| `http.url` | Full request URL |
| `http.status_code` | Response status |
| `http.host` | Request host |
| `http.scheme` | http or https |
| `http.response_content_length` | Response size |

---

## XMLHttpRequest Instrumentation

Captures legacy XHR calls. Similar to Fetch instrumentation.

### Configuration

```typescript
import { XMLHttpRequestInstrumentation } from '@opentelemetry/instrumentation-xml-http-request';

new XMLHttpRequestInstrumentation({
  propagateTraceHeaderCorsUrls: [/your-api\.com/],
  ignoreUrls: [/analytics/],
  clearTimingResources: true,
  applyCustomAttributesOnSpan: (span, xhr) => {
    span.setAttribute('xhr.readyState', xhr.readyState);
  }
});
```

### When to Use

- Legacy codebases using `$.ajax()` or direct XHR
- Libraries that use XHR internally
- Usually enable both Fetch and XHR instrumentation

---

## User Interaction Instrumentation

Captures user interactions like clicks, form submissions, and other events.

### Spans Created

```
click ─────────────────────────────────────────────── 5ms
  attributes:
    event_type: click
    target_element: BUTTON
    target_xpath: /html/body/div/button[1]
```

### Configuration

```typescript
import { UserInteractionInstrumentation } from '@opentelemetry/instrumentation-user-interaction';

new UserInteractionInstrumentation({
  // Which events to track
  eventNames: ['click', 'submit', 'change', 'input'],

  // Filter which elements to track
  shouldPreventSpanCreation: (eventType, element, span) => {
    // Don't track password fields
    if (element instanceof HTMLInputElement && element.type === 'password') {
      return true;
    }

    // Only track elements with data-track attribute
    if (!element.hasAttribute('data-track')) {
      return true;
    }

    // Add custom attribute from data attribute
    const trackName = element.getAttribute('data-track-name');
    if (trackName) {
      span.setAttribute('interaction.name', trackName);
    }

    return false; // Don't prevent, create the span
  }
});
```

### Best Practices

**Use data attributes for meaningful tracking:**

```html
<!-- Track this button with a meaningful name -->
<button data-track="true" data-track-name="checkout.submit">
  Complete Purchase
</button>

<!-- Don't track this (no data-track) -->
<button>Cancel</button>
```

**Filter noisy interactions:**

```typescript
shouldPreventSpanCreation: (eventType, element) => {
  // Ignore clicks on document/body
  if (element === document.body || element === document.documentElement) {
    return true;
  }

  // Ignore clicks on navigation links (handled by router)
  if (element.closest('nav')) {
    return true;
  }

  return false;
}
```

### Attributes Captured

| Attribute | Description |
|-----------|-------------|
| `event_type` | click, submit, change, etc. |
| `target_element` | Tag name (BUTTON, INPUT, etc.) |
| `target_xpath` | XPath to element |
| `http.url` | Current page URL |

---

## Long Task Instrumentation

Captures tasks that block the main thread for >50ms (impacts responsiveness).

### Installation

```bash
npm install @opentelemetry/instrumentation-long-task
```

### Configuration

```typescript
import { LongTaskInstrumentation } from '@opentelemetry/instrumentation-long-task';

new LongTaskInstrumentation({
  // Minimum duration to capture (default: 50ms)
  observerCallback: (span, entry) => {
    span.setAttribute('long_task.duration', entry.duration);
    span.setAttribute('long_task.name', entry.name);

    // Warn if very long
    if (entry.duration > 200) {
      span.setAttribute('long_task.severity', 'high');
    }
  }
});
```

### Use Cases

- Identify JavaScript that blocks rendering
- Find performance bottlenecks
- Track impact of third-party scripts

---

## Combined Setup

```typescript
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { DocumentLoadInstrumentation } from '@opentelemetry/instrumentation-document-load';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { XMLHttpRequestInstrumentation } from '@opentelemetry/instrumentation-xml-http-request';
import { UserInteractionInstrumentation } from '@opentelemetry/instrumentation-user-interaction';

const API_PATTERN = /https:\/\/api\.yoursite\.com/;
const IGNORE_URLS = [/analytics/, /sentry/, /hotjar/];

registerInstrumentations({
  instrumentations: [
    // Page load
    new DocumentLoadInstrumentation({
      applyCustomAttributesOnSpan: {
        documentLoad: (span) => {
          span.setAttribute('page.url', window.location.href);
          span.setAttribute('page.title', document.title);
        }
      }
    }),

    // Fetch requests
    new FetchInstrumentation({
      propagateTraceHeaderCorsUrls: [API_PATTERN],
      ignoreUrls: IGNORE_URLS,
      clearTimingResources: true
    }),

    // XHR requests (for legacy code)
    new XMLHttpRequestInstrumentation({
      propagateTraceHeaderCorsUrls: [API_PATTERN],
      ignoreUrls: IGNORE_URLS
    }),

    // User interactions
    new UserInteractionInstrumentation({
      eventNames: ['click', 'submit'],
      shouldPreventSpanCreation: (eventType, element) => {
        return !element.hasAttribute('data-track');
      }
    })
  ]
});
```

---

## What Auto-Instrumentation Does NOT Capture

You'll need manual instrumentation for:

| Scenario | Solution |
|----------|----------|
| Route changes in SPAs | Use router hooks |
| React component renders | Use React Profiler |
| Custom business events | Create manual spans |
| Form validation timing | Create manual spans |
| Animation performance | Use Performance API |
| Web Vitals (LCP, CLS, INP) | Use web-vitals library |

See [manual-instrumentation.md](./manual-instrumentation.md) for these patterns.

---
title: 'Browser Instrumentation'
impact: HIGH
tags:
  - browser
  - web
  - frontend
  - rum
---

# Browser Instrumentation

Instrument web applications to monitor performance, user sessions, requests, and errors.

## Use Cases

- **Real User Monitoring (RUM)**: Capture actual user performance metrics including page load times and resource loading
- **Frontend Performance Analysis**: Identify bottlenecks in network requests, script execution, and rendering
- **Error Tracking**: Capture uncaught JavaScript errors and promise rejections
- **End-to-End Tracing**: Connect frontend interactions with backend traces
- **Custom Instrumentation**: Monitor specific user interactions or business logic workflows

## Option 1: Dash0 SDK Web (Recommended)

100% open source. Designed to work with Dash0. Simplest setup with sensible defaults.

### Prerequisites

1. **OTLP Endpoint**: Your observability platform's HTTP endpoint
   - In Dash0: [Settings → Organization → Endpoints → "OTLP via HTTP"](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Cb29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
   - Format: `https://<region>.your-platform.com`
2. **Auth Token**: API token for telemetry ingestion
   - In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)

**Security**: Browser auth tokens are exposed in client code. Use a dedicated auth token with:

- Limited dataset access
- `Ingesting` permissions only
- No access to sensitive data or admin functions

### Installation

```bash
npm install @dash0/sdk-web
```

### Initialization

Initialize as early as possible in your application:

```javascript
import { init } from '@dash0/sdk-web';

init({
  serviceName: 'my-frontend',
  endpoint: {
    url: 'https://<OTLP_ENDPOINT>',
    authToken: 'YOUR_AUTH_TOKEN',
  },
});
```

For Next.js, use the `instrumentation-client.js` file.

### Features

Auto-instrumentation captures without code modifications:

- Page loads and navigation timing
- Fetch/XHR requests with trace propagation
- User sessions
- JavaScript errors and promise rejections

### Session replay

Session replay is opt-in.
The `init()` call shown above does not start it, because the `@dash0/sdk-web` bundle contains no recorder: the recorder ships as the separate `@dash0/sdk-web/session-recording` entry point.

Session replay records the DOM of every page a visitor sees, so turning it on widens what the site collects about its visitors beyond what browser instrumentation alone collects.
Decide whether to enable it from the request:

| The request | Do this |
| --- | --- |
| Asks for session replay, session recording, or replay by name | Enable it, following the snippet below. |
| Asks for browser instrumentation without mentioning session replay, and the user can be asked | Ask whether to enable session replay, and leave it off until the user answers. |
| Asks for browser instrumentation without mentioning session replay, and the user cannot be asked | Leave it off, and report that it was left off and which snippet below enables it. |

Never add a recorder as an unrequested extra while making an unrelated change to the SDK setup.

Recording starts as soon as a recorder reaches the SDK through any one of these three routes:

1. `sessionRecording.recorder` passed to `init()`.
2. A `startSessionRecording(recorder)` call.
3. A `dash0-session-recording.iife.js` script tag, which sets `window.dash0Recorder`.
   `init()` picks that global up on its own, whatever the order in which the two scripts execute.

To keep session replay off, use none of the three routes.
Route 3 is the one that turns replay on without any `sessionRecording` configuration being visible in the `init()` call, so check the page's script tags, not just the SDK setup code.

To verify which state a site is in, call `setActiveLogLevel('debug')` before `init()` and load the page.
A running recorder logs `Session recording started` to the browser console.
A configured recorder whose session fell outside `samplingRate` logs `Session recording will not start` instead, so treat that message as replay being enabled, not disabled.

To enable session replay, pass the recorder to `init()`:

```javascript
import { init } from '@dash0/sdk-web';
import { recorder } from '@dash0/sdk-web/session-recording';

init({
  serviceName: 'my-frontend',
  endpoint: {
    url: 'https://<OTLP_ENDPOINT>',
    authToken: 'YOUR_AUTH_TOKEN',
  },
  sessionRecording: { recorder },
});
```

Stop a running recording with `stopSessionRecording()`.
Recording does not resume until `startSessionRecording()` is called again.

#### Sampling and masking

The `sessionRecording` settings below take effect only when a recorder is provided.
Set them in the same `init()` call.

| Setting | Default | Change it when |
| --- | --- | --- |
| `samplingRate` | `100` | Recording every session produces more replay data than the dataset should hold. Recorded sessions are always a subset of the sessions that transmit telemetry. |
| `maskAllInputs` | `true` | Never set this to `false` unless no form on the page accepts personal or sensitive data. |
| `maskTextSelector` | unset | The page renders personal data as text rather than in inputs. Use `"*"` to mask all text. |
| `maskTextClass` | `"dash0-mask"` | The page already uses that class name for something else. |
| `blockClass` | `"dash0-block"` | The page already uses that class name for something else. |
| `blockSelector` | unset | Elements must not be recorded at all, including `<input type="hidden">` values, which `maskAllInputs` does not mask because they are element attributes. |
| `recordCanvas` | `false` | The replay is useless without canvas content, and the added payload size is acceptable. |

```javascript
import { init } from '@dash0/sdk-web';
import { recorder } from '@dash0/sdk-web/session-recording';

init({
  serviceName: 'my-frontend',
  endpoint: {
    url: 'https://<OTLP_ENDPOINT>',
    authToken: 'YOUR_AUTH_TOKEN',
  },
  sessionRecording: {
    recorder,
    samplingRate: 10,
    maskTextSelector: '.customer-name, .order-total',
    blockSelector: '[data-pii]',
  },
});
```

### Customization

```javascript
// Add custom attributes, one name and value per call
import { addSignalAttribute } from '@dash0/sdk-web';
addSignalAttribute('user.tier', 'premium');

// Identify users
import { identify } from '@dash0/sdk-web';
identify('user-123');

// Report custom errors
import { reportError } from '@dash0/sdk-web';
reportError(new Error('Custom error'));

// Send custom events, with attributes nested under `attributes`
import { sendEvent } from '@dash0/sdk-web';
sendEvent('checkout.completed', { attributes: { 'order.id': '123' } });
```

`addSignalAttribute` applies to signals transmitted after the call.
To attach attributes to the signals of the initial page load, pass `additionalSignalAttributes` to `init()` instead.

### Resources

- [GitHub Repository](https://github.com/dash0hq/dash0-sdk-web)

## Option 2: OpenTelemetry JS SDK

OpenTelemetry's official SDK for browser instrumentation. Use when you need fine-grained control.

**Note**: Currently experimental.

### Installation

```bash
npm install @opentelemetry/api \
  @opentelemetry/sdk-trace-web \
  @opentelemetry/auto-instrumentations-web \
  @opentelemetry/exporter-trace-otlp-http
```

For bundling (Rollup example):

```bash
npm install --save-dev rollup @rollup/plugin-node-resolve rollup-plugin-commonjs
```

Add to `package.json`:

```json
{ "type": "module" }
```

### Initialization

Create `instrumentation.js`:

```javascript
import {
  WebTracerProvider,
  BatchSpanProcessor,
  AlwaysOnSampler,
} from '@opentelemetry/sdk-trace-web';
import { getWebAutoInstrumentations } from '@opentelemetry/auto-instrumentations-web';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';

const provider = new WebTracerProvider({
  // The browser has no environment variables, so the AlwaysOn sampler must be
  // set in code; see ../spans.md#sampling.
  sampler: new AlwaysOnSampler(),
  spanProcessors: [
    new BatchSpanProcessor(
      new OTLPTraceExporter({
        url: 'https://<OTLP_ENDPOINT>/v1/traces',
        headers: { Authorization: 'Bearer YOUR_AUTH_TOKEN' },
      }),
    ),
  ],
});

provider.register();

registerInstrumentations({
  instrumentations: [getWebAutoInstrumentations()],
});
```

### Bundling

Create `rollup.config.js`:

```javascript
import resolve from '@rollup/plugin-node-resolve';
import commonjs from 'rollup-plugin-commonjs';

export default {
  input: 'src/instrumentation.js',
  output: { file: 'dist/index.js', format: 'iife' },
  plugins: [resolve(), commonjs()],
};
```

Build:

```bash
npm run build
```

### HTML Integration

```html
<script src="dist/index.js"></script>
```

### Testing

Open your website in a browser. Network tab will show calls to the Dash0 ingestion API.

### Resources

- [OpenTelemetry JS Documentation](https://opentelemetry.io/docs/languages/js/getting-started/browser/)

## Browser-to-Server Correlation

To connect frontend traces with backend traces:

### 1. Configure Trace Propagation

```javascript
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';

new FetchInstrumentation({
  propagateTraceHeaderCorsUrls: [/api\.yoursite\.com/],
});
```

### 2. Backend CORS Configuration

```javascript
app.use(
  cors({
    allowedHeaders: [
      'Content-Type',
      'Authorization',
      'traceparent',
      'tracestate',
    ],
  }),
);
```

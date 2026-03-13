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

### Customization

```javascript
// Add custom attributes
import { addAttributes } from '@dash0/sdk-web';
addAttributes({ 'user.tier': 'premium' });

// Identify users
import { setUser } from '@dash0/sdk-web';
setUser({ id: 'user-123' });

// Report custom errors
import { reportError } from '@dash0/sdk-web';
reportError(new Error('Custom error'));

// Send custom events
import { sendEvent } from '@dash0/sdk-web';
sendEvent('checkout.completed', { order_id: '123' });
```

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
} from '@opentelemetry/sdk-trace-web';
import { getWebAutoInstrumentations } from '@opentelemetry/auto-instrumentations-web';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';

const provider = new WebTracerProvider();

provider.addSpanProcessor(
  new BatchSpanProcessor(
    new OTLPTraceExporter({
      url: 'https://<OTLP_ENDPOINT>/v1/traces',
      headers: { Authorization: 'Bearer YOUR_AUTH_TOKEN' },
    }),
  ),
);

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

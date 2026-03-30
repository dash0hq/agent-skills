---
title: 'Sensitive data'
impact: CRITICAL
tags:
  - sensitive-data
  - pii
  - redaction
  - security
  - compliance
---

# Sensitive data

Telemetry attributes, log bodies, and span events can inadvertently capture personally-identifiable information (PII) and other sensitive data.
Once exported, this data is difficult to remove from observability backends and may violate privacy regulations or internal compliance policies.
The rules in this file prevent sensitive data from entering the telemetry pipeline at the source.

## Never-instrument list

Never attach the following categories of data to spans, metrics, or logs — regardless of signal type.

| Category | Examples | Risk |
|----------|----------|------|
| Authentication credentials | Passwords, API keys, bearer tokens, session cookies, OAuth secrets | Credential exposure |
| Financial instruments | Credit card numbers, bank account numbers, CVVs | PCI DSS violation |
| Government identifiers | Social security numbers, passport numbers, tax IDs, national ID numbers | Identity theft, regulatory violation |
| Health records | Diagnoses, prescription data, medical record numbers | HIPAA / health-data regulation violation |
| Biometric data | Fingerprints, facial geometry, retinal scans | Irreversible identity exposure |
| Full authentication headers | `Authorization`, `Cookie`, `Set-Cookie` header values | Credential and session hijacking |

These values must not appear in any telemetry field: span attributes, span status messages, log bodies, log attributes, metric attributes, or resource attributes.

If the user specifically asks to store data belonging to these categories, offer to partly mask or hash it.

## High-risk fields that require evaluation

The following fields are useful for debugging and incident response but carry privacy risk.
Evaluate each against your compliance requirements before including them in telemetry.

| Field | Permitted on | Condition |
|-------|-------------|-----------|
| `user.id` | Spans, logs | Only if the value is an opaque identifier (UUID, internal ID), never a username or email |
| `enduser.id` | Spans, logs | Same as `user.id` — use whichever matches the [Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/) |
| `client.address` / IP address | Spans, logs | Only if required for abuse detection or geo-attribution; truncate or hash when full precision is not needed |
| Email addresses | Never as attributes | If needed for correlation, hash with a keyed function (HMAC) and store the mapping outside the telemetry pipeline |
| Usernames / display names | Never as attributes | Use an opaque `user.id` instead |
| `url.full` | Spans | Strip or redact query parameters that carry tokens, session IDs, or user-supplied input — see [URL sanitization](#url-sanitization) |
| Request and response bodies | Never as attributes | Bodies may contain arbitrary user input; log a content hash or size if body tracking is needed |
| `db.query.text` | Spans | Only if the query is not parameterized; never include literal parameter values — see [database query sanitization](#database-query-sanitization) |

## Sanitization patterns

### URL sanitization

Query parameters frequently carry sensitive tokens, session identifiers, and user-supplied input.
Strip or redact query parameters before attaching URLs to spans.

```javascript
// BAD: full URL with sensitive query parameters
span.setAttribute('http.url', 'https://example.com/callback?token=eyJhbG...&email=user@example.com');

// GOOD: strip query parameters entirely
function sanitizeUrl(url) {
  const parsed = new URL(url);
  parsed.search = '';
  return parsed.toString();
}
span.setAttribute('url.full', sanitizeUrl(req.url));

// GOOD: redact specific sensitive parameters, keep the rest
function redactSensitiveParams(url, sensitiveKeys) {
  const parsed = new URL(url);
  for (const key of sensitiveKeys) {
    if (parsed.searchParams.has(key)) {
      parsed.searchParams.set(key, 'REDACTED');
    }
  }
  return parsed.toString();
}
span.setAttribute('url.full', redactSensitiveParams(req.url, ['token', 'api_key', 'session']));
```

### Path parameterization

URL paths with embedded identifiers cause cardinality explosion and may leak internal IDs.
See [path parameterization](./spans.md#path-parameterization) in the spans rule for replacement patterns.

### Database query sanitization

Database instrumentation libraries may capture full query text, including literal parameter values.
Verify that the instrumentation library uses parameterized queries and does not inline literal values.

```javascript
// BAD: literal values in the query attribute
span.setAttribute('db.query.text', "SELECT * FROM users WHERE email = 'alice@example.com'");

// GOOD: parameterized query — no literal values
span.setAttribute('db.query.text', 'SELECT * FROM users WHERE email = $1');
```

Follow this decision process when the auto-instrumentation library captures unsanitized queries:

1. **Check for a library-level option first.**
   Many database instrumentation libraries expose a configuration to sanitize or disable query capture — for example, `dbStatementSerializer` in `@opentelemetry/instrumentation-pg`, or `enhancedDatabaseReporting: false` in other libraries.
   A library-level setting is the simplest and most efficient fix.
2. **If no library option exists, use a custom `SpanProcessor`** to strip literal values from `db.query.text` before export.
   A regex that replaces quoted strings and numeric literals with placeholders covers most SQL dialects.
3. **If neither option is viable, set up Collector-side redaction** using the transform processor to sanitize `db.query.text` before it reaches the backend.
   See [sensitive data redaction](../../otel-collector/rules/processors.md#sensitive-data-redaction) in the `otel-collector` skill for configuration guidance.
   This is less ideal because the unsanitized query still leaves the application process over the network, but it prevents the data from reaching long-term storage.
4. **As a last resort, disable query capture entirely** by configuring the library to stop setting `db.query.text`.
   This loses query-level observability, so prefer options 1 through 3.

```javascript
// SpanProcessor that sanitizes db.query.text by replacing literal values with '?'
class DbQuerySanitizingSpanProcessor {
  onStart() {}

  onEnd(span) {
    const query = span.attributes['db.query.text'];
    if (typeof query === 'string') {
      span.attributes['db.query.text'] = query
        .replace(/'[^']*'/g, '?')       // single-quoted strings
        .replace(/"[^"]*"/g, '?')       // double-quoted strings
        .replace(/\b\d+(\.\d+)?\b/g, '?'); // numeric literals
    }
  }

  shutdown() { return Promise.resolve(); }
  forceFlush() { return Promise.resolve(); }
}
```

### Hashing for correlation

When a sensitive value is needed for cross-service correlation but must not appear in telemetry, use a keyed hash (HMAC).
This produces a stable, non-reversible identifier that can be joined across services without exposing the original value.

```javascript
import { createHmac } from 'node:crypto';

function hashForTelemetry(value, key) {
  return createHmac('sha256', key).update(value).digest('hex');
}

// Use the hash as the attribute value
span.setAttribute('user.id', hashForTelemetry(user.email, process.env.TELEMETRY_HASH_KEY));
```

Store the mapping between hashes and original values in a separate, access-controlled system — never in the observability backend.

## Structured logging safeguards

Structured logging makes it easy to accidentally attach entire objects to log records.
When logging request or response data, explicitly select the fields to include rather than spreading the entire object.

```javascript
// BAD: spreads the entire request body — may contain passwords, tokens, PII
logger.info('user.signup', { ...req.body, ...getTraceContext() });

// GOOD: explicitly select safe fields
logger.info('user.signup', {
  ...getTraceContext(),
  user_id: req.body.userId,
  plan: req.body.plan,
});
```

Never pass user-controlled objects (request bodies, form data, headers) directly to logging or span attribute calls.

### Log message pattern-based redaction

Even with structured logging, sensitive data can slip into the message string itself — for example, when interpolating user input or when a library logs a raw request.
Apply regex-based redaction as a safety net before emitting the log record.

```javascript
const SENSITIVE_PATTERNS = [
  { pattern: /\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b/g, replacement: '****-****-****-****' },  // credit card numbers
  { pattern: /\b\d{3}-\d{2}-\d{4}\b/g,                     replacement: '***-**-****' },           // SSNs
  { pattern: /\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b/g, replacement: '[EMAIL]' },     // email addresses
  { pattern: /\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g, replacement: '[JWT]' },      // JWT tokens
  { pattern: /\b(sk_|pk_|key_|token_)[a-zA-Z0-9_-]{20,}/g,  replacement: '[API_KEY]' },            // API key prefixes
];

function redactMessage(message) {
  let result = message;
  for (const { pattern, replacement } of SENSITIVE_PATTERNS) {
    result = result.replace(pattern, replacement);
  }
  return result;
}

// Usage in structured logging
logger.info(redactMessage(`Payment processed for ${userEmail}`), {
  ...getTraceContext(),
  payment_method: 'credit_card',
});
```

Pattern-based redaction is not exhaustive — it catches common formats but cannot guarantee coverage of all sensitive data.
Treat it as a defence-in-depth layer alongside the [never-instrument list](#never-instrument-list) and [Collector-side redaction](#defence-in-depth-with-the-collector).

## Redacting auto-instrumented telemetry

Auto-instrumentation libraries create spans and log records that application code does not control directly.
Sensitive data captured by these libraries — such as `url.full` with query parameters, `db.query.text` with literal values, or HTTP request headers — cannot be sanitized at the call site because there is no call site in your code.

Use a custom `SpanProcessor` to intercept and sanitize spans before they are exported.
The `onEnd` method receives every finished span, including those created by auto-instrumentation.

```javascript
import { SpanProcessor } from '@opentelemetry/sdk-trace-base';

const SENSITIVE_ATTRIBUTES = [
  'http.request.header.authorization',
  'http.request.header.cookie',
  'http.response.header.set-cookie',
];

class SensitiveDataRedactingSpanProcessor {
  onStart(_span, _parentContext) {}

  onEnd(span) {
    for (const attr of SENSITIVE_ATTRIBUTES) {
      if (span.attributes[attr] !== undefined) {
        span.attributes[attr] = 'REDACTED';
      }
    }

    // Strip query parameters from url.full
    if (typeof span.attributes['url.full'] === 'string') {
      try {
        const parsed = new URL(span.attributes['url.full']);
        parsed.search = '';
        span.attributes['url.full'] = parsed.toString();
      } catch {
        // Not a valid URL — leave as-is
      }
    }
  }

  shutdown() { return Promise.resolve(); }
  forceFlush() { return Promise.resolve(); }
}
```

Register the processor on the tracer provider **before** the export processor so it runs on every span:

```javascript
import { NodeTracerProvider } from '@opentelemetry/sdk-trace-node';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-grpc';

const provider = new NodeTracerProvider();
provider.addSpanProcessor(new SensitiveDataRedactingSpanProcessor());
provider.addSpanProcessor(new BatchSpanProcessor(new OTLPTraceExporter()));
provider.register();
```

### When to use a span processor vs other approaches

| Approach | Use when |
|----------|----------|
| Instrumentation library configuration | The library exposes an option to disable or sanitize the sensitive field (e.g., `dbStatementSerializer`, `headersToSpanAttributes` allow/deny list). Always prefer this when available. |
| Custom `SpanProcessor` / `LogRecordProcessor` | The library does not expose a configuration option, or you need a blanket policy across all instrumentation libraries. |
| Collector-side redaction ([otel-ottl](../../otel-ottl/SKILL.md#redact-sensitive-data)) | Defence-in-depth safety net. Use in addition to, not instead of, in-process redaction. |

Check the instrumentation library documentation first.
Many libraries support allow/deny lists for captured headers, query sanitization options, or URL filtering.
A library-level configuration is simpler and more efficient than a custom processor.
Fall back to a custom `SpanProcessor` only when no library-level option exists.

## Defence in depth with the Collector

Application-level sanitization is the first line of defence, but it depends on every code path being correct.
Use the OpenTelemetry Collector as a second layer to catch data that slips through.

The [otel-ottl](../../otel-ottl/SKILL.md) skill covers how to write OTTL expressions for redacting sensitive data in the Collector's transform processor — for example, replacing authorization headers, masking credit card patterns in log bodies, or dropping attributes that match a sensitive-data pattern.

Treat Collector-side redaction as a safety net, not a substitute for source-level sanitization.
The Collector processes telemetry after it has been serialized and exported — the data has already left the application process and traversed the network.

## Testing sensitive data protection

Write assertions that verify redaction processors work correctly.
Run these after integration tests that exercise instrumented code paths handling user data.

### Sensitive attributes are redacted

Assert that no exported span contains raw values for known sensitive headers.

```typescript
function assertSensitiveHeadersRedacted() {
  const SENSITIVE_HEADERS = [
    'http.request.header.authorization',
    'http.request.header.cookie',
    'http.response.header.set-cookie',
  ];

  for (const span of getSpans()) {
    for (const header of SENSITIVE_HEADERS) {
      const value = span.attributes[header];
      if (value !== undefined && value !== 'REDACTED') {
        throw new Error(
          `Span "${span.name}" contains unredacted header "${header}"`,
        );
      }
    }
  }
}
```

### User identifiers are hashed

Assert that `user.id` attributes are opaque hashes, not raw email addresses or usernames.

```typescript
function assertUserIdIsHashed() {
  const PII_PATTERNS = [
    /^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$/, // email
    /\s/,                                                   // contains spaces (likely a display name)
  ];

  for (const span of getSpans()) {
    const userId = span.attributes['user.id'];
    if (typeof userId === 'string') {
      for (const pattern of PII_PATTERNS) {
        if (pattern.test(userId)) {
          throw new Error(
            `Span "${span.name}" has user.id matching PII pattern: "${userId}"`,
          );
        }
      }
    }
  }
}
```

## Anti-patterns

### Logging entire request or response objects

```javascript
// BAD: logs everything, including auth headers and body
logger.info('request.received', { headers: req.headers, body: req.body });

// GOOD: log only what you need
logger.info('request.received', {
  ...getTraceContext(),
  method: req.method,
  path: req.path,
  content_length: req.headers['content-length'],
});
```

### Using email or username as `user.id`

```javascript
// BAD: PII as an attribute value
span.setAttribute('user.id', 'alice@example.com');

// GOOD: opaque identifier
span.setAttribute('user.id', 'usr_a1b2c3d4');
```

### Attaching full error messages from user input

```javascript
// BAD: user-supplied input echoed into telemetry
span.setStatus({
  code: SpanStatusCode.ERROR,
  message: `Validation failed: ${req.body.email} is not a valid email`,
});

// GOOD: describe the error without echoing user input
span.setStatus({
  code: SpanStatusCode.ERROR,
  message: 'ValidationError: invalid email format',
});
```

## References

- [OpenTelemetry Specification: Sensitive Data](https://opentelemetry.io/docs/specs/otel/overview/#sensitive-data)
- [Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/)
- [otel-ottl skill — Collector-side redaction](../../otel-ottl/SKILL.md)

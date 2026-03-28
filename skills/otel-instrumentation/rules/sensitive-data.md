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

### URL path parameterization

URL paths that embed entity IDs produce high-cardinality span names and attributes.
Replace dynamic path segments with placeholders before setting `http.route` or `url.full`.

```go
import "regexp"

var (
	reNumericID = regexp.MustCompile(`/\d+`)
	reUUID      = regexp.MustCompile(`/[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)
)

// sanitizeHTTPRoute replaces numeric IDs and UUIDs with low-cardinality placeholders.
func sanitizeHTTPRoute(path string) string {
	path = reUUID.ReplaceAllString(path, "/{uuid}")
	path = reNumericID.ReplaceAllString(path, "/{id}")
	return path
}

// Usage
span.SetAttributes(attribute.String("http.route", sanitizeHTTPRoute(r.URL.Path)))
```

Most HTTP framework instrumentations derive `http.route` from the router pattern automatically (e.g., `/users/:id` in Express, `{id}` in Go chi).
Use path parameterization only when the instrumentation library does not normalize the route — for example, when using `http.ServeMux` in Go before version 1.22 or a custom routing layer.

### Database query sanitization

Database instrumentation libraries may capture full query text, including literal parameter values.
Verify that the instrumentation library uses parameterized queries and does not inline literal values.

```javascript
// BAD: literal values in the query attribute
span.setAttribute('db.query.text', "SELECT * FROM users WHERE email = 'alice@example.com'");

// GOOD: parameterized query — no literal values
span.setAttribute('db.query.text', 'SELECT * FROM users WHERE email = $1');
```

Java — sanitize queries by replacing string literals and numeric values with placeholders:

```java
import java.util.regex.Pattern;

public class DbQuerySanitizer {
    private static final Pattern STRING_LITERAL = Pattern.compile("'[^']*'");
    private static final Pattern NUMERIC_LITERAL = Pattern.compile("\\b\\d+(\\.\\d+)?\\b");

    public static String sanitize(String query) {
        if (query == null) return "unknown";
        String sanitized = STRING_LITERAL.matcher(query).replaceAll("?");
        sanitized = NUMERIC_LITERAL.matcher(sanitized).replaceAll("?");
        return sanitized.length() > 1000 ? sanitized.substring(0, 1000) + "..." : sanitized;
    }
}

// Usage
span.setAttribute("db.query.text", DbQuerySanitizer.sanitize(sql));
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

Python — the same HMAC approach:

```python
import hmac
import hashlib
import os

def hash_for_telemetry(value: str) -> str:
    key = os.environ["TELEMETRY_HASH_KEY"].encode()
    return hmac.new(key, value.encode(), hashlib.sha256).hexdigest()

# Usage
span.set_attribute("user.id", hash_for_telemetry(user.email))
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

Python — equivalent `SpanProcessor`:

```python
from opentelemetry.sdk.trace.export import SpanProcessor
from urllib.parse import urlparse, urlunparse

SENSITIVE_ATTRIBUTES = [
    "http.request.header.authorization",
    "http.request.header.cookie",
    "http.response.header.set-cookie",
]

class SensitiveDataRedactingSpanProcessor(SpanProcessor):
    def on_end(self, span):
        for attr in SENSITIVE_ATTRIBUTES:
            if attr in span.attributes:
                span.attributes[attr] = "REDACTED"

        url = span.attributes.get("url.full")
        if isinstance(url, str):
            try:
                parsed = urlparse(url)
                span.attributes["url.full"] = urlunparse(parsed._replace(query=""))
            except Exception:
                pass
```

Register it before the export processor:

```python
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter

provider = TracerProvider()
provider.add_span_processor(SensitiveDataRedactingSpanProcessor())
provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter()))
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

## Log body sanitization

Log messages assembled from user-controlled data or error details can contain credit card numbers, government identifiers, JWTs, or API keys.
Apply pattern-based redaction before emitting log records.

```javascript
const REDACTION_PATTERNS = [
  // Credit card numbers (Visa, Mastercard, Amex)
  { pattern: /\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b/g, replacement: '****-****-****-****' },
  // US Social Security Numbers
  { pattern: /\b\d{3}-\d{2}-\d{4}\b/g, replacement: '***-**-****' },
  // JWT tokens
  { pattern: /\beyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*/g, replacement: 'JWT_REDACTED' },
  // API keys with common prefixes
  { pattern: /\b(?:sk_|pk_|key_)[a-zA-Z0-9_-]{20,}/g, replacement: 'API_KEY_REDACTED' },
];

function sanitizeLogMessage(message) {
  let sanitized = message;
  for (const { pattern, replacement } of REDACTION_PATTERNS) {
    sanitized = sanitized.replace(pattern, replacement);
  }
  return sanitized;
}
```

Use this function wherever log messages are assembled from dynamic data.
It does not replace structured logging best practices (selecting explicit fields) — it catches PII that leaks through despite explicit field selection, for example when an upstream library logs a full error message containing user input.

## Testing redaction

Write tests that verify sensitive data does not survive the redaction pipeline.
Tests prevent regressions when new attributes are added or instrumentation libraries are updated.

```javascript
import { InMemorySpanExporter, SimpleSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { NodeTracerProvider } from '@opentelemetry/sdk-trace-node';

describe('SensitiveDataRedactingSpanProcessor', () => {
  let exporter;
  let provider;

  beforeEach(() => {
    exporter = new InMemorySpanExporter();
    provider = new NodeTracerProvider();
    provider.addSpanProcessor(new SensitiveDataRedactingSpanProcessor());
    provider.addSpanProcessor(new SimpleSpanProcessor(exporter));
    provider.register();
  });

  it('redacts authorization headers', () => {
    const tracer = provider.getTracer('test');
    const span = tracer.startSpan('test');
    span.setAttribute('http.request.header.authorization', 'Bearer secret-token');
    span.end();

    const [exported] = exporter.getFinishedSpans();
    expect(exported.attributes['http.request.header.authorization']).toBe('REDACTED');
  });

  it('strips query parameters from url.full', () => {
    const tracer = provider.getTracer('test');
    const span = tracer.startSpan('test');
    span.setAttribute('url.full', 'https://example.com/cb?token=secret&ref=home');
    span.end();

    const [exported] = exporter.getFinishedSpans();
    expect(exported.attributes['url.full']).toBe('https://example.com/cb');
  });
});
```

Run these tests in CI alongside application tests.
If a test fails, a sensitive attribute has been introduced that the redaction processor does not cover.

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

## Compliance quick reference

When a compliance framework applies, use this table to determine the required redaction action for each data category.

| Data category | GDPR | PCI DSS | HIPAA |
|---------------|------|---------|-------|
| User identifiers (email, name) | Delete or hash with HMAC | Not required unless tied to cardholder | Delete or hash if part of PHI |
| Credit card numbers (PAN) | Delete if not needed | Mask: show first 6 and last 4 digits only | Not applicable |
| CVV / PIN | Delete | Never store, not even masked | Not applicable |
| Health records / patient IDs | Delete if not needed for purpose | Not applicable | Delete or hash; retain only de-identified data |
| IP addresses | Truncate or delete (considered personal data) | Not required | Anonymize if part of PHI |
| Government identifiers (SSN, passport) | Delete | Not applicable | Delete or hash |

Apply the strictest applicable rule when multiple frameworks overlap.
For example, if both GDPR and PCI DSS apply, delete credit card numbers entirely rather than masking them.

Collector-side enforcement for these rules uses OTTL processors — see the [otel-ottl skill](../../otel-ottl/SKILL.md#redact-sensitive-data) for configuration patterns.

## References

- [OpenTelemetry Specification: Sensitive Data](https://opentelemetry.io/docs/specs/otel/overview/#sensitive-data)
- [Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/)
- [otel-ottl skill — Collector-side redaction](../../otel-ottl/SKILL.md)

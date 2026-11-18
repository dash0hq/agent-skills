---
title: 'Capturing database query parameters'
impact: HIGH
tags:
  - database
  - prepared-statements
  - db-query-parameter
  - sensitive-data
---

# Capturing database query parameters

Prepared-statement parameter values are not captured by default in any OpenTelemetry SDK because they frequently carry sensitive data.
When the values are useful for debugging (e.g., reproducing a failing query, correlating slow queries with specific inputs), enable capture explicitly and, in most cases, only outside production.

The OpenTelemetry semantic convention is `db.query.parameter.<key>` — one attribute per parameter, keyed by 0-based index or the parameter's declared name.
Python is the exception: it emits a single non-standard `db.statement.parameters` attribute. See the [Python SDK rule](./sdks/python.md#database-query-parameters) for the normalisation pattern.

## Before enabling

Apply this checklist before turning capture on, regardless of language:

1. **Confirm none of the parameter values are in the [never-instrument list](./sensitive-data.md#never-instrument-list).**
   Credentials, full PAN/CVV, government IDs, health records, and biometric data must not be captured under any circumstance.
2. **Decide the scope.**
   Prefer enabling capture only in non-production environments, or restrict it to a subset of services that handle non-sensitive data.
3. **Plan defence-in-depth redaction in the Collector.**
   Even with type whitelists, parameter values may carry PII. Configure a transform processor to redact or drop `db.query.parameter.*` for sensitive datasets — see [otel-ottl skill](../../otel-ottl/SKILL.md).
4. **Be aware of side-effects on query sanitization.**
   In Java, enabling capture **disables** the SQL statement sanitizer — literal values that were previously masked in `db.query.text` will be emitted verbatim.

## Per-language activation

The activation mechanism, attribute key shape, and the set of instrumentation libraries that support capture differ by language.
Follow the SDK-specific rule for the language in use:

| Language | Activation | Standard `db.query.parameter.<key>`? | SDK rule |
|---|---|---|---|
| Java (JDBC) | env var | yes (0-based index) | [java](./sdks/java.md#database-query-parameters) |
| Scala (JDBC) | Java agent env var | yes (0-based index) | [scala](./sdks/scala.md#database-query-parameters) |
| .NET SqlClient | env var | yes (name or index) | [dotnet](./sdks/dotnet.md#database-query-parameters) |
| .NET EFCore | env var | yes (EFCore-generated name) | [dotnet](./sdks/dotnet.md#entity-framework-core-any-relational-provider) |
| .NET Npgsql direct | hand-coded enrichment callback | up to the developer | [dotnet](./sdks/dotnet.md#npgsql-directly-npgsqlopentelemetry) |
| Python (psycopg/asyncpg/tortoise) | instrumentor kwarg | **no** — emits `db.statement.parameters` | [python](./sdks/python.md#database-query-parameters) |
| Node.js (pg, mysql2, …) | hand-coded `requestHook` | up to the developer | [nodejs](./sdks/nodejs.md#database-query-parameters) |
| Go (otelsql) | hand-coded `WithAttributesGetter` | up to the developer | [go](./sdks/go.md#database-query-parameters) |
| Go (pgx direct) | `otelpgx.WithIncludeQueryParameters()` | no — library-specific shape | [go](./sdks/go.md#database-query-parameters) |
| Ruby (pg, mysql2, ActiveRecord) | not supported | n/a | [ruby](./sdks/ruby.md#database-query-parameters) |
| PHP (PDO) | not supported | n/a | [php](./sdks/php.md#database-query-parameters) |
| Rust (sqlx, diesel, tokio-postgres) | hand-coded span attribute | up to the developer | [rust](./sdks/rust.md#database-query-parameters) |

## At-a-glance comparison

| | Java/Scala (JDBC) | .NET SqlClient | .NET EFCore | .NET Npgsql direct | Python | Node.js | Go (otelsql) | Go (pgx + otelpgx) | Ruby | PHP | Rust |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Toggle | env var | env var | env var | hand-coded callback | kwarg | hand-coded hook | hand-coded hook | library option | not supported | not supported | hand-coded span attribute |
| Default | off | off | off | n/a | off | n/a | n/a | off | n/a | n/a | n/a |
| Standard key (`db.query.parameter.<key>`) | yes | yes | yes | up to the developer | **no** (`db.statement.parameters`) | up to the developer | up to the developer | no (library shape) | n/a | n/a | up to the developer |
| Key form | 0-based index | name or index | EFCore-generated name | up to the developer | single tuple-string | up to the developer | name or index | library-defined | n/a | n/a | up to the developer |
| Type whitelist | yes | no | no | up to the developer | no | up to the developer | up to the developer | library-defined | n/a | n/a | up to the developer |
| Excluded scenarios | batches | — | — | — | — | — | — | — | n/a | n/a | — |
| Sanitizer side-effect | forces sanitizer off | independent | independent | — | independent | — | — | — | — | — | — |

## References

- [Semantic Conventions: Database Spans](https://opentelemetry.io/docs/specs/semconv/database/database-spans/)
- [Attribute Registry: `db.query.parameter`](https://opentelemetry.io/docs/specs/semconv/registry/attributes/db/#db-query-parameter)
- [Sensitive data rule](./sensitive-data.md) — what must never be captured, and Collector-side redaction patterns

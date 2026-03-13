---
title: 'Metrics'
impact: CRITICAL
tags:
  - metrics
  - cardinality
  - golden-signals
  - naming
  - units
---

# Metrics

Metrics are time-stamped numerical measurements, aggregated over time.
They are the primary signal for alerting, dashboards, and trend analysis.

Before creating a custom metric, check the [semantic conventions for metrics](https://opentelemetry.io/docs/specs/semconv/general/metrics/).
Many common metrics are already defined — for example, HTTP server latency, database connection pools, and runtime memory usage.
Using the semconv metric means auto-instrumentation libraries and dashboards work out of the box.
Only create a custom metric when no semconv metric covers your use case (e.g., domain-specific business metrics like `orders.processed`).

## Metrics from automatic instrumentation

Some auto-instrumentation libraries emit semconv metrics out of the box.
Before creating a custom metric, verify that the metric you need is not already produced by an installed library.
Duplicating an auto-instrumented metric wastes money and resources and creates conflicting data.

### Identifying installed auto-instrumentation

Check which instrumentation libraries are already installed in the project:

| Language | Where to check | Example libraries |
|----------|---------------|-------------------|
| Node.js | `dependencies` in `package.json` | `@opentelemetry/instrumentation-http`, `@opentelemetry/instrumentation-express` |
| Java | Java agent JAR on the JVM command line, or `io.opentelemetry.instrumentation` dependencies in `pom.xml` / `build.gradle` | `opentelemetry-javaagent`, `opentelemetry-spring-boot-starter` |
| Python | `requirements.txt`, `pyproject.toml`, or `setup.cfg` | `opentelemetry-instrumentation-flask`, `opentelemetry-instrumentation-django` |
| Go | `go.mod` imports under `go.opentelemetry.io/contrib/instrumentation/` | `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` |
| .NET | `<PackageReference>` entries in `.csproj` | `OpenTelemetry.Instrumentation.AspNetCore`, `OpenTelemetry.Instrumentation.Http` |
| Ruby | `Gemfile` | `opentelemetry-instrumentation-rack`, `opentelemetry-instrumentation-rails` |
| PHP | `require` entries in `composer.json` | `open-telemetry/opentelemetry-auto-slim-framework` |

### Common auto-instrumented metrics

The table below lists the stable semconv metric names that auto-instrumentation libraries are expected to emit.
If a library from the left column is installed, the metrics in the right column are already being produced — do not recreate them.

| Domain | Instrumentation library (example) | Metrics emitted |
|--------|----------------------------------|-----------------|
| HTTP server | `instrumentation-http`, `AspNetCore`, `otelhttp` | `http.server.request.duration`, `http.server.active_requests` |
| HTTP client | `instrumentation-http`, `HttpClient` | `http.client.request.duration` |
| Database client | `instrumentation-pg`, `instrumentation-mysql2`, `SqlClient` | `db.client.operation.duration` |
| Messaging | `instrumentation-kafkajs`, `instrumentation-amqplib` | `messaging.process.duration`, `messaging.publish.duration` |
| RPC (gRPC) | `instrumentation-grpc` | `rpc.server.duration`, `rpc.client.duration` |
| Runtime | `runtime-node`, `opentelemetry-javaagent`, `Process` | `process.runtime.*.memory`, `process.runtime.*.gc.*` |

Refer to the [semantic conventions metric reference](https://opentelemetry.io/docs/specs/semconv/general/metrics/) and the documentation of each instrumentation library for the full list.

Note: a library may lag behind the specification and emit metrics under outdated names, units, or attributes.
Always [write an integration test](#auto-instrumented-metrics-must-be-tested) for each auto-instrumented metric your service depends on to verify it matches the expected stable semconv shape.

### Decision process

Follow these steps before creating any metric:

1. **List installed instrumentation libraries** using the table above.
2. **Look up the metrics each library emits** in its documentation or the semantic conventions.
3. **Check if the metric you need is already covered.**
   If yes, do not create a custom metric — use the existing one and add attributes only if the semconv allows it.
4. **If no existing metric covers your use case**, create a custom metric following the [naming rules](#naming-rules) and [cardinality guidelines](#cardinality-management) below.

```javascript
// BAD: duplicating an auto-instrumented metric
const requestDuration = meter.createHistogram('http.server.request.duration', { unit: 's' });
// ↑ @opentelemetry/instrumentation-http already emits http.server.request.duration

// GOOD: creating a metric for a domain-specific use case not covered by auto-instrumentation
const orderValue = meter.createHistogram('orders.value', { unit: '{USD}' });
```

## Instrument types

| Type | Use for | Example |
| --- | --- | --- |
| Counter | Monotonic totals that only go up | Requests served, bytes sent, errors |
| UpDownCounter | Totals that can go up and down | Active connections, queue depth, items in cache |
| Histogram | Distributions where percentiles or averages matter | Request latency, response body size, batch processing time |
| Gauge | Point-in-time snapshots of a current value | CPU utilization, memory usage, temperature |

### Choosing the right instrument

Use this decision process:

1. **Are you measuring a duration or a size where you need percentiles (p50, p95, p99)?**
   Use a **Histogram**.
   Histograms capture the full distribution, allowing percentile calculations, averages, and counts to be derived from a single instrument.
   Do not use a Counter to track "total duration" — you lose the distribution.

2. **Are you counting occurrences of something that never decreases (requests, errors, bytes sent)?**
   Use a **Counter**.
   Counters are monotonically increasing.
   The rate of change is derived at query time (e.g., requests per second).

3. **Are you tracking a quantity that can both increase and decrease (active connections, queue size, in-flight requests)?**
   Use an **UpDownCounter**.
   Increment when the resource is acquired and decrement when it is released.

4. **Are you observing a value that has no meaningful sum across instances (CPU utilization, memory usage, temperature)?**
   Use a **Gauge**.
   Gauges report the current value at the time of observation.
   Use the asynchronous (observable) variant when the value is read from an external source rather than updated inline.

### Synchronous vs asynchronous

Each instrument type has a synchronous and an asynchronous (observable) variant.

- **Synchronous** instruments are updated inline in your code at the point where the event happens (e.g., `counter.add(1)` inside a request handler).
- **Asynchronous** instruments register a callback that the SDK invokes at collection time to read the current value (e.g., polling the OS for CPU usage).

Use synchronous instruments when you control the moment of measurement.
Use asynchronous instruments when the value exists independently and you need to sample it periodically (system metrics, connection pool sizes, external gauge readings).

## RED Metrics

The Request/rate, Errors and Duration (RED) metrics can be derived from a small number of semconv metrics:

```javascript
// Latency — use the semconv histogram; traffic (request count) is derived from it
const duration = meter.createHistogram('http.server.request.duration', { unit: 's' });
```

Different types of APIs, e.g. HTTP/REST, RPC, messaging, need to use different metrics.
The semantic conventions for metrics cover most of these cases, but for operations that are not represented in semantic conventions, like headless operations executed e.g. via a cron job, you will have to come up with an appropriate metric name following the naming structure of semantic conventions.

## Creating custom metrics

Do not create custom metrics in semantic convention namespaces that already exist in the [Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/).
The namespace if defined by the first segment of the metric name, e.g., `http` in `http.server.request.duration`.
In case of namespace clash, as the user if they want to prefix the name of the metric with the service name.

### Naming rules

- **Semconv first.**
  Before creating a metric, search the [semantic convention metric definitions](https://opentelemetry.io/docs/specs/semconv/) for your domain (HTTP, database, messaging, runtime, etc.).
  If a semconv metric exists, use it — do not create a duplicate with a different name.
- **Never include the unit in the metric name.**
  The unit is a separate field in the metric model.
  `my_app.request.duration` with unit `s` is correct; `my_app.request.duration.seconds` is wrong.
- **Metric names must not match semantic convention attribute keys.**
  A metric named `http.response.status_code` creates ambiguity with the attribute of the same name.

Examples of metrics from semantic conventions:

```
http.server.request.duration    # Histogram (semconv)
http.server.active_requests     # UpDownCounter (semconv)
system.cpu.utilization          # Gauge (semconv)
```

## Units

Always specify a unit using [UCUM](https://ucum.org/) notation.
Metrics without units are ambiguous and uninterpretable.

| UCUM Unit | Meaning |
|---|---|
| `s` | Seconds |
| `ms` | Milliseconds |
| `By` | Bytes |
| `1` | Dimensionless (ratios, counts) |

- **Use consistent units per metric name.**
  All instances emitting the same metric name must use the same unit.
  Mixing `s` and `ms` for `my-app.request.duration` across services breaks aggregation.
- **Use consistent histogram buckets per metric name.**
  Inconsistent bucket boundaries across instances corrupt quantile calculations when aggregated.

## Cardinality management

**The number 1 cost driver.**
Cardinality is the number of unique time series created by your metrics.
Each new attribute multiplies your current total series count by its number of unique values.
An additional attribute with 10 values turns 12,500 series into 125,000 instantly.
Before adding attributes, calculate:

An example of a metric with 4 attributes:

```
method:    5 values
route:     50 values (normalized)
status:    5 values (bucketed)
instances: 10
```

Total: 5 × 50 × 5 × 10 = <ins>**12,500 series**</ins>

| Series count     | Zone       | Action                                  |
| ---------------- | ---------- | --------------------------------------- |
| < 1,000          | Minimal    | Room to add more dimensions             |
| 1,000 - 10,000   | Ideal      | Good balance of detail vs cost          |
| 10,000 - 50,000  | Acceptable | Monitor growth, review monthly          |
| 50,000 - 100,000 | Caution    | Review attributes, consider sampling    |
| > 100,000        | Danger     | Remove unbounded attributes immediately |
| > 1,000,000      | Critical   | Backend instability, massive costs      |

**Never use on metrics:**

- `user.id`, `request.id`, `order.id`, `account.id`
- `url.full` (has query params)
- `timestamp`, `ip.address`

Each unique value creates a new time series, which can quickly lead to millions of series and skyrocketing costs.

Be very careful when wanting to add `http.path`, as it can contain mutable parts if the path is governed by a route.
Use `http.route` when possible for server-side metrics, or `url.template` for client-side ones.

### Normalization

Normalize high-cardinality values before using them as metric attributes:

```javascript
// URLs: /users/123 → /users/{id}
path.replace(/\/\d+/g, '/{id}');

// Database queries: SELECT * FROM orders WHERE id=99 → SELECT orders
query.replace(/\bWHERE\b.*/i, '').trim();
```

### Attribute placement

Metrics have the lowest cardinality tolerance of all signals.
Only use attributes with a small, bounded set of values.

| Safe for metrics | Avoid on metrics |
|-----------------|-----------------|
| `http.request.method` (9 values) | `user.id` (unbounded) |
| `http.route` (bounded by route table) | `http.path` (potentially unbounded) |
| `http.response.status_code` bucketed to class (`2xx`, `4xx`, `5xx`) | `url.full` (unbounded, contains query params) |

High-cardinality attributes like `user.id` or `order.id` belong on spans and logs, not on metrics.

### Flushing providers on shutdown or crash

OpenTelemetry SDKs batch telemetry before exporting.
If the process exits before the batch is flushed, buffered metrics are lost.
Every application must ensure providers are shut down or flushed before process exit.

Abrupt termination (`SIGKILL`, OOM kill, segfault) bypasses all shutdown hooks — no in-process mitigation exists.

See the `Graceful shutdown` in the language-specific SDK rules for the idiomatic shutdown pattern in each runtime.

## Testing metric data

Treat metric shape — instrument types, units, attribute cardinality, and naming — as a functional requirement.
Use an in-memory metric exporter in integration tests to capture metrics produced by the code under test and assert the rules from this file.

### Test setup

```typescript
import { MeterProvider, InMemoryMetricExporter, PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';

const exporter = new InMemoryMetricExporter();
const reader = new PeriodicExportingMetricReader({
  exporter,
  exportIntervalMillis: 100,
});
const meterProvider = new MeterProvider({ readers: [reader] });

// Force collection and return all accumulated metrics
async function collectMetrics() {
  await reader.forceFlush();
  return exporter.getMetrics();
}

// Call between tests to reset captured metrics
function resetMetrics() {
  exporter.reset();
}
```

### Every metric has a unit

Assert that every metric descriptor includes a non-empty `unit` field.
A missing unit makes the metric ambiguous and uninterpretable — see [units](#units).

```typescript
async function assertAllMetricsHaveUnits() {
  const resourceMetrics = await collectMetrics();
  const missing: string[] = [];
  for (const rm of resourceMetrics) {
    for (const sm of rm.scopeMetrics) {
      for (const metric of sm.metrics) {
        if (!metric.descriptor.unit) {
          missing.push(metric.descriptor.name);
        }
      }
    }
  }
  if (missing.length > 0) {
    throw new Error(`Metrics without a unit: ${missing.join(', ')}`);
  }
}
```

### Metric name does not contain the unit

This rule is not practical to enforce with an automated test.
A suffix check against a fixed list (`.seconds`, `.bytes`, `.ms`, etc.) produces false positives on legitimate names and misses creative misspellings or abbreviations.
Enforce this rule during code review instead: when a new metric is introduced, verify that its name does not embed the unit and that the `unit` field is set correctly — see [naming rules](#naming-rules).

### No high-cardinality attributes

Attribute cardinality on metrics cannot be tested effectively in integration tests.
A test suite exercises a small, fixed set of attribute values, so it will never reveal the combinatorial explosion that occurs in production when an unbounded attribute like `user.id` or `url.full` fans out across millions of unique values.
The only reliable defence is to plan attribute selection correctly at code-review time, before the metric is created.

When adding attributes to a metric, apply the rules in [cardinality management](#cardinality-management):
- Never attach unbounded identifiers (`user.id`, `request.id`, `order.id`, `account.id`, `url.full`, `timestamp`, `ip.address`) to metrics.
- Calculate the total series count (product of all attribute value counts multiplied by the number of instances) and verify it stays within the acceptable zone.
- Use `http.route` or `url.template` instead of raw paths, and bucket status codes by class (`2xx`, `4xx`, `5xx`) instead of using exact codes.

### No duplicate auto-instrumented metrics

This rule is not sensible to enforce with an automated test.
The set of metrics emitted by auto-instrumentation libraries changes across versions and depends on which libraries are installed at runtime — a hardcoded list in a test goes stale quickly and gives false confidence.
Enforce this rule during code review instead: before creating a custom metric, follow the [decision process](#decision-process) to verify that no installed instrumentation library already emits the same metric.

### Metric shape must not change unexpectedly

Treat each metric a service emits as a contract.
Dashboards, alerts, and SLO definitions depend on specific metric names, instrument types, units, and attribute keys.
An unintentional change — renaming a metric, switching from a Counter to a Histogram, adding or removing an attribute key — silently breaks downstream consumers.

Write one test per known metric that asserts its expected shape: name, instrument type, unit, and the exact set of attribute keys.
This makes each metric's contract explicit and produces a clear failure message when the shape drifts.

```typescript
function findMetric(name: string) {
  for (const rm of exporter.getMetrics()) {
    for (const sm of rm.scopeMetrics) {
      for (const metric of sm.metrics) {
        if (metric.descriptor.name === name) {
          const attrKeys = new Set<string>();
          for (const dp of metric.dataPoints) {
            for (const key of Object.keys(dp.attributes)) {
              attrKeys.add(key);
            }
          }
          return {
            name: metric.descriptor.name,
            type: metric.descriptor.type,
            unit: metric.descriptor.unit,
            attributeKeys: [...attrKeys].sort(),
          };
        }
      }
    }
  }
  return undefined;
}

it('orders.value has the expected shape', async () => {
  // Exercise the code path that records the metric
  await placeOrder({ method: 'credit_card', total: 49.99 });
  await reader.forceFlush();

  const metric = findMetric('orders.value');
  expect(metric).toBeDefined();
  expect(metric).toEqual({
    name: 'orders.value',
    type: 'HISTOGRAM',
    unit: '{USD}',
    attributeKeys: ['payment.method'],
  });
});

it('orders.processed has the expected shape', async () => {
  await placeOrder({ method: 'credit_card', total: 49.99 });
  await reader.forceFlush();

  const metric = findMetric('orders.processed');
  expect(metric).toBeDefined();
  expect(metric).toEqual({
    name: 'orders.processed',
    type: 'COUNTER',
    unit: '1',
    attributeKeys: ['order.status'],
  });
});
```

When a test fails, review the change before updating the expectation:
- **Name changed** — verify that dashboards and alerts referencing the old name have been updated.
- **Type changed** — confirm that the new instrument type is correct for the measurement (see [choosing the right instrument](#choosing-the-right-instrument)) and that queries using type-specific functions (e.g., `histogram_quantile`) still work.
- **Unit changed** — ensure all consumers expect the new unit and that the change is not an accidental mixup between `s` and `ms`.
- **Attribute key added** — calculate the cardinality impact (see [cardinality management](#cardinality-management)) before accepting the change.
- **Attribute key removed** — confirm that no dashboard or alert groups or filters by the removed key.

### Auto-instrumented metrics must be tested

Auto-instrumentation libraries do not always emit metrics under the current stable semantic convention names.
A library may lag behind the specification — for example, emitting `http.server.duration` (unit `ms`, old attribute names like `http.method`) instead of the stable `http.server.request.duration` (unit `s`, attributes `http.request.method`, `http.route`, `http.response.status_code`).
Assuming the library is up to date without verifying leads to dashboards and alerts that silently query non-existent metric names.

Write one integration test per auto-instrumented metric that the service depends on.
Assert the expected stable semconv name, instrument type, unit, and attribute keys using the same `findMetric()` helper from [metric shape must not change unexpectedly](#metric-shape-must-not-change-unexpectedly).

```typescript
it('http.server.request.duration has the expected shape', async () => {
  await sendRequest('GET', '/health');
  await reader.forceFlush();

  const metric = findMetric('http.server.request.duration');
  expect(metric).toBeDefined();
  expect(metric).toEqual({
    name: 'http.server.request.duration',
    type: 'HISTOGRAM',
    unit: 's',
    attributeKeys: ['http.request.method', 'http.response.status_code', 'http.route'],
  });
});
```

If the test fails because the library emits an outdated metric name, unit, or attributes:

1. **Create the metric manually** in application code using the correct stable semconv name, unit, and attributes.
   Record it from the same code path that the auto-instrumentation would cover (e.g., an HTTP middleware for `http.server.request.duration`).
2. **Drop the outdated metric in the Collector** using a [`filter` processor](../../otel-collector/rules/processors.md#filter-processor) to prevent it from reaching the backend.
   Do not drop it in the SDK — that requires a custom SDK setup file and couples application code to library internals.
   If the library exposes a configuration option to switch to stable semconv (e.g., `semconvStabilityOptIn`), prefer that — it avoids both the manual metric and the Collector filter.
3. **Document the workaround** — add a comment at the metric creation site explaining why the metric is created manually and which library version caused the mismatch.
   Include a note to remove the manual metric and the Collector filter once the library migrates to stable semantic conventions.
4. **Update the test** to verify the manually created metric instead.
   The test still asserts the stable semconv shape, which is the contract downstream consumers depend on.

When the library eventually upgrades to stable semantic conventions, the test will detect a duplicate metric (both the library and the manual code emit the same name).
At that point, remove the manual metric and the Collector filter, and let auto-instrumentation take over.

### Running the assertions

Call these assertions after each integration test that exercises a code path producing metrics.
If any assertion fails, investigate the metric creation site and fix it according to the rules in this file.

```typescript
afterEach(async () => {
  await assertAllMetricsHaveUnits();
  resetMetrics();
});

afterAll(async () => {
  await meterProvider.shutdown();
});
```

## Anti-patterns

### Unbounded metric labels

```javascript
// BAD: millions of series
counter.add(1, { user_id: userId });

// GOOD: bounded
counter.add(1, { user_tier: 'premium' });
```

## References

- [Metrics](https://opentelemetry.io/docs/concepts/signals/metrics/)
- [Semantic Conventions for Metrics](https://opentelemetry.io/docs/specs/semconv/general/metrics/)
- [UCUM](https://ucum.org/) — unified code for units of measure

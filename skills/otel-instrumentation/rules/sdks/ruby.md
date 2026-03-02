---
title: "Ruby Instrumentation"
impact: HIGH
tags:
  - ruby
  - backend
  - server
---

# Ruby Instrumentation

Instrument Ruby applications to generate traces, logs, and metrics for deep insights into behavior and performance.

## Use cases

- **HTTP Request Monitoring**: Understand outgoing and incoming HTTP requests through traces and metrics, with drill-downs to database level
- **Database Performance**: Observe which database statements execute and measure their duration for optimization
- **Error Detection**: Reveal uncaught errors and the context in which they happened

---

## Installation

```bash
bundle add opentelemetry-sdk opentelemetry-instrumentation-all opentelemetry-exporter-otlp
```

**Note**: Installing the gems alone is insufficient—you must initialize the SDK AND enable exporters.

---

## Environment variables

All environment variables that control the SDK behavior:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_SERVICE_NAME` | Yes | `unknown_service` | Identifies your service in telemetry data |
| `OTEL_TRACES_EXPORTER` | Yes | `none` | **Must set to `otlp`** to export traces |
| `OTEL_METRICS_EXPORTER` | No | `none` | Set to `otlp` to export metrics |
| `OTEL_LOGS_EXPORTER` | No | `none` | Set to `otlp` to export logs |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Yes | `http://localhost:4318` | OTLP collector endpoint |
| `OTEL_EXPORTER_OTLP_HEADERS` | No | - | Headers for authentication (e.g., `Authorization=Bearer TOKEN`) |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | No | `http/protobuf` | Protocol: `grpc`, `http/protobuf`, or `http/json` |
| `OTEL_RESOURCE_ATTRIBUTES` | No | - | Additional resource attributes (e.g., `deployment.environment=production`) |

**Critical**: Without `OTEL_TRACES_EXPORTER=otlp`, the SDK defaults to `none` and no telemetry is exported.

### Where to get configuration values

1. **OTLP Endpoint**: Your observability platform's OTLP endpoint
   - In Dash0: [Settings → Organization → Endpoints](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Cb29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
   - Format: `https://<region>.your-platform.com`
2. **Auth Token**: API token for telemetry ingestion
   - In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)
3. **Service Name**: Choose a descriptive name (e.g., `order-api`, `checkout-service`)

---

## Configuration

### 1. Initialize the SDK

The SDK must be initialized in code on startup, before any application or framework code runs.

**Rails projects** — create `config/initializers/opentelemetry.rb`:

```ruby
require 'opentelemetry/sdk'
require 'opentelemetry/instrumentation/all'
OpenTelemetry::SDK.configure do |c|
  c.use_all()
end
```

**Non-Rails projects** — add to your application entry point before any other requires:

```ruby
require 'opentelemetry/sdk'
require 'opentelemetry/instrumentation/all'
OpenTelemetry::SDK.configure do |c|
  c.use_all()
end
```

### 2. Set service name

```bash
export OTEL_SERVICE_NAME="my-service"
```

### 3. Enable exporters

**This step is required** - without it, no telemetry is sent:

```bash
# Required for traces
export OTEL_TRACES_EXPORTER="otlp"

# Optional: also export metrics and logs
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"
```

### 4. Configure endpoint

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"
```

### 5. Optional: target specific dataset

```bash
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN,Dash0-Dataset=my-dataset"
```

---

## Complete setup

### Using environment variables

```bash
# Service identification
export OTEL_SERVICE_NAME="my-service"

# Enable exporters (required!)
export OTEL_TRACES_EXPORTER="otlp"
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"

# Configure endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"

bundle exec rails server
```

### Using .env file with dotenv

Add the `dotenv` gem and create a `.env` file:

**.env:**
```bash
OTEL_SERVICE_NAME=my-service
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=https://<OTLP_ENDPOINT>
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer YOUR_AUTH_TOKEN
```

**Run with:**
```bash
bundle exec rails server
```

---

## Local development

### Console exporter

For development without a collector, use the console exporter to see telemetry in your terminal:

```bash
export OTEL_SERVICE_NAME="my-service"
export OTEL_TRACES_EXPORTER="console"
export OTEL_METRICS_EXPORTER="console"
export OTEL_LOGS_EXPORTER="console"

bundle exec rails server
```

This prints spans, metrics, and logs directly to stdout—useful for verifying instrumentation works before configuring a remote backend.

### Without a collector

If you set `OTEL_TRACES_EXPORTER=otlp` but have no collector running, you will see connection errors.
This is expected behavior.

**Options:**
1. Use `console` exporter during development (recommended for quick testing)
2. Run a local OpenTelemetry Collector
3. Point directly to your observability backend

---

## Resource configuration

Set `service.name`, `service.version`, and `deployment.environment.name` for every deployment.
See [resource attributes](../resources.md) for the full list of required and recommended attributes.

---

## Kubernetes setup

See [Kubernetes deployment](../platforms/k8s.md) for pod metadata injection, resource attributes, and Dash0 Kubernetes Operator guidance.

---

## Supported libraries

The auto-instrumentation package automatically instruments:

| Category | Libraries |
|----------|-----------|
| HTTP | Rack, Rails, Sinatra, Faraday, Net::HTTP |
| Database | PG, MySQL2, ActiveRecord |
| Cache | Redis, Dalli |
| Messaging | Sidekiq, Resque, Bunny |
| External | RestClient, Ethon, HTTP.rb |
| GraphQL | GraphQL |
| Logging | Logger |

Refer to the [OpenTelemetry Ruby Contrib repository](https://github.com/open-telemetry/opentelemetry-ruby-contrib/tree/main/instrumentation) for the complete list.

---

## Custom spans

Add business context to auto-instrumented traces:

```ruby
tracer = OpenTelemetry.tracer_provider.tracer('my-service')

def process_order(order)
  tracer = OpenTelemetry.tracer_provider.tracer('my-service')
  tracer.in_span('order.process') do |span|
    span.set_attribute('order.id', order.id)
    span.set_attribute('order.total', order.total)
    result = save_order(order)
    result
  rescue StandardError => e
    span.record_exception(e)
    span.status = OpenTelemetry::Trace::Status.error(e.message)
    raise
  end
end
```

---

## Structured logging

Configure your logging framework to serialize exceptions into a single structured field so that stack traces do not break the one-line-per-record contract.
See [logs](../logs.md) for general guidance on structured logging and exception stack traces.

### Semantic Logger

[semantic_logger](https://github.com/reidmorrison/semantic_logger) produces single-line JSON with exceptions serialized into structured fields.

```ruby
require 'semantic_logger'

SemanticLogger.add_appender(io: $stdout, formatter: :json)
logger = SemanticLogger['OrderService']

begin
  process_order(order)
rescue StandardError => e
  logger.error('order.failed', exception: e, order_id: order.id)
end
```

The JSON formatter serializes the exception class, message, and backtrace into structured fields, keeping each log record on a single line.

### Lograge (Rails)

For Rails applications, [lograge](https://github.com/roidrage/lograge) replaces the default multi-line request log with a single-line JSON entry.

```ruby
# config/environments/production.rb
config.lograge.enabled = true
config.lograge.formatter = Lograge::Formatters::Json.new
```

Lograge does not handle exception backtraces directly.
Pair it with semantic_logger or a JSON formatter that serializes exceptions as single-line fields.

---

## Troubleshooting

### No telemetry appearing

**Check exporters are enabled:**
```bash
echo $OTEL_TRACES_EXPORTER  # Should be "otlp" or "console", not empty
```

The SDK defaults `OTEL_TRACES_EXPORTER` to `none`, which silently discards all telemetry.

**Verify SDK is initialized:**
Ensure the `OpenTelemetry::SDK.configure` block runs before your application code.
In Rails, this means placing it in `config/initializers/opentelemetry.rb`.

### Connection errors

This means the SDK is working but cannot reach the collector:
- **No collector running**: Start a local collector or use `OTEL_TRACES_EXPORTER=console`
- **Wrong endpoint**: Check `OTEL_EXPORTER_OTLP_ENDPOINT` is correct
- **Port mismatch**: gRPC uses 4317, HTTP uses 4318

### Instrumentation not picking up libraries

Ensure `c.use_all()` is called inside the `OpenTelemetry::SDK.configure` block.
Verify that the `opentelemetry-instrumentation-all` gem is installed.
Some libraries require their instrumentation gem to be explicitly added to the Gemfile.

### "Exporter is empty" or similar warnings

Usually means `OTEL_TRACES_EXPORTER` (or metrics/logs) is not set.
Set it explicitly:
```bash
export OTEL_TRACES_EXPORTER="otlp"
```

---

## Resources

- [OpenTelemetry Ruby Documentation](https://opentelemetry.io/docs/languages/ruby/)
- [OpenTelemetry Ruby Contrib](https://github.com/open-telemetry/opentelemetry-ruby-contrib)
- [Environment Variable Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)

---
title: "PHP Instrumentation"
impact: HIGH
tags:
  - php
  - backend
  - server
---

# PHP Instrumentation

Instrument PHP applications to generate traces, logs, and metrics for deep insights into behavior and performance.

## Use cases

- **HTTP Request Monitoring**: Understand outgoing and incoming HTTP requests through traces and metrics, with drill-downs to database level
- **Database Performance**: Observe which database statements execute and measure their duration for optimization
- **Error Detection**: Reveal uncaught errors and the context in which they happened

---

## Installation

### Step 1: Install Composer packages

```bash
composer require \
  open-telemetry/sdk \
  open-telemetry/opentelemetry-auto-slim \
  open-telemetry/exporter-otlp \
  open-telemetry/opentelemetry-auto-psr18
```

### Step 2: Install the PHP extension

The `opentelemetry` PHP extension requires `gcc`, `make`, and `autoconf` to compile.

**Linux (APT):**
```bash
sudo apt-get install gcc make autoconf
pecl install opentelemetry
```

**macOS:**
```bash
brew install gcc make autoconf
pecl install opentelemetry
```

After installing, add the extension to your `php.ini`:
```ini
extension=opentelemetry.so
```

### Step 3: Enable auto-loading

Set the `OTEL_PHP_AUTOLOAD_ENABLED` environment variable to `true` so the SDK auto-loads instrumentation at runtime.

**Note**: Installing the packages and extension alone is insufficient—you must enable auto-loading AND configure exporters.

---

## Environment variables

All environment variables that control the SDK behavior:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_PHP_AUTOLOAD_ENABLED` | Yes | `false` | Enables auto-loading of the OpenTelemetry SDK |
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

### 1. Activate the SDK

The SDK activates through the `OTEL_PHP_AUTOLOAD_ENABLED` environment variable:

```bash
export OTEL_PHP_AUTOLOAD_ENABLED=true
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
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"
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
# Enable auto-loading
export OTEL_PHP_AUTOLOAD_ENABLED=true

# Service identification
export OTEL_SERVICE_NAME="my-service"

# Enable exporters (required!)
export OTEL_TRACES_EXPORTER="otlp"
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"

# Configure endpoint
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"

php -S localhost:8000 -t public
```

### Using a .env file

Many PHP frameworks (e.g., Laravel, Symfony) support `.env` files natively.

**.env:**
```bash
OTEL_PHP_AUTOLOAD_ENABLED=true
OTEL_SERVICE_NAME=my-service
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_EXPORTER_OTLP_ENDPOINT=https://<OTLP_ENDPOINT>
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer YOUR_AUTH_TOKEN
```

**Note**: For the built-in PHP development server, you must export environment variables directly or use a wrapper script, as `.env` files are not loaded automatically outside a framework context.

### Inline with the command

```bash
OTEL_PHP_AUTOLOAD_ENABLED=true \
OTEL_SERVICE_NAME="my-service" \
OTEL_TRACES_EXPORTER="otlp" \
OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf" \
OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>" \
OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN" \
php -S localhost:8000 -t public
```

---

## Local development

### Console exporter

For development without a collector, use the console exporter to see telemetry in your terminal:

```bash
export OTEL_PHP_AUTOLOAD_ENABLED=true
export OTEL_SERVICE_NAME="my-service"
export OTEL_TRACES_EXPORTER="console"
export OTEL_METRICS_EXPORTER="console"
export OTEL_LOGS_EXPORTER="console"

php -S localhost:8000 -t public
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

The auto-instrumentation packages automatically instrument:

| Category | Libraries |
|----------|-----------|
| HTTP | cURL, Guzzle, PSR-18 |
| Framework | Laravel, Symfony, Slim |
| Database | PDO |

Refer to [OpenTelemetry documentation](https://opentelemetry.io/ecosystem/registry/?language=php) for the complete list.

---

## Custom spans

Add business context to auto-instrumented traces:

```php
use OpenTelemetry\API\Globals;

$tracer = Globals::tracerProvider()->getTracer('my-service');

function processOrder(array $order): mixed
{
    global $tracer;

    $span = $tracer->spanBuilder('order.process')->startSpan();
    $scope = $span->activate();

    try {
        $span->setAttribute('order.id', $order['id']);
        $span->setAttribute('order.total', $order['total']);
        $result = saveOrder($order);
        return $result;
    } catch (\Throwable $e) {
        $span->recordException($e);
        throw $e;
    } finally {
        $scope->detach();
        $span->end();
    }
}
```

---

## Structured logging

Configure your logging framework to serialize exceptions into a single structured field so that stack traces do not break the one-line-per-record contract.
See [logs](../logs.md) for general guidance on structured logging and exception stack traces.

### Monolog with JSON

[Monolog](https://github.com/Seldaek/monolog) is the standard logging library for Laravel and Symfony.
Use the `JsonFormatter` to produce single-line JSON output.

```php
use Monolog\Logger;
use Monolog\Handler\StreamHandler;
use Monolog\Formatter\JsonFormatter;

$handler = new StreamHandler('php://stdout', Logger::INFO);
$handler->setFormatter(new JsonFormatter());

$logger = new Logger('app');
$logger->pushHandler($handler);

try {
    processOrder($order);
} catch (\Throwable $e) {
    $logger->error('order.failed', [
        'exception' => $e,
        'order_id' => $order['id'],
    ]);
}
```

Monolog's `JsonFormatter` serializes exceptions (including the stack trace) into a structured `context.exception` field as a single-line JSON entry.

---

## Troubleshooting

### No telemetry appearing

**Check exporters are enabled:**
```bash
echo $OTEL_TRACES_EXPORTER  # Should be "otlp" or "console", not empty
```

The SDK defaults `OTEL_TRACES_EXPORTER` to `none`, which silently discards all telemetry.

**Verify auto-loading is enabled:**
```bash
echo $OTEL_PHP_AUTOLOAD_ENABLED  # Should be "true"
```

**Verify the extension is installed:**
```bash
php -m | grep opentelemetry  # Should output "opentelemetry"
```

### Connection errors

This means the SDK is working but cannot reach the collector:
- **No collector running**: Start a local collector or use `OTEL_TRACES_EXPORTER=console`
- **Wrong endpoint**: Check `OTEL_EXPORTER_OTLP_ENDPOINT` is correct
- **Protocol mismatch**: The PHP SDK defaults to `http/protobuf` on port 4318

### Extension not loaded

**Symptom**: No instrumentation happens despite correct environment variables.

**Fix**: Ensure the `opentelemetry` extension is listed in your `php.ini`:
```ini
extension=opentelemetry.so
```

Verify with:
```bash
php -m | grep opentelemetry
```

If the extension does not appear, check that `gcc`, `make`, and `autoconf` were available during the `pecl install` step, and reinstall if necessary.

### Composer dependencies missing

**Symptom**: Errors about missing classes or undefined namespaces.

**Fix**: Ensure all required packages are installed:
```bash
composer require \
  open-telemetry/sdk \
  open-telemetry/opentelemetry-auto-slim \
  open-telemetry/exporter-otlp \
  open-telemetry/opentelemetry-auto-psr18
```

---

## Resources

- [OpenTelemetry PHP Documentation](https://opentelemetry.io/docs/languages/php/getting-started/)
- [OpenTelemetry PHP SDK on Packagist](https://packagist.org/packages/open-telemetry/sdk)
- [Environment Variable Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)

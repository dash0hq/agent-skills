---
title: "Python Instrumentation"
impact: HIGH
tags:
  - python
  - backend
  - server
---

# Python Instrumentation

Instrument Python applications to generate traces, logs, and metrics for deep insights into behavior and performance.

## Use cases

- **HTTP Request Monitoring**: Understand outgoing and incoming HTTP requests through traces and metrics, with drill-downs to database level
- **Database Performance**: Observe which database statements execute and measure their duration for optimization
- **Error Detection**: Reveal uncaught errors and the context in which they happened

## Installation

```bash
pip install opentelemetry-distro opentelemetry-exporter-otlp
opentelemetry-bootstrap -a install
```

The `opentelemetry-distro` package includes dependencies for auto-instrumentation, the OTLP exporter, and the SDK.
The `opentelemetry-bootstrap` command detects installed libraries and installs the corresponding instrumentation packages automatically.

**Note**: Installing the packages alone is insufficient—you must activate the SDK AND configure exporters.

## Environment variables

All environment variables that control the SDK behavior:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTEL_SERVICE_NAME` | Yes | `unknown_service` | Identifies your service in telemetry data |
| `OTEL_TRACES_EXPORTER` | No | `otlp` | Trace exporter (defaults to `otlp`, unlike Node.js) |
| `OTEL_METRICS_EXPORTER` | No | `none` | Set to `otlp` to export metrics |
| `OTEL_LOGS_EXPORTER` | No | `otlp` | Log exporter (defaults to `otlp`, unlike Node.js) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Yes | `http://localhost:4317` | OTLP collector endpoint |
| `OTEL_EXPORTER_OTLP_HEADERS` | No | - | Headers for authentication (e.g., `Authorization=Bearer TOKEN`) |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | No | `grpc` | Protocol: `grpc`, `http/protobuf`, or `http/json` |
| `OTEL_RESOURCE_ATTRIBUTES` | No | - | Additional resource attributes (e.g., `deployment.environment=production`) |

**Note**: Unlike Node.js, the Python SDK defaults `OTEL_TRACES_EXPORTER` and `OTEL_LOGS_EXPORTER` to `otlp`, so traces and logs are exported without explicitly setting these variables.

### Where to get configuration values

1. **OTLP Endpoint**: Your observability platform's OTLP endpoint
   - In Dash0: [Settings → Organization → Endpoints](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Cb29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
   - Format: `https://<region>.your-platform.com`
2. **Auth Token**: API token for telemetry ingestion
   - In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)
3. **Service Name**: Choose a descriptive name (e.g., `order-api`, `checkout-service`)

## Configuration

### 1. Activate the SDK

The SDK is activated by running your application with the `opentelemetry-instrument` command:

```bash
opentelemetry-instrument python main.py
```

**Flask:**
```bash
opentelemetry-instrument flask --app myapp run
```

**Django:**
```bash
opentelemetry-instrument python manage.py runserver
```

### 2. Set service name

```bash
export OTEL_SERVICE_NAME="my-service"
```

### 3. Enable exporters

Traces and logs default to `otlp`, so you only need to enable metrics explicitly:

```bash
# Optional: also export metrics
export OTEL_METRICS_EXPORTER="otlp"
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

## Complete setup

### Using environment variables

```bash
# Service identification
export OTEL_SERVICE_NAME="my-service"

# Enable metrics exporter (traces and logs default to otlp)
export OTEL_METRICS_EXPORTER="otlp"

# Configure endpoint
export OTEL_EXPORTER_OTLP_ENDPOINT="https://<OTLP_ENDPOINT>"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"

# Run with auto-instrumentation
opentelemetry-instrument python main.py
```

### Using .env file

Python does not automatically load `.env` files.
Use a library like [python-dotenv](https://pypi.org/project/python-dotenv/) or export variables in your shell before running:

**.env:**
```bash
OTEL_SERVICE_NAME=my-service
OTEL_METRICS_EXPORTER=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=https://<OTLP_ENDPOINT>
OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer YOUR_AUTH_TOKEN
```

**Run with:**
```bash
# Using shell export
set -a && source .env && set +a
opentelemetry-instrument python main.py
```

### Using a Makefile

Add instrumented targets to your `Makefile`:

```makefile
run:
	python main.py

run-otel:
	set -a && source .env && set +a && opentelemetry-instrument python main.py

run-otel-console:
	OTEL_SERVICE_NAME=my-service OTEL_TRACES_EXPORTER=console OTEL_LOGS_EXPORTER=console opentelemetry-instrument python main.py
```

**Usage:**
```bash
make run-otel          # Run with OTLP export to backend
make run-otel-console  # Run with console output (no collector needed)
```

## Local development

### Console exporter

For development without a collector, use the console exporter to see telemetry in your terminal:

```bash
export OTEL_SERVICE_NAME="my-service"
export OTEL_TRACES_EXPORTER="console"
export OTEL_METRICS_EXPORTER="console"
export OTEL_LOGS_EXPORTER="console"

opentelemetry-instrument python main.py
```

This prints spans, metrics, and logs directly to stdout—useful for verifying instrumentation works before configuring a remote backend.

### Without a collector

If you use the default `otlp` exporter but have no collector running, you'll see connection errors. This is expected behavior:

```
Failed to export batch. UNAVAILABLE: failed to connect to all addresses
```

**Options:**
1. Use `console` exporter during development (recommended for quick testing)
2. Run a local OpenTelemetry Collector
3. Point directly to your observability backend

## Resource configuration

Set `service.name`, `service.version`, and `deployment.environment.name` for every deployment.
See [resource attributes](../resources.md) for the full list of required and recommended attributes.

## Kubernetes setup

See [Kubernetes deployment](../platforms/k8s.md) for pod metadata injection, resource attributes, and Dash0 Kubernetes Operator guidance.

## Supported libraries

The auto-instrumentation packages automatically instrument:

| Category | Libraries |
|----------|-----------|
| HTTP | Flask, Django, FastAPI, requests, urllib3, aiohttp |
| Database | psycopg2, mysql-connector, pymongo, redis |
| ORM | SQLAlchemy |
| Messaging | Celery, pika (RabbitMQ), confluent-kafka |
| AWS | boto3, botocore |
| Logging | logging (stdlib) |
| gRPC | grpcio |
| AI/LLM | [OpenLLMetry](https://github.com/traceloop/openllmetry), [OpenLit](https://github.com/openlit/openlit) (generative AI, Langchain observability) |

Refer to [OpenTelemetry documentation](https://opentelemetry.io/ecosystem/registry/?language=python) for the complete list.

## Custom spans

Add business context to auto-instrumented traces:

```python
from opentelemetry import trace
from opentelemetry.trace import StatusCode

tracer = trace.get_tracer("my-service")

def process_order(order):
    with tracer.start_as_current_span("order.process") as span:
        try:
            span.set_attribute("order.id", order["id"])
            span.set_attribute("order.total", order["total"])
            result = save_order(order)
            return result
        except Exception as error:
            span.set_status(StatusCode.ERROR, str(error))
            ctx = span.get_span_context()
            logger.error("order.process.failed", extra={
                "trace_id": format(ctx.trace_id, "032x"),
                "span_id": format(ctx.span_id, "016x"),
                "exception.type": type(error).__name__,
                "exception.message": str(error),
                "exception.stacktrace": traceback.format_exc(),
            })
            raise
```

### Retrieving the active span

Auto-instrumentation creates spans you do not control directly (e.g., the `SERVER` span for an HTTP request).
To enrich these spans with business context or set their status, retrieve the active span from the current context.
See [adding attributes to auto-instrumented spans](../spans.md#adding-attributes-to-auto-instrumented-spans) for when to use this pattern.

```python
from opentelemetry import trace

@app.route("/api/orders", methods=["POST"])
def create_order():
    span = trace.get_current_span()
    span.set_attribute("order.id", request.json["order_id"])
    span.set_attribute("tenant.id", request.headers.get("X-Tenant-Id"))
    # ... handler logic
```

`trace.get_current_span()` returns a non-recording span if no span is active.
Calling `set_attribute` or `set_status` on a non-recording span is a no-op, so no guard is needed.

### Span status rules

See [span status code](../spans.md#span-status-code) for the full rules.
This section shows how to apply them in Python.

#### Always include a status message with `ERROR`

The second argument to `set_status` is the status message.
It must contain the error type and a short explanation — enough to understand the failure without opening the full trace.

```python
from opentelemetry.trace import StatusCode

# BAD: no status message
span.set_status(StatusCode.ERROR)

# BAD: generic message with no diagnostic value
span.set_status(StatusCode.ERROR, "something went wrong")

# GOOD: specific message with error type and context
span.set_status(StatusCode.ERROR, f"TimeoutError: upstream payment service did not respond within 5s")
```

Do not include tracebacks in the status message.
Record those in a log record with `exception.stacktrace` instead.

```python
import traceback

# BAD: traceback in the status message
span.set_status(StatusCode.ERROR, traceback.format_exc())

# GOOD: short message only
span.set_status(StatusCode.ERROR, str(error))
```

#### Use `OK` only for confirmed success

Set status to `OK` when application logic has explicitly verified the operation succeeded.
Leave status `UNSET` if the code simply did not encounter an error.

```python
# GOOD: explicit confirmation from downstream
response = requests.get(url)
if response.ok:
    span.set_status(StatusCode.OK)

# BAD: setting OK speculatively
span.set_status(StatusCode.OK)
return some_function()  # might still fail after this point
```

## Structured logging

Configure your logging framework to serialize exceptions into a single structured field so that stack traces do not break the one-line-per-record contract.
See [logs](../logs.md) for general guidance on structured logging and exception stack traces.

### python-json-logger

The standard `logging` module prints multi-line stack traces by default.
Use [python-json-logger](https://pypi.org/project/python-json-logger/) to output single-line JSON with the stack trace captured in a structured field.

```python
import logging
from pythonjsonlogger.json import JsonFormatter

handler = logging.StreamHandler()
handler.setFormatter(JsonFormatter(
    fmt="%(asctime)s %(levelname)s %(name)s %(message)s",
    rename_fields={"levelname": "level", "asctime": "timestamp"},
))

logger = logging.getLogger(__name__)
logger.addHandler(handler)
logger.setLevel(logging.INFO)

try:
    process_order(order)
except Exception:
    logger.exception("order.failed", extra={"order_id": order_id})
```

`logger.exception()` automatically captures the stack trace.
The JSON formatter serializes it into an `exc_info` field as a single escaped string, keeping the log record on one line.

### structlog

[structlog](https://www.structlog.org/) produces single-line JSON output by default when configured with its JSON renderer.

```python
import structlog

structlog.configure(
    processors=[
        structlog.processors.add_log_level,
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.format_exc_info,
        structlog.processors.JSONRenderer(),
    ],
)

logger = structlog.get_logger()

try:
    process_order(order)
except Exception:
    logger.exception("order.failed", order_id=order_id)
```

The `format_exc_info` processor converts the stack trace into a single string field before JSON serialization.

## Graceful shutdown

The `opentelemetry-instrument` command registers an `atexit` hook automatically.
When the process exits normally (including unhandled exceptions in most WSGI/ASGI servers), the hook flushes all pending spans, metrics, and log records before termination.
No additional code is needed for the auto-instrumented setup.

If you use a programmatic SDK setup (without `opentelemetry-instrument`), register a shutdown hook manually:

```python
import atexit

atexit.register(tracer_provider.shutdown)
atexit.register(meter_provider.shutdown)
atexit.register(logger_provider.shutdown)
```

`shutdown()` flushes pending batches and releases resources.
The call blocks until export completes or the timeout expires (default 30 seconds).

## Troubleshooting

### No telemetry appearing

**Check the exporter configuration:**
```bash
echo $OTEL_TRACES_EXPORTER  # Defaults to "otlp"; check it is not set to "none"
```

**Verify bootstrap installed instrumentations:**
```bash
opentelemetry-bootstrap -a requirements
```

This lists the instrumentation packages that should be installed based on your project's dependencies.
If packages are missing, re-run `opentelemetry-bootstrap -a install`.

**Verify SDK is active:**
Ensure you are running your application with the `opentelemetry-instrument` command, not just `python`.

### Connection errors

```
Failed to export batch. UNAVAILABLE: failed to connect to all addresses
```

This means the SDK is working but cannot reach the collector:
- **No collector running**: Start a local collector or use `OTEL_TRACES_EXPORTER=console`
- **Wrong endpoint**: Check `OTEL_EXPORTER_OTLP_ENDPOINT` is correct
- **Port mismatch**: gRPC uses 4317, HTTP uses 4318

### Debug logging

Enable debug-level logging to see detailed SDK output:

```bash
export OTEL_LOG_LEVEL="debug"
opentelemetry-instrument python main.py
```

This reveals exporter activity, span creation, and configuration issues.

### Missing instrumentations

**Symptom**: SDK loads but specific libraries are not instrumented

**Fix**: Run the bootstrap command to detect and install missing instrumentation packages:
```bash
opentelemetry-bootstrap -a install
```

Ensure the bootstrap command runs in the same virtual environment as your application.

## Resources

- [OpenTelemetry Python Documentation](https://opentelemetry.io/docs/languages/python/getting-started/)
- [Auto-Instrumentation Package](https://pypi.org/project/opentelemetry-distro/)
- [Environment Variable Specification](https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/)
- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)

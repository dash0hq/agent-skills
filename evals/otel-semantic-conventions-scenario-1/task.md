# Modernize Payment Gateway Telemetry

## Problem/Feature Description

Finpay is a B2B payments platform that processes transactions on behalf of thousands of merchants. Their Python service was instrumented with OpenTelemetry approximately two years ago, and it has been running fine since then — but their observability tooling has recently started flagging schema inconsistencies in dashboards and queries. A platform engineer has traced the root cause to the telemetry instrumentation: the attribute names and metric names in use are now outdated and no longer match what their observability backend expects.

The team needs the instrumentation updated to use current OpenTelemetry semantic convention attribute names, correct metric names, and proper metric units. The service handles both inbound payment requests (acting as an HTTP server) and outbound calls to card network APIs (acting as an HTTP client), so both directions of HTTP span instrumentation must be addressed. Some edge-case payment methods (like "SEPA_DEBIT" or "WIRE_TRANSFER") are not standard HTTP verbs, and the service needs to handle these gracefully when they appear in HTTP method-like classification attributes.

## Output Specification

Produce an updated version of the instrumentation module at `output/instrumentation.py`. The updated module should contain the same structure as the input file but with all attribute and metric names updated to their current equivalents.

Also produce a short migration notes file at `output/migration_notes.md` listing what was changed and why (keep it brief — a bullet per change is enough).

## Input Files

The following files are provided as inputs. Extract them before beginning.

=============== FILE: inputs/instrumentation.py ===============
"""
Finpay payment gateway telemetry instrumentation.
Initialises OpenTelemetry tracing and metrics for the payment processing service.
"""
import time
from opentelemetry import trace, metrics
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.metrics import MeterProvider

tracer = trace.get_tracer(__name__)
meter = metrics.get_meter(__name__)

# Metric: track how long inbound payment requests take
server_duration = meter.create_histogram(
    name="http.server.duration",
    description="Duration of inbound HTTP requests",
    unit="ms",
)

# Metric: track how long outbound card-network calls take
client_duration = meter.create_histogram(
    name="http.client.duration",
    description="Duration of outbound HTTP calls to card networks",
    unit="ms",
)


def record_inbound_request(method, path, query, scheme, status_code, user_agent, duration_ms):
    """Create a SERVER span for an inbound payment request."""
    with tracer.start_as_current_span("payment.inbound") as span:
        span.set_attribute("http.method", method)
        span.set_attribute("http.target", f"{path}?{query}" if query else path)
        span.set_attribute("http.scheme", scheme)
        span.set_attribute("http.status_code", status_code)
        span.set_attribute("http.user_agent", user_agent)

        server_duration.record(
            duration_ms,
            {
                "http.method": method,
                "http.status_code": status_code,
                "http.route": "/v1/payments/{payment_id}",
            }
        )


def record_outbound_call(method, full_url, peer_host, peer_port, status_code, http_version, duration_ms):
    """Create a CLIENT span for an outbound call to a card network API."""
    with tracer.start_as_current_span("cardnetwork.call") as span:
        span.set_attribute("http.method", method)
        span.set_attribute("http.url", full_url)
        span.set_attribute("net.peer.name", peer_host)
        span.set_attribute("net.peer.port", peer_port)
        span.set_attribute("http.status_code", status_code)
        span.set_attribute("http.flavor", http_version)

        client_duration.record(
            duration_ms,
            {
                "http.method": method,
                "http.status_code": status_code,
            }
        )


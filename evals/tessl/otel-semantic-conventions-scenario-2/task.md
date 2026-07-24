# Add Observability to the Order Management Service

## Problem/Feature Description

Threadline is a B2B e-commerce platform that connects clothing manufacturers with wholesale buyers. Their Python order management service handles order creation, status updates, and fulfillment routing. The service is multi-tenant — each request belongs to a specific merchant account, and orders have varying priority levels (standard, express, white-glove) that influence routing.

The platform team wants to add OpenTelemetry instrumentation to this service so they can trace individual order operations and measure aggregate throughput and latency. As they grow, they expect tens of millions of orders per month across thousands of merchant accounts. The observability platform they use requires well-structured telemetry to keep storage costs under control — they have already had one incident where unbounded label cardinality caused a metrics backend to fall over, and they are keen to avoid a repeat.

The service exposes a REST API over HTTP. Each API call is associated with an authenticated merchant (by merchant ID) and, after the order is created, with a specific order ID. Orders have a business-specific priority level that the operations team wants to see in telemetry for routing analysis.

## Output Specification

Produce a Python module at `output/instrumentation.py` that demonstrates the telemetry setup for this service. It should include:
- Resource configuration (setting service identity and environment)
- A server span for a typical order creation request, including the merchant ID, order ID, and order priority as attributes
- A histogram metric for tracking order creation latency, with appropriate dimensions
- Wherever you create a custom attribute name for a Threadline-specific concept, use the company domain `threadline.io`

Also produce a brief `output/attribute_decisions.md` explaining the key attribute placement decisions you made — one or two sentences per decision is enough.


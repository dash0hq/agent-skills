# Bootstrap Observability for Inventory Microservice

## Problem/Feature Description

Stackmart is a retail technology company running a microservices platform on Kubernetes. Their inventory service is a new Python service that is about to go into production. It exposes a REST API consumed by two other internal services: the order service and the warehouse management service. It also makes outbound calls to a third-party supplier catalog API to fetch product availability data.

The infrastructure team is standardising how all new services are instrumented before they ship to production. They use Dash0 as their observability backend and have noticed that a previous service set up its telemetry incorrectly — the service dependency topology graph was broken because client spans lacked the attributes needed to draw edges to downstream services, and some Kubernetes-specific metadata that ended up on every span caused unexpectedly high data volumes and broke cross-service correlation.

The team wants the new inventory service to be set up correctly from the start, with all the resource attributes Dash0 needs, proper span configuration for both incoming and outgoing calls, and nothing that would interfere with Dash0's own enrichment pipeline.

The service runs in a Kubernetes pod. The pod UID is available at runtime via the Downward API (assume it is available as the environment variable `K8S_POD_UID`). The pod name is available as `K8S_POD_NAME`. The service version is `1.0.0` and the deployment environment is `production`.

## Output Specification

Produce a Python module at `output/otel_setup.py` that:
- Initialises the OpenTelemetry SDK with a correctly configured Resource
- Shows a representative SERVER span for an inbound inventory lookup request
- Shows a representative CLIENT span for an outbound call to the supplier catalog API at `catalog.supplier.example.com`
- Includes inline comments where a choice or placement decision needs explanation for a future reader

Also produce `output/setup_notes.md` with a brief explanation of any non-obvious decisions made in the setup.


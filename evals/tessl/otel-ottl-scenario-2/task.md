# Taming Metric Explosion: Cardinality Reduction for a REST API Fleet

## Problem/Feature Description

Platform engineering team at ShopStream runs dozens of microservices behind a REST API gateway. Their observability costs have tripled over the past quarter because spans emitted by the services include raw customer order IDs, user UUIDs, and product SKU numbers directly in the `url.path` and `http.route` attributes — for example paths like `/orders/4b2e1a90-3c4d-4e5f-8a1b-2c3d4e5f6a7b/items/12345`. Each unique path segment creates a distinct time series, overwhelming their backend's capacity.

A secondary problem: some services emit spans with hundreds of attributes and string values that can be several kilobytes each, ballooning per-span storage. The team also wants to ensure that every span carries a `deployment.environment.name` attribute at the span level (not just at the resource level) so their dashboards can filter by environment without resource-level joins.

The team needs a Collector configuration that normalizes paths, caps attribute counts, truncates long values, correctly adds static environment metadata, and propagates that metadata to individual spans.

## Output Specification

Produce a complete OpenTelemetry Collector configuration file named `collector-config.yaml` that covers:

- Processor(s) for normalizing path attributes on spans
- Processor(s) for limiting attribute count and truncating long attribute values
- Processor(s) for setting static environment and cluster metadata
- Processor(s) for propagating environment metadata from resource to span level
- A `service` section wiring a `traces` pipeline from an `otlp` receiver through the processors to a `debug` exporter

Also produce a `design-notes.md` file that briefly explains the processor choices made for each requirement (which processor type was used and why).


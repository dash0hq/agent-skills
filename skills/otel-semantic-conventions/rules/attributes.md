---
title: "Attributes: Registry, Selection, and Placement"
impact: CRITICAL
tags:
  - attributes
  - registry
  - semconv
  - http
  - database
  - messaging
  - rpc
---

# Attributes

When adding attributes to telemetry, always search the [Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/) first and place each attribute at the correct telemetry level.
Wrong attributes — or attributes at the wrong level — make telemetry unqueryable and break cross-service correlation.

## Core principles

1. **Registry first.**
   Before creating a custom attribute, search the [Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/).
   If a registered attribute exists for your concept, use it — even if the name is not exactly what you would choose.
2. **No custom attributes unless necessary.**
   Custom attributes fragment querying and break tooling.
   Only create them for truly domain-specific concepts that have no registry equivalent.
   When you must, use a namespaced prefix (e.g., `com.acme.order.priority`).
3. **Low cardinality in names, high cardinality in attributes.**
   Span names and metric attribute values must be bounded.
   Put variable data (IDs, paths, user inputs) into span attributes instead.
4. **Right level, every time.**
   Place attributes at the correct telemetry level (resource, scope, span, log, metric data point).
   Never duplicate resource-level data on every span.
5. **Consistent placement.**
   Once you decide an attribute belongs at a certain level, keep it there everywhere.
   Inconsistency (sometimes resource, sometimes span) makes querying unreliable.

## Attribute placement

### Levels

| Level | What belongs here | Examples |
|---|---|---|
| **Resource** | Identity and environment of the telemetry source. Stable for the lifetime of the process. | `service.name`, `service.version`, `deployment.environment.name`, `k8s.pod.name`, `host.name` |
| **Scope** | Identity of the instrumentation library or component. | `otel.scope.name`, `otel.scope.version` |
| **Span** | Request-specific context for a single operation. | `http.request.method`, `http.response.status_code`, `db.operation.name`, `url.path` |
| **Span Event** | Point-in-time occurrences within a span. | `exception.type`, `exception.message`, `exception.stacktrace` |
| **Log Record** | Structured log entry attributes. | `log.file.path`, severity fields, body fields |
| **Metric Data Point** | Low-cardinality dimensions for aggregation. | `http.request.method`, `http.response.status_code`, `http.route` |

### Common mistakes

- **Putting `service.name` on spans.** It is a resource attribute. Putting it on spans duplicates data and wastes storage.
- **Putting `k8s.pod.name` on every span.** Kubernetes metadata belongs on the resource. The Collector's `k8sattributes` processor handles this.
- **Inconsistent placement.** If `deployment.environment.name` is a resource attribute in one service, it must be a resource attribute in every service. Mixing levels breaks cross-service queries.
- **High-cardinality metric attributes.** Attributes like `url.path` or `user.id` on metrics cause cardinality explosion. Metrics attributes must be low-cardinality. Use `http.route`, not `url.path`.
- **Putting `enduser.*` or `user.*` on resources.** User identity changes per request — these are span or log attributes, never resource attributes. A resource describes the process, not who is calling it.
- **Putting `browser.*` on resources.** Browser attributes like `browser.language` or `browser.brands` vary per request in server-side telemetry. They belong on spans, not on the resource.

## Required and important attributes

### Must-have resource attributes

| Attribute | Type | Why |
|---|---|---|
| `service.name` | string | **Required.** Identifies the service. Without it, telemetry is unattributable. Falls back to `unknown_service` if not set. |
| `service.version` | string | Enables version-aware analysis, deployment tracking, and regression detection. |
| `service.instance.id` | string | Uniquely identifies the service instance (e.g., pod). Must be unique across all instances sharing the same `service.name`. |
| `deployment.environment.name` | string | Distinguishes production from staging/dev. Previously `deployment.environment`. |
| `k8s.pod.uid` | string | **Required for Kubernetes workloads.** Enables telemetry correlation via the `k8sattributes` processor. Prefer over `k8s.pod.ip`, which breaks with service meshes (Istio, and Linkerd). |

### Most common span attributes

#### HTTP

| Current Attribute | Type | Req | Previous Name |
|---|---|---|---|
| `http.request.method` | string | Required | `http.method` |
| `http.response.status_code` | int | Conditionally required | `http.status_code` |
| `url.scheme` | string | Required | `http.scheme` |
| `url.path` | string | Required | Part of `http.target` |
| `url.query` | string | Conditionally required | Part of `http.target` |
| `url.full` | string | Required (client) | `http.url` |
| `http.route` | string | Conditionally required (server) | `http.route` (unchanged) |
| `server.address` | string | Required | `net.peer.name` (client) / `net.host.name` (server) |
| `server.port` | int | Required | `net.peer.port` / `net.host.port` |
| `client.address` | string | Recommended (server) | `http.client_ip` |
| `network.protocol.version` | string | Recommended | `http.flavor` (values changed: `2.0` → `2`) |
| `user_agent.original` | string | Recommended | `http.user_agent` |
| `error.type` | string | Conditionally required | *(new)* |

#### Database

| Current Attribute | Type | Req | Previous Name |
|---|---|---|---|
| `db.system.name` | string | Required | `db.system` |
| `db.operation.name` | string | Conditionally required | `db.operation` |
| `db.collection.name` | string | Conditionally required | `db.sql.table` / `db.mongodb.collection` |
| `db.namespace` | string | Conditionally required | `db.name` |
| `db.query.text` | string | Opt-in | `db.statement` |
| `db.response.status_code` | string | Conditionally required | *(new)* |
| `server.address` | string | Required | `net.peer.name` |
| `server.port` | int | Conditionally required | `net.peer.port` |
| `error.type` | string | Conditionally required | *(new)* |

#### Messaging

| Current Attribute | Type | Req | Previous Name |
|---|---|---|---|
| `messaging.system` | string | Required | `messaging.system` (unchanged) |
| `messaging.operation.name` | string | Required | `messaging.operation` |
| `messaging.destination.name` | string | Conditionally required | `messaging.destination` |
| `messaging.message.id` | string | Recommended | `messaging.message_id` |
| `messaging.consumer.group.name` | string | Conditionally required | `messaging.kafka.consumer_group` etc. |

#### RPC

| Current Attribute | Type | Req | Previous Name |
|---|---|---|---|
| `rpc.system` | string | Required | `rpc.system` (unchanged) |
| `rpc.service` | string | Recommended | `rpc.service` (unchanged) |
| `rpc.method` | string | Recommended | `rpc.method` (unchanged) |
| `rpc.grpc.status_code` | int | Required (gRPC) | `rpc.grpc.status_code` (unchanged) |
| `server.address` | string | Required | `net.peer.name` |
| `server.port` | int | Required | `net.peer.port` |

#### Error

| Attribute | Type | Notes |
|---|---|---|
| `error.type` | string | The error class, status code, or stable identifier. Set whenever span status is `ERROR`. See [span status code rules](../../otel-instrumentation/rules/spans.md#span-status-code). |
| `exception.type` | string | On span events of type `exception`. |
| `exception.message` | string | Human-readable error message. |
| `exception.stacktrace` | string | Full stacktrace as a string. |

### The `_OTHER` pattern

Attributes like `http.request.method` only accept a fixed set of known values. Unknown values are normalized to `_OTHER` and the original is preserved in a companion attribute (e.g., `http.request.method_original`). This bounds cardinality while retaining detail.

## Attribute registry namespaces

Before creating a custom attribute, check if it belongs to an existing namespace. The registry contains 80+ namespaces.

### Infrastructure and compute

| Namespace | Covers |
|---|---|
| `service` | Service identity: name, namespace, version, instance ID |
| `deployment` | Deployment environment, ID, name, status |
| `host` | Physical or virtual host: hostname, IP, arch, OS |
| `os` | Operating system type, version, description |
| `process` | OS process: PID, command, arguments, owner |
| `container` | Container instance: ID, image, runtime, name |
| `k8s` | Kubernetes resources: pod, node, deployment, namespace, service |
| `cloud` | Cloud provider, account, region, availability zone, platform |
| `faas` | Serverless function: name, version, trigger, invocation ID |
| `device` | Physical device: ID, manufacturer, model |

### Cloud providers

| Namespace | Covers |
|---|---|
| `aws` | AWS services: ECS, EKS, Lambda, S3, DynamoDB, SQS, SNS, Bedrock |
| `gcp` | GCP services: Cloud Run, Compute Engine, client libraries |
| `azure` | Azure services: Cosmos DB, client libraries |
| `heroku` | Heroku app, release, dyno metadata |

### Protocols and communication

| Namespace | Covers |
|---|---|
| `http` | HTTP request/response: method, status code, headers, body size |
| `rpc` | Remote procedure calls: system, service, method, status |
| `graphql` | GraphQL operations: document, operation name, type |
| `dns` | DNS queries: resolved addresses, queried domain |
| `tls` | TLS/SSL: cipher suite, protocol version, certificate details |
| `network` | Network connection: transport, protocol, peer/local address |
| `client` | Client side of connection: address, port |
| `server` | Server side of connection: address, port |

### Data systems

| Namespace | Covers |
|---|---|
| `db` | Database operations: system, operation, collection, namespace, query |
| `messaging` | Messaging systems: Kafka, RabbitMQ, SQS, destination, operation |

### URLs and identity

| Namespace | Covers |
|---|---|
| `url` | URL components: scheme, path, query, full, template |
| `user_agent` | User-Agent header: original string, parsed name |
| `user` | User identity: ID, email, name, roles |
| `enduser` | End user: ID, scope, role |
| `session` | Session: ID, previous session ID |
| `peer` | Remote service: `peer.service` for uninstrumented peers |

### Errors and diagnostics

| Namespace | Covers |
|---|---|
| `error` | Error classification: `error.type` |
| `exception` | Exception details: type, message, stacktrace |
| `code` | Source code location: function, filepath, line number |
| `thread` | Thread identity: ID, name |
| `log` | Log record metadata: file path, iostream, record UID |
| `event` | Event identification: `event.name` |

### Application runtimes

| Namespace | Covers |
|---|---|
| `jvm` | JVM: GC, memory pools, buffer pools, threads, classes |
| `dotnet` | .NET runtime: GC generation, heap info |
| `go` | Go runtime: memory types, goroutines |
| `nodejs` | Node.js: event loop state |
| `v8js` | V8 engine: GC type, heap spaces |
| `cpython` | CPython: GC generation |

### AI and ML

| Namespace | Covers |
|---|---|
| `gen_ai` | Generative AI: model, provider, token usage, tool calls |
| `openai` | OpenAI-specific: API type, service tier |
| `mcp` | Model Context Protocol: tool interactions |

### CI/CD and source control

| Namespace | Covers |
|---|---|
| `cicd` | CI/CD pipelines: pipeline name, run ID, task |
| `vcs` | Version control: repository, branch, commit, change (PR) |

### Mobile and browser

| Namespace | Covers |
|---|---|
| `browser` | Browser: brands, language, platform, mobile flag |
| `android` | Android: app state, API level |
| `ios` | iOS: app lifecycle state |
| `app` | Client apps: installation ID, screen, widgets |

### Other

| Namespace | Covers |
|---|---|
| `feature_flag` | Feature flags: key, variant, provider, evaluation |
| `test` | Testing: test case, suite, result |
| `geo` | Geolocation: country, region, city, coordinates |
| `file` | File system: name, path, size, hash |
| `artifact` | Distribution artifacts: filename, hash, version |
| `hw` | Hardware components: CPU, disk, memory, sensors |
| `system` | System metrics: filesystem, memory, paging |
| `telemetry` | Telemetry SDK: name, version, language |
| `otel` | OpenTelemetry internals: status, scope, component name |

Full registry: https://opentelemetry.io/docs/specs/semconv/registry/attributes/

## References

- [Attribute Registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/) — the single source of truth for all attribute definitions
- [Semantic Conventions Specification](https://opentelemetry.io/docs/specs/semconv/) — full specification across all signals
- [Dash0 Semantic Conventions Explainer](https://www.dash0.com/knowledge/otel-semantic-conventions-explainer) — comprehensive guide to understanding and applying conventions

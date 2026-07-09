---
title: "Serverless / sidecar-less containers"
impact: HIGH
tags:
  - deployment
  - serverless
  - app-runner
  - cloud-run
  - fly
  - lambda
  - direct-export
---

# Serverless and sidecar-less container platforms

On managed container platforms you often **cannot run a Collector** next to
your app: there is no pod to add a sidecar to, no host to run a DaemonSet on,
and no way to co-locate a process. Examples: **AWS App Runner**, **Google
Cloud Run**, **Fly.io Machines**, **Azure Container Apps**, and
container-image **AWS Lambda**.

The pattern here is the opposite of the Kubernetes patterns in
[deployment](../deployment.md): instead of the SDK exporting to a nearby
Collector, the **SDK exports OTLP directly to the backend's OTLP endpoint**.
There is no Collector in the request path.

## When to use this

Use direct SDK-to-backend export when **all** of these hold:

- You cannot deploy a Collector as a sidecar, DaemonSet, or co-process.
- Your telemetry volume is per-instance and modest (one app, not an
  aggregation point for many services).
- You do not need Collector-side processing (tail sampling, cross-signal
  enrichment, OTTL redaction) before egress.

If you outgrow this (need sampling, redaction, or fan-in from many services),
run a **gateway Collector** as a separate always-on service and point the
serverless apps' `OTEL_EXPORTER_OTLP_ENDPOINT` at it. That is the one
Collector topology that fits serverless: remote, not co-located.

## Configuration (direct export)

Drive everything through the SDK's environment variables — no Collector
config file. See the language SDK rules under `otel-instrumentation` for the
activation mechanism (e.g. Node.js uses a `NODE_OPTIONS` register hook).

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=https://<backend-otlp-endpoint>
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
# auth header per your backend, e.g. for Dash0:
#   Authorization=Bearer <auth-token>,Dash0-Dataset=<dataset>
OTEL_EXPORTER_OTLP_HEADERS=<backend auth headers>
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
OTEL_SERVICE_NAME=<service>
OTEL_RESOURCE_ATTRIBUTES=service.namespace=<ns>,deployment.environment=<env>
```

- **Protocol:** prefer `http/protobuf`. Serverless egress is HTTPS on 443;
  many of these platforms do not give you an arbitrary-port gRPC path, and
  OTLP-over-HTTP is universally reachable on 443. (Contrast with the
  Collector-to-backend path in [exporters](../exporters.md), which uses gRPC.)
- **Endpoint:** your backend's OTLP ingest endpoint (for Dash0, the org's
  Settings then Endpoints page). It is usually **not** the same host as the
  backend's query or REST API.

## The two things that bite you on serverless

### 1. Egress must actually reach the internet

Direct export only works if the container can make outbound HTTPS to the
backend's OTLP endpoint. On platforms with **VPC-only egress and no NAT/internet
route**, the exporter silently fails (connection timeouts, no spans, no
error surfaced to the user). This is the most common "I set everything and
see nothing" cause.

- **AWS App Runner** with `EgressType: VPC`: the VPC needs a **NAT gateway**
  (or the ingress reached some other way). App Runner's default
  (`EgressType: DEFAULT`, AWS-managed egress) already has internet access.
- **Cloud Run / Container Apps:** default egress reaches the internet;
  a VPC connector with "all traffic" routing plus no Cloud NAT does not.

Verify egress before debugging the SDK: `curl -sS -o /dev/null -w '%{http_code}' https://<backend-otlp-endpoint>` from inside the container (or a same-network task) should not hang.

### 2. Keep the ingest token out of your IaC

Deliver `OTEL_EXPORTER_OTLP_HEADERS` (which embeds the auth token) from the
platform's **secret store**, not as a plaintext environment variable in your
template:

- App Runner: `RuntimeEnvironmentSecrets` → a Secrets Manager secret holding
  the full header string (for Dash0: `Authorization=Bearer <token>,Dash0-Dataset=<ds>`).
- Cloud Run: `--set-secrets` from Secret Manager.
- Fly: `fly secrets set`.

Store the whole header string as one secret (env vars cannot be composed from
parts at runtime), and mint it out of band so the token never lands in a
committed file or a CloudFormation/Terraform parameter.

## Flush on shutdown

Serverless platforms send `SIGTERM` on every scale-in and rolling replace,
which is far more frequent than a long-lived VM. Batched spans buffered at
that moment are lost unless the SDK flushes on shutdown. Most language
auto-instrumentation registers a `SIGTERM`/`SIGINT` flush hook automatically;
confirm it for your SDK, and additionally flush on `uncaughtException` /
unhandled rejection (see the language rule, e.g. `otel-instrumentation`'s
Node.js graceful-shutdown section). Short-lived function runtimes
(Lambda) should use the platform's OTel Lambda layer / `BatchSpanProcessor`
with a forced flush before the handler returns.

## Worked example: AWS App Runner (Node.js container, Dash0 as the backend)

No Collector. The image adds `@opentelemetry/auto-instrumentations-node`; the
App Runner service sets the register hook and the OTLP env vars; the ingest
token is a Secrets Manager secret injected as `OTEL_EXPORTER_OTLP_HEADERS`;
the service's VPC has a NAT gateway so OTLP reaches the ingress.

```yaml
# CloudFormation: AWS::AppRunner::Service > SourceConfiguration >
# ImageRepository > ImageConfiguration
RuntimeEnvironmentVariables:
  - { Name: NODE_OPTIONS, Value: "--require @opentelemetry/auto-instrumentations-node/register" }
  - { Name: OTEL_SERVICE_NAME, Value: my-service }
  - { Name: OTEL_TRACES_EXPORTER, Value: otlp }
  - { Name: OTEL_METRICS_EXPORTER, Value: otlp }
  - { Name: OTEL_LOGS_EXPORTER, Value: otlp }
  - { Name: OTEL_EXPORTER_OTLP_PROTOCOL, Value: http/protobuf }
  - { Name: OTEL_EXPORTER_OTLP_ENDPOINT, Value: https://ingress.eu-west-1.aws.dash0.com }
  - { Name: OTEL_RESOURCE_ATTRIBUTES, Value: "service.namespace=my-ns,deployment.environment=production" }
RuntimeEnvironmentSecrets:
  # secret value: Authorization=Bearer <token>,Dash0-Dataset=<dataset>
  - { Name: OTEL_EXPORTER_OTLP_HEADERS, Value: <secret-arn> }
```

The result is identical to a Collector deployment from the backend's side — spans,
metrics, and logs arrive over OTLP — but with zero Collector infrastructure
to run or pay for.

## References

- [Deployment decision process](../deployment.md)
- [Exporters (Collector-to-Dash0, gRPC)](../exporters.md)
- Language SDK setup: the `otel-instrumentation` skill (per-language rules)
- [OpenTelemetry Collector deployment patterns](https://opentelemetry.io/docs/collector/deployment/)

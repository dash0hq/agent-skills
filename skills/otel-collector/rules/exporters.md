---
title: "Exporters"
impact: CRITICAL
tags:
  - exporters
  - otlp
  - dash0
  - authentication
---

# Exporters

Exporters send processed telemetry to backends.
Every pipeline must end with at least one exporter.

## Choosing a protocol

Use OTLP/gRPC for Collector-to-backend communication.
gRPC provides better throughput and supports bidirectional streaming, which matters at the Collector's aggregation volume.
Fall back to OTLP/HTTP only when network proxies do not support HTTP/2.

| Protocol | Exporter key | Default port | When to use |
|----------|-------------|-------------|-------------|
| gRPC | `otlp` | 4317 | Default for all Collector-to-backend exports |
| HTTP | `otlphttp` | 4318 | Network proxies that block HTTP/2 |

## OTLP/gRPC exporter to Dash0

Use the OTLP/gRPC exporter to send traces, metrics, and logs to Dash0.

### Where to get configuration values

1. **OTLP Endpoint**: In Dash0: [Settings → Organization → Endpoints](https://app.dash0.com/settings/endpoints?s=eJwtyzEOgCAQRNG7TG1Cb29h5REMcVclIUDYsSLcXUxsZ95vcJgbxNObEjNET_9Eok9wY2FIlzlNUnJItM_GYAM2WK7cqmgdlbcDE0yjHlRZfr7KuDJj2W-yoPf-AmNVJ2I%3D)
2. **Auth Token**: In Dash0: [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens)

### Minimal configuration

```yaml
exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer <AUTH_TOKEN>"
```

Replace `<OTLP_ENDPOINT>` with your Dash0 OTLP endpoint (e.g., `ingress.eu-west-1.aws.dash0.com:4317`).
Replace `<AUTH_TOKEN>` with your Dash0 auth token; see the [Authentication](#authentication) section for how to optimally set up the authentication token.

### Production configuration

Configure retry, timeout, compression, and sending queue for reliable delivery.

```yaml
exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer <AUTH_TOKEN>"
    compression: gzip
    timeout: 30s
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
      storage: file_storage
```

### Compression

Enable `gzip` compression to reduce network bandwidth.
The Dash0 ingress endpoint supports gzip-compressed OTLP/gRPC.

```yaml
# GOOD — reduces bandwidth by 60-80 percent
exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer <AUTH_TOKEN>"
    compression: gzip

# BAD — uncompressed traffic wastes bandwidth
exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer <AUTH_TOKEN>"
    compression: none
```

### Retry on failure

Enable retries to handle transient network errors and backend unavailability.

| Setting | Default | Recommendation |
|---------|---------|----------------|
| `initial_interval` | `5s` | Keep default unless backend has strict rate limiting |
| `max_interval` | `30s` | Keep default for exponential backoff ceiling |
| `max_elapsed_time` | `300s` | Increase for backends with extended maintenance windows |
| `randomization_factor` | `0.5` | Keep default to spread retry storms |

### Sending queue

The sending queue buffers telemetry when the backend is temporarily unavailable.
Without it, data is dropped during transient failures.

```yaml
exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer <AUTH_TOKEN>"
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
      storage: file_storage
```

| Setting | Default | Recommendation |
|---------|---------|----------------|
| `num_consumers` | `10` | Increase for high-throughput pipelines |
| `queue_size` | `1000` | Set to 5000 for production workloads |
| `storage` | (in-memory) | Set to `file_storage` for persistence across restarts |

Use `file_storage` with the [file storage extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/storage/filestorage) to persist the queue to disk.
In-memory queues lose buffered data when the Collector restarts.

### Authentication

Do not hardcode auth tokens in configuration files.
Reference an environment variable instead.

Create a dedicated auth token with `Ingesting` permissions only.
The Collector needs to send telemetry, not query or manage the organization.
In Dash0, create the token at [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens) and select the `Ingesting` scope.
See [auth tokens](https://www.dash0.com/documentation/dash0/key-concepts/auth-tokens) for details on available permission scopes.

```yaml
# GOOD — token from environment variable
exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"

# BAD — hardcoded token in config
exporters:
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer dh0_1a2b3c4d5e6f..."
```

Set the environment variable in your deployment manifest:

```yaml
env:
  - name: DASH0_AUTH_TOKEN
    valueFrom:
      secretKeyRef:
        name: dash0-credentials
        key: auth-token
```

## OTLP/HTTP exporter

Use the OTLP/HTTP exporter when gRPC is not available (e.g., network proxies that do not support HTTP/2).

```yaml
exporters:
  otlphttp:
    endpoint: https://<OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
    compression: gzip
```

The OTLP/HTTP exporter uses port 4318 by default.
Check your Dash0 endpoint documentation for the correct URL.

## Debug exporter

Use the debug exporter during development to print telemetry to the Collector's stdout.
Do not enable the debug exporter in production — it generates excessive log output.

```yaml
# GOOD — development only
exporters:
  debug:
    verbosity: detailed

# BAD — debug exporter in production pipeline
exporters:
  debug:
    verbosity: detailed
  otlp:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlp, debug]  # debug wastes CPU and I/O in production
```

### Verbosity levels

| Level | Output | Use for |
|-------|--------|---------|
| `basic` | One line per export (count only) | Verifying that data flows through the pipeline |
| `normal` | One line per telemetry item | Spot-checking individual items |
| `detailed` | Full telemetry item with all attributes | Debugging attribute values and structure |

## Multiple exporters

Send telemetry to multiple backends by listing multiple exporters in a pipeline.
Each exporter receives a copy of the data independently.

```yaml
exporters:
  otlp/dash0:
    endpoint: <OTLP_ENDPOINT>
    headers:
      Authorization: "Bearer ${env:DASH0_AUTH_TOKEN}"
  otlp/secondary:
    endpoint: secondary-backend:4317
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter]
      exporters: [otlp/dash0, otlp/secondary]
```

Use named instances (`otlp/dash0`, `otlp/secondary`) to configure multiple exporters of the same type.

## Anti-patterns

- **Exporting without TLS.**
  OTLP/gRPC uses TLS by default.
  Setting `tls.insecure: true` sends telemetry (including potentially sensitive data) in plaintext.
  Only disable TLS for local development or within a trusted network with mTLS (e.g., a service mesh).
- **Hardcoding auth tokens in configuration files.**
  Tokens in config files end up in version control, container images, and logs.
  Use environment variables with `${env:VARIABLE_NAME}` syntax.
- **Missing `sending_queue` in production.**
  Without a queue, transient backend failures cause immediate data loss.
  Always enable the sending queue for production deployments.
- **Using debug exporter in production.**
  The debug exporter serializes every telemetry item to stdout, consuming CPU and disk I/O.
  Remove it from production pipelines.

## References

- [OTLP/gRPC exporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlpexporter)
- [OTLP/HTTP exporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter)
- [Debug exporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/debugexporter)
- [File storage extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/storage/filestorage)

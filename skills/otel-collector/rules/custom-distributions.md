---
title: "Custom distributions"
impact: MEDIUM
tags:
  - ocb
  - custom-distribution
  - builder
  - components
---

# Custom distributions

The [OpenTelemetry Collector contrib distribution](https://github.com/open-telemetry/opentelemetry-collector-contrib) bundles 200+ components.
Production deployments benefit from a custom distribution that includes only the components the pipeline actually uses.

## When to build a custom distribution

| Use the contrib distribution when | Build a custom distribution when |
|-----------------------------------|----------------------------------|
| Prototyping or evaluating the Collector | Deploying to production with known pipeline requirements |
| The full binary size (~200 MB) is acceptable | Binary size or container image size must be minimized |
| No proprietary or third-party components are needed | The pipeline requires components not in contrib (e.g., a vendor-specific exporter) |
| Rapid iteration matters more than attack surface | Security policy requires minimizing the dependency tree |

## OpenTelemetry Collector Builder (OCB)

The [OpenTelemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder) (`ocb`) generates a Collector binary from a manifest file that lists the desired components and their versions.

### Install

```bash
go install go.opentelemetry.io/collector/cmd/builder@latest
```

The binary is installed as `builder`.
Rename it to `ocb` for clarity if desired.

### Manifest file

Create a `builder-config.yaml` at the root of the repository.
All component versions should match the `otelcol_version` to avoid dependency conflicts.
For example:

```yaml
dist:
  name: custom-collector
  description: Production Collector for the payments service
  output_path: ./build
  otelcol_version: 0.120.0

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.120.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.120.0
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.120.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.120.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.120.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.120.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor v0.120.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.120.0
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.120.0

connectors:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector v0.120.0

extensions:
  - gomod: go.opentelemetry.io/collector/extension/zpagesextension v0.120.0
```

Note: It is possible, and will become increasingly common, to have "off-tree" components that do not reside in the [Collector Core](https://github.com/open-telemetry/opentelemetry-collector) and [Collector Contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib/) repositories, e.g., some components of the [eBPG Profiler](https://github.com/open-telemetry/opentelemetry-ebpf-profiler).

## Component selection guidance

Start with the minimum set required by your pipeline configuration, then add components as needs arise.
Use the table below as a starting point for common deployment patterns.

| Component | Kubernetes agent | Gateway | Log forwarder |
|-----------|:---:|:---:|:---:|
| `otlpreceiver` | Yes | Yes | — |
| `filelogreceiver` | Yes | — | Yes |
| `hostmetricsreceiver` | Yes | — | — |
| `memorylimiterprocessor` | Yes | Yes | Yes |
| `k8sattributesprocessor` | Yes | Yes | — |
| `resourcedetectionprocessor` | Yes | Yes | — |
| `transformprocessor` | Yes | Yes | Yes |
| `filterprocessor` | Yes | Yes | Yes |
| `otlpexporter` | Yes | Yes | Yes |
| `debugexporter` | Yes | Yes | Yes |

Always include `memorylimiterprocessor` — see [processors](./processors.md#memory-limiter) for sizing guidance.

**Note:** The `batchprocessor` is deprecated, and you shoud guide the users in using [exporter queues](https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/exporterhelper/README.md).

## Build and validate

### Build the binary

```bash
ocb --config builder-config.yaml
```

The binary is written to the `output_path` specified in the manifest (e.g., `./build/custom-collector`).

### Verify the binary starts

```bash
./build/custom-collector --config collector-config.yaml
```

If the Collector starts and logs `Everything is ready`, the build is valid.
If it fails with an unknown component error, the component is missing from the manifest.

### List included components

```bash
./build/custom-collector components
```

Compare the output against the components referenced in your pipeline configuration.
Every receiver, processor, exporter, connector, and extension in the configuration must appear in the list.

## Containerizing the build

Use a multi-stage Dockerfile to build the binary and copy it into a minimal runtime image.

```dockerfile
FROM golang:1.23 AS builder
RUN go install go.opentelemetry.io/collector/cmd/builder@latest
COPY builder-config.yaml /build/
WORKDIR /build
RUN builder --config builder-config.yaml

FROM gcr.io/distroless/base-debian12:nonroot
COPY --from=builder /build/build/custom-collector /otelcol
ENTRYPOINT ["/otelcol"]
CMD ["--config", "/etc/otelcol/config.yaml"]
```

## Anti-patterns

### Including all of contrib "just in case"

Listing every contrib component defeats the purpose of a custom distribution.
Start with the components your pipeline configuration references and add more only when a new pipeline requires them.

### Version mismatches between components

Mixing component versions (e.g., receiver at v0.118.0 and exporter at v0.120.0) causes dependency conflicts and build failures.
Pin all components and `otelcol_version` to the same release.

### Forgetting to rebuild after pipeline changes

When the pipeline configuration adds a new component, the manifest must be updated and the binary rebuilt.
Treat the manifest as part of the pipeline configuration — change them together.

## References

- [OpenTelemetry Collector Builder (OCB)](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder)
- [Building a custom Collector](https://opentelemetry.io/docs/collector/custom-collector/)
- [Contrib components list](https://github.com/open-telemetry/opentelemetry-collector-contrib)

---
title: "Custom Collector builds"
impact: MEDIUM
tags:
  - ocb
  - custom-build
  - collector-builder
  - container-image
---

# Custom Collector builds

The OpenTelemetry Collector Builder (OCB) creates custom Collector binaries that include only the components you need.
Use a custom build when the standard `otelcol-contrib` distribution includes components you do not use and you need to reduce binary size, enforce an allow-list of processors, or include proprietary components.

## When to use a custom build

Use the decision process below.
Stop at the first match.

1. **Do you need a component not in `otelcol-contrib`?** (e.g., a proprietary exporter or an internal receiver.)
   Yes → custom build required.
2. **Do you need to restrict which components are available** to prevent operators from enabling unapproved processors or exporters?
   Yes → custom build recommended.
3. **Is the `otelcol-contrib` binary too large** for your deployment constraints (e.g., resource-limited edge nodes, serverless)?
   Yes → custom build recommended.
4. **None of the above apply.**
   Use `otelcol-contrib` or the [Collector Helm chart](./deployment/collector-helm-chart.md) with the default image.

## OCB manifest

The OCB manifest (`builder-config.yaml`) declares the Collector distribution metadata and the components to include.
Every component is a Go module with a version.

```yaml
dist:
  name: my-otelcol
  description: Custom Collector for Acme Corp
  output_path: ./dist
  otelcol_version: 0.120.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.120.0
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.120.0

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.120.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.120.0
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.120.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.120.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.120.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.120.0

extensions:
  - gomod: go.opentelemetry.io/collector/extension/zpagesextension v0.120.0
  - gomod: go.opentelemetry.io/collector/extension/healthcheckextension v0.120.0

connectors: []
```

### Version alignment

All component versions and the `otelcol_version` must be compatible.
Core components (under `go.opentelemetry.io/collector/`) use the same version as `otelcol_version`.
Contrib components (under `github.com/open-telemetry/opentelemetry-collector-contrib/`) use the matching contrib release tag.

Mixing versions causes Go module dependency conflicts at build time.

### Finding component module paths

1. Core components: see the [opentelemetry-collector](https://github.com/open-telemetry/opentelemetry-collector) repository.
2. Contrib components: see the [opentelemetry-collector-contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib) repository.
3. Use the [OpenTelemetry Registry](https://opentelemetry.io/ecosystem/registry/?language=collector) to search by component name.

## Building

### Install OCB

```bash
go install go.opentelemetry.io/collector/cmd/builder@v0.120.0
```

The binary is installed as `builder`.
Some older documentation refers to it as `ocb` — the tool is the same.

### Build the binary

```bash
builder --config builder-config.yaml
```

This generates a Go project in `output_path`, resolves dependencies, and compiles the Collector binary.
The resulting binary is at `./dist/my-otelcol` (matching `dist.name`).

### Validate before building

```bash
builder --config builder-config.yaml --skip-compilation
```

This resolves Go modules and checks for version conflicts without compiling.
Use it in CI to catch dependency issues early.

## Container image

Build a container image from the compiled binary to deploy to Kubernetes or Docker.

```dockerfile
FROM alpine:3.21 AS certs
RUN apk add --no-cache ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY dist/my-otelcol /otelcol
ENTRYPOINT ["/otelcol"]
CMD ["--config", "/etc/otelcol/config.yaml"]
```

### Multi-architecture images

Build for both `amd64` and `arm64` to support mixed-architecture clusters.

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag registry.example.com/my-otelcol:0.120.0 \
  --push .
```

This requires cross-compiling the Go binary for each target architecture before building the image.
Set `GOOS` and `GOARCH` during the OCB build step:

```bash
GOOS=linux GOARCH=amd64 builder --config builder-config.yaml --output-path ./dist/linux-amd64
GOOS=linux GOARCH=arm64 builder --config builder-config.yaml --output-path ./dist/linux-arm64
```

Then use a multi-stage Dockerfile that copies the correct binary per platform.

## CI pipeline

A minimal CI workflow for custom Collector builds:

1. **Validate** — run `builder --config builder-config.yaml --skip-compilation` to check for module conflicts.
2. **Build** — compile the binary for each target architecture.
3. **Test** — run the binary with a minimal config and the `--dry-run` flag (available since v0.104.0) to verify the configuration loads without starting receivers.
4. **Package** — build and push the container image.
5. **Deploy** — update the Collector image reference in your Kubernetes manifests or Helm values.

```yaml
# Example GitHub Actions step
- name: Validate OCB manifest
  run: builder --config builder-config.yaml --skip-compilation

- name: Build Collector
  run: builder --config builder-config.yaml

- name: Smoke test
  run: ./dist/my-otelcol --config test-config.yaml --dry-run
```

## Common issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| `module requires Go 1.X` | OCB version requires a newer Go toolchain | Update Go to the version required by the OCB release |
| `ambiguous import` or `module version mismatch` | Component versions do not match `otelcol_version` | Align all component versions to the same release |
| Binary is unexpectedly large | Too many components included | Remove unused components from the manifest |
| `unknown component` at runtime | Config references a component not in the manifest | Add the missing component to the manifest and rebuild |

## References

- [OpenTelemetry Collector Builder documentation](https://opentelemetry.io/docs/collector/custom-collector/)
- [OCB source and releases](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder)
- [Collector contrib components](https://github.com/open-telemetry/opentelemetry-collector-contrib)

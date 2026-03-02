---
title: 'Resolving configuration values'
impact: CRITICAL
tags:
  - configuration
  - resource-attributes
  - service-name
  - service-version
  - environment
  - kubernetes
  - exporter
---

# Resolving configuration values

OpenTelemetry instrumentation requires project-specific values that cannot be hardcoded in guidance.
Before writing any instrumentation code, resolve each value below using the ordered lookup strategies.
Use the first source that yields a result.
If no source matches, ask the user.

## Resource attributes

Resource attributes identify *what* is producing telemetry.
See [resource attributes](./resources.md) for semantics and constraints on each attribute.

### Service identity

These attributes identify the logical service and are resolved from the application's codebase, build metadata, or deployment configuration.
They apply to every deployment target (bare metal, Docker, Kubernetes, serverless).

#### `service.name`

Check the following sources in order:

1. **Existing OpenTelemetry configuration** — look for `OTEL_SERVICE_NAME` and `OTEL_RESOURCE_ATTRIBUTES` in `.env`, `.env.local`, `.env.production`, `docker-compose.yml`, Kubernetes manifests, or Helm values files.
2. **Package manifest name field** — read the project name from the build metadata file:
   - Node.js: `name` in `package.json`.
   - Java (Maven): `artifactId` in `pom.xml`.
   - Java (Gradle): `rootProject.name` in `settings.gradle` or `settings.gradle.kts`.
   - .NET: `<AssemblyName>` or `<RootNamespace>` in the `.csproj` file.
   - Go: last path segment of the `module` directive in `go.mod` (e.g., `github.com/acme/order-api` → `order-api`).
   - Python: `name` in `pyproject.toml`, `setup.cfg`, or `setup.py`.
   - Ruby: `spec.name` in the `.gemspec`, or the directory name if none exists.
   - PHP: `name` in `composer.json` (take the segment after the `/`).
3. **Project directory name** — use the name of the root project directory as a last resort.

Convert the result to kebab-case if it contains spaces, underscores, or mixed case (e.g., `OrderApi` → `order-api`).

See [resource attributes](./resources.md#servicename) for naming constraints (stable, unique, human-readable, case-consistent).

#### `service.version`

Check the following sources in order:

1. **Existing OpenTelemetry configuration** — look for `service.version` in `OTEL_RESOURCE_ATTRIBUTES` in `.env`, `.env.local`, `.env.production`, `docker-compose.yml`, Kubernetes manifests, or Helm values files.
2. **CI/CD version injection** — look for version variables in CI pipeline files (`.github/workflows/*.yml`, `.gitlab-ci.yml`, `Jenkinsfile`).
   If the pipeline writes a version into an environment variable or build arg, reference that variable instead of hardcoding.
3. **Package manifest version field**:
   - Node.js: `version` in `package.json`.
   - Java (Maven): `version` in `pom.xml`.
   - Java (Gradle): `version` in `build.gradle` or `build.gradle.kts`.
   - .NET: `<Version>` or `<AssemblyVersion>` in the `.csproj` file.
   - Go: latest git tag (Go modules do not embed a version in `go.mod`).
   - Python: `version` in `pyproject.toml`, `setup.cfg`, or `setup.py`.
   - Ruby: `spec.version` in the `.gemspec`.
   - PHP: `version` in `composer.json`.
4. **Git metadata** — use the output of `git describe --tags --always` to derive a version.
   If tags exist, this produces a tag-based version (e.g., `v1.4.2`).
   If no tags exist, use the first 7 characters of the current commit SHA.

Never hardcode a literal version string (e.g., `1.0.0`) in environment files or code.
Always reference a variable that the build pipeline or deployment tooling populates.

#### `deployment.environment.name`

Check the following sources in order:

1. **Existing OpenTelemetry configuration** — look for `deployment.environment.name` in `OTEL_RESOURCE_ATTRIBUTES` in `.env`, `.env.local`, `.env.production`, `docker-compose.yml`, Kubernetes manifests, or Helm values files.
2. **Framework environment variables** — check for an existing variable that indicates the deployment target:
   - Node.js: `NODE_ENV`.
   - Ruby: `RAILS_ENV` or `RACK_ENV`.
   - Python (Django): `DJANGO_SETTINGS_MODULE` (extract the environment segment, e.g., `config.settings.production` → `production`).
   - Python (Flask): `FLASK_ENV`.
   - Java (Spring): `SPRING_PROFILES_ACTIVE`.
   - .NET: `ASPNETCORE_ENVIRONMENT` or `DOTNET_ENVIRONMENT`.
   - PHP (Laravel): `APP_ENV`.
   - Go: no standard variable; check for `ENV`, `APP_ENV`, or `GO_ENV` in existing config.
3. **Kubernetes namespace** — if the project has Kubernetes manifests, use the namespace as a proxy (e.g., `production`, `staging`, `dev`).
4. **Dockerfile or Compose target** — check for `TARGET` or `ENVIRONMENT` build args.

If no source exists, use a variable reference (e.g., `${DEPLOYMENT_ENVIRONMENT}`) as a placeholder and leave a code comment instructing the operator to set it at deploy time.
Do not default to `development` or `production` — an incorrect default is worse than a missing value because it silently mixes telemetry.

#### `service.namespace`

Check the following sources in order:

1. **Existing OpenTelemetry configuration** — look for `service.namespace` in `OTEL_RESOURCE_ATTRIBUTES`.
2. **Monorepo structure** — if the service lives under a parent directory that groups related services (e.g., `apps/webstore/checkout-service`), use the grouping directory name (e.g., `webstore`).
3. **Organization or product name** — derive from the repository name or organization prefix (e.g., `acme-webstore`).

If no grouping concept exists in the project, omit `service.namespace` rather than inventing one.

#### `service.instance.id`

The triplet (`service.namespace`, `service.name`, `service.instance.id`) must be globally unique.
The value must be stable for the lifetime of the process and must not expose sensitive infrastructure details (pod names, container IDs) without explicit user consent.
Do not ask the user — derive it automatically.

Generate a random [RFC 4122](https://www.rfc-editor.org/rfc/rfc4122) UUID v4 at process startup.
When a stable (deterministic) identifier is preferred over a random one, generate a UUID v5 from an inherent unique value (e.g., the Kubernetes pod UID) using the OpenTelemetry-defined namespace `4d63009a-8d0f-11ee-aad7-4c796ed8e320`.

Do not ever use `$(hostname)`.
Hostnames are not guaranteed to be unique — multiple processes on the same host, containers sharing a hostname, or recycled hostnames in auto-scaling groups all produce collisions.

If the application runs multiple worker processes inside a single container (e.g., Gunicorn, Puma, PHP-FPM), each worker must have its own `service.instance.id`.

### Kubernetes attributes

These attributes identify the Kubernetes workload and are resolved from infrastructure manifests, Collector configuration, or infrastructure-as-code.
They apply only when the application runs in Kubernetes.
For the full list of Kubernetes resource attributes and downward API configuration, see [Kubernetes deployment](./platforms/k8s.md).

#### `k8s.container.name`

Read the container name from the pod spec in the project's Kubernetes manifests.
Search for Deployment, StatefulSet, DaemonSet, or Job manifests in directories like `k8s/`, `deploy/`, `manifests/`, `charts/`, or `helm/`.
Use the `name` field under `spec.containers[]`.

If the pod has a single container, set `k8s.container.name` to match that container name.
If the pod has multiple containers, set `k8s.container.name` on each container's `env` block to match its own `name`.

#### `k8s.cluster.name`

This value is external to the cluster and cannot be inferred from inside the codebase.

Check the following sources in order:

1. **Existing Collector configuration** — search for `k8s.cluster.name` in OpenTelemetry Collector config files (`otel-collector-config.yaml`, Helm values).
2. **Infrastructure-as-code** — check Terraform (`*.tf`), Pulumi, or CloudFormation files for the cluster name variable.
3. **Ask the user** — if no source exists, ask for the cluster name.

## Exporter connection

Exporter settings control *where* telemetry is sent and how the SDK authenticates with the backend.

### OTLP endpoint

The OTLP endpoint (`OTEL_EXPORTER_OTLP_ENDPOINT`) tells the SDK where to send traces, metrics, and logs.

Check the following sources in order:

1. **Existing configuration** — look for `OTEL_EXPORTER_OTLP_ENDPOINT` in `.env`, `.env.local`, `.env.production`, `docker-compose.yml`, Kubernetes manifests (ConfigMaps, Secrets), or Helm values files.
2. **Collector sidecar or DaemonSet** — if the project deploys an OpenTelemetry Collector as a sidecar or DaemonSet, the SDK endpoint is `http://localhost:4317` (gRPC) or `http://localhost:4318` (HTTP).
3. **Ask the user** — if no existing configuration exists, ask for the OTLP endpoint URL.

Never invent placeholder URLs.
Use `<OTLP_ENDPOINT>` as a placeholder only in code comments or documentation, never as an actual value.

### Authentication to backends

The authorization header (`OTEL_EXPORTER_OTLP_HEADERS`) authenticates the SDK with the backend.

Check the following sources in order:

1. **Existing configuration** — look for `OTEL_EXPORTER_OTLP_HEADERS` in `.env`, `.env.local`, `.env.production`, `docker-compose.yml`, Kubernetes Secrets, or Helm values files.
2. **Localhost collectors** — if the endpoint is `localhost`, no auth header is needed.
   The collector handles authentication with the backend separately.
3. **Ask the user** — if no existing configuration exists, ask for the auth token.

Never write auth tokens directly into source code or committed configuration files.
Use `<AUTH_TOKEN>` as a placeholder only in code comments or documentation, never as an actual value.

In Kubernetes, store the token in a Secret and mount it as an environment variable:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: otel-auth
  namespace: <namespace>
type: Opaque
stringData:
  token: "<AUTH_TOKEN>"
```

Reference the Secret in the pod spec:

```yaml
env:
  - name: OTEL_EXPORTER_OTLP_HEADERS
    valueFrom:
      secretKeyRef:
        name: otel-auth
        key: token
```

The `OTEL_EXPORTER_OTLP_HEADERS` value must include the full header string (e.g., `Authorization=Bearer <token>`).
Set the Secret's `stringData.token` field accordingly.

Outside Kubernetes, store the token in a secrets manager or an untracked `.env` file (listed in `.gitignore`).

#### Dash0 auth tokens

In Dash0, create auth tokens at [Settings → Auth Tokens → Create Token](https://app.dash0.com/settings/auth-tokens).
Auth tokens operate at the organization level and grant access to all permitted resources within that organization.

Apply the principle of least privilege when choosing permissions:

- **Backend services** — create a token with `Ingesting` permissions only.
  Backend tokens are stored server-side and are not exposed to end users, but limiting scope reduces the impact of a leak.
- **Browser and client-side SDKs** — create a separate, dedicated token with `Ingesting` permissions only and limited dataset access.
  Browser tokens are embedded in client code and visible to anyone who inspects the page source.
  Never reuse a backend token for browser instrumentation.

Revoke tokens immediately if they are compromised or no longer needed.
See [auth tokens](https://www.dash0.com/documentation/dash0/key-concepts/auth-tokens) for full details.

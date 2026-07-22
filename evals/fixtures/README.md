# Fixture contract

Fixtures are deliberately uninstrumented applications that eval scenarios point the agent at.
The agent adds OpenTelemetry instrumentation; the harness then builds and runs the result and judges the telemetry that reaches the sink.
Every application fixture directory under `evals/fixtures/` follows the contract below so scenario code, Docker hooks, and the traffic driver work identically across languages.
The exception is [`collector-workspace/`](./collector-workspace/README.md), which is not an application but an OpenTelemetry Collector configuration workspace with its own contract, executed as a host process by the Collector fixture hooks in `evals/scenarios/collector_hooks.go`.

## Contract

Each fixture is a minimal HTTP service with exactly this behavior:

- 1 inbound endpoint: `GET /checkout` returns a `200` JSON response.
- 1 outbound HTTP call: handling `GET /checkout` performs a `GET` request to the URL in the `DOWNSTREAM_URL` environment variable, so the harness can point the fixture at a stub downstream server.
- The listen port comes from the `PORT` environment variable, defaulting to `8080`.
- A `Dockerfile` at the fixture root builds a runnable image with a deterministic, pinned base image; the build may reach package registries, the running container may not (R21).
- No OpenTelemetry dependencies, imports, or configuration: the fixture is the code the agent instruments, so it must start from zero.

Keep fixtures plain and idiomatic for their language.
They stand in for real application code, so avoid cleverness the agent would not encounter in the wild.

## Browser fixture exception

`browser-service` is the one fixture whose telemetry source is a web page rather than the server process, so it extends the contract instead of following it exactly.
The full contract still applies — the server serves `GET /checkout` backed by the `DOWNSTREAM_URL` call, honors `PORT`, ships a `Dockerfile`, and carries no OpenTelemetry dependencies — and the deviations below are additions:

- `GET /checkout-data` is the same-origin endpoint the page's JavaScript fetches on load; it reuses the `/checkout` handler, so the outbound `DOWNSTREAM_URL` call still happens.
- `GET /env.js` exposes the server's `EVAL_`-prefixed runtime configuration to the page as `window.__EVAL_ENV__`, because browsers cannot read process environment variables.
- `GET /` and the static assets under `static/` are the page whose in-browser activity produces the telemetry; the server process stays uninstrumented and out of the scenario assertions.

The contract lint in `evals/scenarios/fixtures_test.go` encodes this list as `browserContractExceptions` and fails when a documented exception disappears from the fixture, so the lint stays at full strength for every fixture.

## Synthetic data

All fixture data must be obviously synthetic so leaked telemetry can never be mistaken for real customer data.
Use reserved example domains and `TEST-`-prefixed identifiers, for example `user@example.test`, `TEST-0001`, and `TEST-SKU-0001`.
Never embed real names, emails, tokens, or endpoints.

## How the harness runs a fixture

The runner copies the fixture into a temporary workspace, lets the agent modify the copy, and then invokes the scenario's `FixtureHooks`.
The Docker hooks build the workspace image, start it on an internal Docker network next to an OTLP relay (alias `otel-relay`) and a stub downstream server (alias `downstream`), and drive traffic at `GET /checkout` through a helper container.
The composed container environment supplies `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`, `OTEL_EXPORTER_OTLP_HEADERS` (bearer authentication), `OTEL_RESOURCE_ATTRIBUTES` (the per-run `test.id`), `PORT`, and `DOWNSTREAM_URL`; the agent's changes must read exporter configuration from those variables rather than hardcode endpoints or credentials.

## Adding a language

1. Create `evals/fixtures/<language>-service/` implementing the contract above.
2. Register a scenario for it in `evals/scenarios/` declaring the SDK rule file it covers, and remove the matching `pendingScenarios` entry in `evals/harness/registry.go`.
3. Extend the fixture contract lint test in `evals/scenarios/fixtures_test.go` with the new directory.

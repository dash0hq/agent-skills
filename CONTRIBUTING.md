# Contributing

This repository holds OpenTelemetry skills consumed operationally by coding agents.
Authoring rules for skill content live in [AGENTS.md](AGENTS.md) (the canonical copy; [CLAUDE.md](CLAUDE.md) is a symlink to it): prose style, the skill separation of concerns, the eval registry rules, and the fenced-example annotation contract.
Read those before changing anything under `skills/`.

This guide covers one thing those rules point at but do not walk through: how to run the evals locally to judge whether the skills actually work.

## What the evals measure

The eval harness in `evals/custom/` runs a coding agent, headless, against fixture applications with a skill loaded, then asserts on the telemetry that actually reaches an OpenTelemetry Collector sink.
A passing scenario means an agent following the skill produced working telemetry.
A failing scenario names where it broke, which is the signal you use to judge skill effectiveness.

The full operator manual — CI workflows, the quarantine process, and the pinned-version bump process — is [evals/custom/README.md](evals/custom/README.md).
This section is the local quick start.

## Prerequisites

- Go at the version pinned in `evals/custom/go.mod` (1.26.3 or newer).
- An `ANTHROPIC_API_KEY` for the scenarios that invoke the agent.
  The hermetic tests and example validation need no key.
  Keep it in a local `.env` file rather than the command line — see "Configuring the API key" below.
- The `claude` CLI at the version pinned in `evals/custom/versions.env`, on `PATH`.
  Override the binary with `EVAL_AGENT_BINARY` if it is installed under another name.
- Docker, running, for the instrumentation, browser, and Kubernetes scenarios.
  The Collector, OTTL, and semantic-convention scenarios run `otelcol-contrib` as a host process and need no Docker.
- `kind` for the Kubernetes scenarios only.

Each agent scenario is a real headless agent run, so it costs money — on the order of cents to low dollars per scenario, and tens of dollars for the full matrix.
Set `EVAL_VERDICT_DIR` to capture each verdict as JSON, which records `total_cost_usd` per run.

## Configuring the API key

Copy the template in the repository root and fill in your key:

```bash
cp .env.example .env
# then edit .env and set ANTHROPIC_API_KEY=sk-...
```

`.env` is gitignored and never committed.
The scenario entrypoints load it automatically before running, so you can run the agent scenarios without prefixing every command with the key.
A variable already set in the environment always wins over the file, so `ANTHROPIC_API_KEY=... go test ...` still overrides `.env` for a one-off run.
The file also accepts the optional `EVAL_AGENT_BINARY`, `EVAL_SCENARIOS`, and `EVAL_VERDICT_DIR` knobs documented in the template.

## The three layers

Run them from the cheapest signal to the most thorough.

### Content correctness — no key, no Docker

```bash
cd evals && go run ./cmd/validate-examples
```

Extracts and validates every fenced example in `skills/` — Collector configurations, OTTL statements, Kubernetes manifests, and SDK code.
SDK code blocks are classified into complete versus fragment: complete Go blocks compile against a pinned OpenTelemetry Go SDK dependency set via the host `go` toolchain, complete blocks in other languages report `skipped-no-toolchain` until their fixture-image compilers land, and fragments (import snippets, method bodies, and elided examples) are reported in the `code-fragment` category but not compiled.
When `go` is absent, complete Go blocks report `skipped-no-toolchain` rather than passing silently.
The run opens with a one-line summary that shows the real validated-versus-exempt split — including how many code blocks compiled and how many were skipped for want of a toolchain — so a green run cannot overstate itself.
Pass `--dry-run` for the per-block classification report, including the code-complete versus code-fragment split, without failing on errors.

### Effectiveness, the Docker-free subset — needs only the API key

The Collector, OTTL, and semantic-convention scenarios run the pinned `otelcol-contrib` as a host process, so they need no Docker:

```bash
cd evals
EVAL_SCENARIOS=collector-pipeline-hardening,ottl-redaction,ottl-enrichment,semconv-attributes \
  go test ./scenarios -run TestScenarios -v -timeout 30m
```

### Full effectiveness — API key and Docker

Start Docker first.
The instrumentation, `.NET`, and browser scenarios build and run fixture containers:

```bash
cd evals
# One SDK, the fastest end-to-end check:
EVAL_SCENARIOS=instr-go-http go test ./scenarios -run TestScenarios -v -timeout 30m

# The whole non-Kubernetes matrix — omit EVAL_SCENARIOS to run everything:
go test ./scenarios -run TestScenarios -v -timeout 90m
```

The Kubernetes scenarios are a separate entrypoint and need Docker and `kind`:

```bash
cd evals
go test ./scenarios -run TestKubernetesScenarios -v -timeout 60m
```

These commands read `ANTHROPIC_API_KEY` from the repository-root `.env` (see "Configuring the API key").
Without a key — no `.env` entry and none in the environment — the scenarios skip and `go test ./...` runs only the hermetic harness tests, which prove the harness works rather than judging the skills.

## Reading a verdict

A failing scenario prints its failure class, which tells you where the agent broke:

- `agent-noskill` — the agent never consulted the skill.
- `agent-build` — the agent's output does not build.
- `agent-telemetry` — the output builds, but no telemetry arrived, so the skill's guidance is incomplete.
- `agent-assert` — telemetry arrived but has the wrong shape, for example a missing `service.name`.
- `infra` or `infra-fail` — an API, Docker, or harness problem, not the skill's fault.

Set `EVAL_VERDICT_DIR=/tmp/verdicts` to write each verdict as JSON alongside the agent transcript and the received-telemetry paths, so you can inspect exactly what the agent did and what the sink received.

## Quarantined scenarios

`evals/custom/quarantine.yaml` lists scenarios the pull-request gate skips, so a known-red scenario cannot block unrelated work; the release run still executes and reports them, so recovery stays observable.
To watch a quarantined scenario locally, name it explicitly in `EVAL_SCENARIOS`.

No scenarios are currently quarantined.
`instr-go-logs` and `instr-dotnet-nuget` were quarantined as known skill-content gaps this eval exists to catch (the Go log-correlation recipe and the .NET NuGet install path were undocumented in the skills), but both were un-quarantined once repeated real runs showed the agent handles the gap itself; skill-doc improvements are tracked as GitHub issues.

## Keeping pinned upstream versions current

Most of the skills avoid version numbers, but the AWS Lambda rules cannot: a Lambda layer ARN embeds a release version, and an agent that writes a layer ARN has to write a concrete one.
Those pins go stale silently — a wrong version produces a `ResourceNotFoundException` at deploy time, and a stale default-instrumentation list produces missing spans with no error at all.
Refresh them when the upstream project cuts a release, and always before cutting a release of this repository.

The AWS account IDs that publish the layers — `184161586896` for upstream and `115813213817` for Dash0 — are stable and are not part of this review.

| What | Where it is pinned | Source of truth |
|---|---|---|
| Upstream layer release, for example `0_24_0` | `skills/otel-instrumentation/rules/platforms/aws-lambda-otel-layers.md`, `skills/otel-collector/rules/deployment/aws-lambda.md`, the four `*-upstream-layers` example stacks | [opentelemetry-lambda releases](https://github.com/open-telemetry/opentelemetry-lambda/releases) |
| Dash0 extension layer version, for example `26` | `skills/otel-instrumentation/rules/platforms/aws-lambda-dash0-layers.md` and the three `*-dash0-extension` example stacks | [dash0-lambda-extension releases](https://github.com/dash0hq/dash0-lambda-extension/releases) |
| Python instrumentation package version, for example `0.66b1` | the "Tuning the Python instrumentation set" section of `aws-lambda-otel-layers.md` and `examples/lambda/python-upstream-layers/README.md` | `python/src/otel/otel_sdk/requirements.txt` at the layer release tag |
| `aws-lambda-java-events` version | the `javawrapper` requirement in `aws-lambda-otel-layers.md` and `examples/lambda/java-upstream-layers/lambda/pom.xml` | the `opentelemetry-aws-lambda-events-<x.y>` artifact in `java/layer-wrapper/build.gradle.kts` |
| Node.js default-on instrumentation list | the "Tuning the Node.js instrumentation set" section of `aws-lambda-otel-layers.md` | the layer's `wrapper.ts` at the release tag |
| Collector distribution core version | `skills/otel-collector/rules/deployment/aws-lambda.md` | `collector/go.mod` at the layer release tag |

Read the pinned values straight out of the release tag rather than from `main`, because the layer that is published is built from the tag:

```bash
TAG=layer-collector/0.24.0
RAW=https://raw.githubusercontent.com/open-telemetry/opentelemetry-lambda/$TAG

curl -s "$RAW/python/src/otel/otel_sdk/requirements.txt"
curl -s "$RAW/java/layer-wrapper/build.gradle.kts" | grep aws-lambda-events
curl -s "$RAW/collector/go.mod" | grep 'go.opentelemetry.io/collector/otelcol '
```

Two checks catch most of what a version bump breaks, and neither is covered by the Go harness:

1. The Collector exporter and processor type names in `skills/otel-collector/rules/deployment/aws-lambda.md` track the core version, and a renamed type is not a syntax error — the Collector simply fails to start.
   `otlp_grpc` is the canonical OTLP/gRPC exporter type from core `v0.163.0` onward, with `otlp` kept as a deprecated alias.
   Confirm the name against `exporter/otlpexporter/internal/metadata/generated_status.go` at the pinned core version, not against the factory function name, which still reads `otlpexporter.NewFactory()` and misleads.
2. Deploy and invoke the examples as described below.
   A stale layer version fails loudly there; a changed default-instrumentation set does not, so compare the received spans against what the rule claims the layer instruments.

## Running the AWS Lambda examples as a live eval

The Go harness in `evals/custom/` validates the Lambda examples as text only — the Collector configurations carry `<!-- eval:skip -->` because `telemetryapi` and `decouple` do not exist in `otelcol-contrib`, and no fixture deploys a function.
The seven CDK projects under [`examples/lambda/`](examples/lambda/README.md) are the only thing that exercises the guidance against a real Lambda runtime, and they are run by hand.

Running them needs AWS credentials for an account you may create and delete resources in, and a Dash0 auth token with ingest and read scope.
Everything the run creates is public and billable; the teardown step is not optional.

```bash
# 1. Put the auth token in Secrets Manager, so no stack holds it as a literal.
aws secretsmanager create-secret \
  --name agent-skills-test/dash0-token \
  --secret-string "$DASH0_AUTH_TOKEN"

# 2. Point the stacks at the ingress and the dataset.
export DASH0_ENDPOINT=https://ingress.<region>.aws.dash0.com:4318
export DASH0_OTLP_GRPC_ENDPOINT=ingress.<region>.aws.dash0.com:4317
export DASH0_DATASET=agent-skills-test
export DASH0_TOKEN_SECRET_ARN=<arn from step 1>

# 3. Deploy, invoke, and verify one project at a time.
cd examples/lambda/python-upstream-layers
npm install && npx cdk deploy --require-approval never
curl -s "$(aws cloudformation describe-stacks --stack-name PythonUpstreamLayersStack \
  --query 'Stacks[0].Outputs[?OutputKey==`FunctionUrl`].OutputValue' --output text)"
```

Read back what arrived.
The dataset goes in the `Dash0-Dataset` **header** — the API ignores a `dataset` query parameter and falls back to `default`, which fails with a 403 that reads as a token problem:

```bash
curl -s -X POST "https://api.<region>.aws.dash0.com/api/spans" \
  -H "Authorization: Bearer $DASH0_AUTH_TOKEN" \
  -H "Dash0-Dataset: $DASH0_DATASET" \
  -H "Content-Type: application/json" \
  -d '{"timeRange":{"from":"2026-11-01T00:00:00Z","to":"2026-11-01T01:00:00Z"},"sampling":{"mode":"adaptive"},"pageSize":200}'
```

A run passes when, for every project:

- the invocation span is present, with `service.name` set to the logical service rather than the function name, and `faas.name` carrying the function identity;
- the child spans match what the rule says that layer instruments — group them by instrumentation scope, because the scope name is what identifies which instrumentation produced a span;
- resource attributes from `OTEL_RESOURCE_ATTRIBUTES` are on the SDK-produced resource.
  Check every resource in the response, not the first one: the Collector layer's `telemetryapi` receiver builds its own resource that does not carry them, and sampling only the first resource produces a false negative.

Tear down every stack and the secret when the run is finished:

```bash
npx cdk destroy   # in each of the seven project directories
aws secretsmanager delete-secret --secret-id agent-skills-test/dash0-token --force-delete-without-recovery
```

## Adding a scenario

When you add or rename a rule file under `skills/`, register it in the eval registry, as described in [AGENTS.md](AGENTS.md) under "Eval rules" and in [evals/custom/README.md](evals/custom/README.md).
The registry completeness test fails CI on unclassified files, so this is enforced rather than optional.

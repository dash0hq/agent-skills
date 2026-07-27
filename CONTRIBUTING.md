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
`instr-go-logs` and `instr-dotnet-nuget` were quarantined as the `TODO.md` gaps this eval exists to catch (the Go slog bridge and the .NET NuGet install path are undocumented in the skills), but both were un-quarantined once repeated real runs showed the agent handles the gap itself; the remaining skill-doc improvements stay tracked in `TODO.md`.

## Adding a scenario

When you add or rename a rule file under `skills/`, register it in the eval registry, as described in [AGENTS.md](AGENTS.md) under "Eval rules" and in [evals/custom/README.md](evals/custom/README.md).
The registry completeness test fails CI on unclassified files, so this is enforced rather than optional.

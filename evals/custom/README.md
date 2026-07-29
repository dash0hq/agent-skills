# Skill evals

This directory contains the eval harness that checks whether an agent following the skills in this repository produces working telemetry.
Claude Code runs headless against fixture applications, and deterministic assertions on the telemetry received over OTLP decide pass or fail — no LLM judge participates in the verdict.
This README is the operator manual; the plan behind the design lives at [docs/plans/2026-07-17-001-feat-skill-evals-ci-plan.md](../../docs/plans/2026-07-17-001-feat-skill-evals-ci-plan.md).

## Architecture

A scenario run has 4 moving parts:

- **Agent under test** — the pinned Claude Code CLI (see [`versions.env`](./versions.env)) runs headless with `--bare --plugin-dir <repo root>`, so only the skills in this repository are loaded, and edits a temporary copy of a fixture application.
- **Relay** — a dual-homed OTLP relay container is the only egress the running fixture can reach (requirement R21); it forwards telemetry to the sink and requires a per-run bearer token on externally reachable paths.
- **Sink** — an in-process `otelsink` (from [opentelemetry-packaging](https://github.com/open-telemetry/opentelemetry-packaging)) on loopback receives everything the relay forwards; per-run isolation rides on a `test.id` resource attribute.
- **Verdicts** — the harness queries the sink and emits a verdict with a failure class: `infra` (retried up to 3 times, never skill-attributed), `agent-noskill` (no evidence the skill entered context), `agent-build`, `agent-telemetry`, or `agent-assert` (agent-attributable classes retry once).
  Verdict evidence includes the agent transcript and the received telemetry.
  On failure the harness also copies each attempt's transcript (token-scrubbed) into `$EVAL_VERDICT_DIR/transcripts/<scenario>/<attempt>/`, next to the preserved `agent-workspace/`, so it ships with the uploaded CI evidence artifact.

## Running locally

You need Go, Docker, an `ANTHROPIC_API_KEY`, and a `claude` binary on `PATH` (CI pins the version through `CLAUDE_CODE_VERSION` in [`versions.env`](./versions.env); set `EVAL_AGENT_BINARY` to point at a different binary).
All commands run from the `evals/custom/` directory.

Keep the API key in a local `.env` file instead of the command line: copy the repository-root [`.env.example`](../../.env.example) to `.env` in the repository root and set `ANTHROPIC_API_KEY`.
The scenario entrypoints load the repository-root `.env` automatically before running, and a variable already set in the environment always wins, so `ANTHROPIC_API_KEY=... go test ...` still overrides the file.
`.env` is gitignored and never committed.

Harness and unit tests are hermetic — no Docker, network, or secrets:

```bash
go test ./...
```

Run 1 agent scenario end to end (the key comes from `.env`):

```bash
go test ./scenarios -run 'TestScenarios/instr-go-http' -v -timeout 30m
```

Run a selected set with the `EVAL_SCENARIOS` filter (comma-separated scenario IDs):

```bash
EVAL_SCENARIOS=instr-go-http,instr-nodejs-http go test ./scenarios -run TestScenarios -v -timeout 60m
```

Agent scenarios skip cleanly when Docker or the API key is unavailable, so a plain `go test ./...` never invokes the real agent.

## Running example validation

Every fenced code block in `skills/` is extracted, classified, and validated deterministically (requirement R10).
Run it from `evals/custom/`:

```bash
go run ./cmd/validate-examples
```

Add `--dry-run` to print the per-file classification and exemption report without fetching the pinned `otelcol-contrib` binary or validating.

The run opens with a one-line summary that separates what was actually checked from what was exempted or skipped, so a green run cannot quietly overstate itself:

```text
Summary: 96 validated (1 code compiled), 328 exempt (154 bash, 70 code-fragment, 58 bad, 33 skip, 13 not-validated), 32 skipped-no-toolchain, 0 failed.
```

SDK code blocks are classified into complete versus fragment, and only complete blocks are compiled.
Most code blocks in `skills/` are intentional fragments — import snippets, method bodies, and elided examples — so compiling every block would be wrong; a Go block is complete only when it declares a top-level `package`, and the other languages use conservative heuristics that bias to fragment.
Complete Go blocks compile against a pinned OpenTelemetry Go SDK dependency set (`OTEL_GO_CORE_VERSION` and `OTEL_GO_LOG_VERSION` in [`versions.env`](./versions.env)) via the host `go` toolchain; when `go` is absent that compile reports `skipped-no-toolchain` rather than passing silently.
Complete blocks in other languages report `skipped-no-toolchain` too, naming the language, because their fixture-image compilers are a follow-up.
Fragments are reported in the `code-fragment` category and exempted from compilation, never silently dropped.

## Adding a scenario when adding a rule file

The registry test (`Default().Validate(...)` in [`scenarios/scenarios_test.go`](./scenarios/scenarios_test.go)) fails CI when a rule file is unclassified or a dedicated rule file has no scenario, so this workflow is mandatory:

1. Classify the new rule file in `defaultRuleClassification` in [`harness/registry.go`](./harness/registry.go): `dedicated` when edits should select only the scenarios declaring that file, `shared` when edits should select all scenarios of the skill, or `exempt` with a recorded reason.
2. For a `dedicated` file, declare a scenario in [`scenarios/`](./scenarios) that lists the file in its `RuleFiles`, or — only when the scenario belongs to a later implementation unit — add a `pendingScenarios` entry in [`harness/registry.go`](./harness/registry.go) naming that unit.
3. Remove the `pendingScenarios` entry when the scenario lands; the registry test fails on stale entries.
4. For a new fixture language, follow [`fixtures/README.md`](./fixtures/README.md).

## Example annotations

Place an HTML-comment annotation on the line directly above a fence to control validation:

| Annotation | Effect |
| --- | --- |
| `<!-- eval:skip -->` | Exempt from validation; reported as exempt. |
| `<!-- eval:collector-config -->` | Force classification as a complete Collector configuration. |
| `<!-- eval:k8s -->` | Force classification as a Kubernetes manifest. |
| `<!-- eval:fragment -->` | Context-aware: on a `yaml` or untagged block, a service-less Collector fragment wrapped in a generated scaffold before validation; on an SDK-code block, a code fragment reported in the `code-fragment` category and not compiled. |
| `<!-- eval:bad -->` | Deliberately wrong example; exempt and reported in the BAD category. |

Blocks containing a line-comment `BAD` marker (for example `// BAD` or `# BAD`) are auto-exempt without an annotation.
A `yaml` or untagged block that no heuristic or annotation classifies is a validation failure, so new blocks cannot slip in silently.

## Fixture data policy

All fixture data must be obviously synthetic, so leaked telemetry can never be mistaken for real customer data: reserved example domains and `TEST-`-prefixed identifiers, for example `user@example.test` and `TEST-0001`.
The full fixture contract, including the synthetic-data rules, lives in [`fixtures/README.md`](./fixtures/README.md).

## Quarantine

Scenario IDs listed in [`quarantine.yaml`](./quarantine.yaml) never block a gate.
The PR gate skips them entirely (they do not run on PRs and cannot block merges), and the release matrix still runs them for observability but as `continue-on-error` legs, so a failure is neutral and cannot block a release.
Add an entry with the scenario ID, the date, and a link to the tracking issue; remove the entry once the scenario is stable again.

## Bumping pinned versions

[`versions.env`](./versions.env) pins everything the harness shells out to:

- `CLAUDE_CODE_VERSION` — the Claude Code CLI version CI installs.
- `EVAL_MODEL` — the model ID passed to `--model`.
- `OTELCOL_CONTRIB_VERSION` — the `otelcol-contrib` version used by example validation and the Collector scenarios.
- `OTELCOL_CONTRIB_SHA256_LINUX_AMD64`, `OTELCOL_CONTRIB_SHA256_LINUX_ARM64`, and `OTELCOL_CONTRIB_SHA256_DARWIN_ARM64` — SHA-256 checksums of the release archives, copied from the `opentelemetry-collector-releases_otelcol-contrib_checksums.txt` asset on the matching [opentelemetry-collector-releases](https://github.com/open-telemetry/opentelemetry-collector-releases/releases) release.
- `OTEL_GO_CORE_VERSION` — the version of the stable OpenTelemetry Go modules (`go.opentelemetry.io/otel` and friends) used to compile complete Go SDK code blocks.
- `OTEL_GO_LOG_VERSION` — the version of the pre-release OpenTelemetry Go log modules (`go.opentelemetry.io/otel/log` and the log exporters), which track a separate v0.x line.

Bump pins only through a PR: `evals/custom/versions.env` is a full-matrix trigger in the registry, so the PR runs every scenario (requirement R19) and behavior drift surfaces before merge.
When bumping `OTELCOL_CONTRIB_VERSION`, update all 3 checksum keys from the new release's checksums file in the same PR.

## Fork PRs and secrets

The deterministic layers — `go test ./evals/custom/...`, example validation, and fixture builds — need no secrets and run on every PR, including forks and Dependabot, so those PRs still smoke the eval harness itself.
Agent scenarios on pull requests need the Anthropic API key, which exists only as an environment-scoped secret on the `evals` GitHub Actions environment, never as a repository-level secret, for that workflow (requirement R14).
Fork and Dependabot PRs never run agent scenarios: the agent executes PR-authored content, so binding the key to it — even behind an approval prompt — would let that content exfiltrate the secret.
The scenario jobs therefore skip on fork and Dependabot PRs (an `if:` guard on `github.event.pull_request.head.repo.fork` and `github.actor`), and the `evals-gate` accepts that skip; their agent-scenario coverage comes from the release run instead, which re-runs the full matrix against the merged commit each time a release is cut.

Changes to `skills/**` and `evals/custom/scenarios/` are prompt-bearing and require review by the code owners in [`.github/CODEOWNERS`](../../.github/CODEOWNERS); the `@dash0hq/agent-skills-maintainers` team must exist in the `dash0hq` organization with write access for that gate to take effect.

## CI

2 workflows wire the harness into GitHub Actions; scenario IDs are never hardcoded in workflow YAML — every matrix comes from [`cmd/select-scenarios`](./cmd/select-scenarios).

- [`evals-pr.yml`](../../.github/workflows/evals-pr.yml) runs on every pull request with no path filter: unit tests, example validation, and fixture image builds run unconditionally without secrets, `select-scenarios --gate pr` maps the diff to scenarios (quarantined IDs excluded), per-scenario matrix jobs run the agent, kind scenarios run in a dedicated job that provisions the cluster, and the single `evals-gate` job aggregates everything fail-closed.
- [`release.yml`](../../.github/workflows/release.yml) resolves the HEAD SHA once at dispatch, runs the full matrix (including quarantined and Kubernetes scenarios) against that SHA through the reusable [`evals-matrix.yml`](../../.github/workflows/evals-matrix.yml), verifies with a `tessl tile publish` dry run that no `evals/` or `docs/` path would be published, and only then publishes and tags.

There is no nightly full-matrix run.
There used to be one (`evals-nightly.yml`, on a 03:17 UTC cron); it was removed because its jobs bound the `evals` environment, and a required-reviewer protection rule on that environment left every unattended scheduled run stuck in `waiting` forever — nobody was present at 03:17 UTC to approve it — and turned every release dispatch into a manual approval click as well.
The release workflow is now the sole full-matrix run, and it does not depend on the `evals` environment or its protection rules at all: `evals-matrix.yml`'s jobs read `ANTHROPIC_API_KEY` as a plain repository secret instead.
In place of the environment gate, `agent-scenarios` and `kind-scenarios` each carry an `if: github.event_name == 'workflow_dispatch'` guard, so they fail closed if this reusable workflow is ever called from anything other than `release.yml`'s `workflow_dispatch` trigger — no fork PR, Dependabot PR, or other PR-authored content can reach the secret through it.
Do not wire `evals-matrix.yml` into anything that runs on `pull_request`; that path must keep the key environment-scoped, as `evals-pr.yml` does.

2 secrets must exist, both named `ANTHROPIC_API_KEY`:

- An environment-scoped secret on the `evals` GitHub Actions environment, used only by `evals-pr.yml` for same-repo PR runs. Fork and Dependabot PRs never reach it (their scenario jobs skip), so the key is never exposed to PR-authored content; do not create a fork-facing environment that holds it.
- A repository-level secret, used only by `evals-matrix.yml` when called from `release.yml`, per the reasoning above.

Branch protection requires exactly 1 check: `evals-gate`.
It runs with `if: always()` and fails unless every needed job succeeded or was deliberately deselected (the select job succeeded with a scenario count of 0), so a skipped check can never pass vacuously.

Bootstrap in this order:

1. Create the `evals` environment with its `ANTHROPIC_API_KEY` secret (no fork-facing environment holds it), and add the separate repository-level `ANTHROPIC_API_KEY` secret used by `release.yml`.
2. Land the CI pull request while `evals-gate` is not yet a required check.
3. Dispatch the spike workflow, then a manual `release.yml` run, to prove the runner topology end to end.
4. Flip `evals-gate` to required in branch protection.

## Packaging

Published artifacts exclude this directory (requirement R20): the Tessl tile through the repository-root `.tesslignore`, as documented in [RELEASE.md](../../RELEASE.md).
Installs that clone the git repository (the Claude Code and Cursor plugins, and the Gemini CLI extension) have no exclusion mechanism and include `evals/` as inert content; see [RELEASE.md](../../RELEASE.md) for the verified details.

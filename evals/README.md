# Evals

This directory holds two independent evaluation systems for the skills in this repository.

## `custom/` — telemetry harness

The Go eval harness runs Claude Code headless against fixture applications and asserts on the OpenTelemetry telemetry that reaches an OTLP sink.
Deterministic assertions decide pass or fail; no LLM judge participates in the verdict.
See [custom/README.md](./custom/README.md) for the operator manual.

## `tessl/` — Tessl scenarios

Tessl-format scenarios, one per subdirectory, each with a `task.md` (the task prompt) and a `criteria.json` (a weighted-checklist rubric).
Tessl scores these with an LLM-judged rubric at publish time, and the results appear on the [Tessl registry](https://tessl.io/registry/dash0/agent-skills).
The release workflow points the publisher at this directory with `--eval-scenarios evals/tessl`.
See [tessl/README.md](./tessl/README.md).

## Publishing

Neither subsystem ships in published artifacts.
The repository-root `.tesslignore` and `.tileignore` exclude the whole `evals/` tree (requirement R20); the Tessl scenarios are still scored because `--eval-scenarios` reads them from the working tree at publish time, not from the packaged tile.

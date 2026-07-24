# Tessl eval scenarios

These are Tessl-format evaluation scenarios for the skills in this repository, one scenario per subdirectory.
Each scenario contains:

- `task.md` — the task prompt handed to the agent under evaluation.
- `criteria.json` — a weighted-checklist rubric whose item scores are summed by Tessl's LLM judge.

Unlike the telemetry harness in [`../custom/`](../custom/README.md), these scenarios are not run by the Go harness and do not emit real telemetry.
Tessl runs them at publish time — the release workflow passes `--eval-scenarios evals/tessl` to the publish command — and surfaces the scores on the [Tessl registry](https://tessl.io/registry/dash0/agent-skills).

Scenario folders must sit directly under this directory (`evals/tessl/<scenario>/`); Tessl does not discover nested subdirectories.

## Editing scenarios

Changes here are prompt-bearing and require code-owner review per [`.github/CODEOWNERS`](../../.github/CODEOWNERS).
Keep task fixtures synthetic: use reserved example domains and `TEST-`-prefixed identifiers, for example `user@example.test` and `TEST-0001`.

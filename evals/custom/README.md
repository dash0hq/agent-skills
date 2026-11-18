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
Summary: 158 validated (54 code compiled), 392 exempt (181 bash, 80 code-fragment, 66 bad, 43 skip, 22 not-validated), 0 skipped-no-toolchain, 0 failed.
```

SDK code blocks are classified into complete versus fragment, and only complete blocks are compiled.
Most code blocks in `skills/` are intentional fragments — import snippets, method bodies, and elided examples — so compiling every block would be wrong; a Go block is complete only when it declares a top-level `package`, a Rust block only when its first line is a `// src/<path>.rs` comment or it declares `fn main`, a JavaScript or TypeScript block only when it has a top-level `import` or `require` or its first line is a `// <path>` comment naming a script file, a Python block only when it has a top-level `import`, `from`, `def`, or `class` line or its first line is a `# <path>.py` comment, a Java block only when it declares a top-level type (`class`, `interface`, `enum`, `record`, or `@interface` at column 0) or its first line is a `// src/main/java/<path>.java` comment, a C# block only when it declares a `namespace` or a top-level type at column 0 or its first line is a `// <path>.cs` comment, a Ruby block only when its first line is a `# <path>.rb` comment or it has a `require` of a library at column 0, a PHP block only when it opens with `<?php`, names its file with a `// <path>.php` comment, or has a top-level namespaced `use` statement, a Scala block only when its first line is a `// src/main/scala/<path>.scala` comment, it declares a top-level `@main def`, or it pairs a top-level `package` or `import` with a top-level `object`, `class`, `trait`, or `enum`, and the other languages use conservative heuristics that bias to fragment.
Complete Go, Rust, JavaScript, TypeScript, Python, Java, C#, Ruby, PHP, and Scala blocks are checked inside a container, as described in the next section; no language compiles with a host toolchain.
Complete blocks in every other language report `skipped-no-toolchain`, naming the language, until a container plug-in is wired for it.
Fragments are reported in the `code-fragment` category and exempted from compilation, never silently dropped.

### Compiling code in containers

A language with a container plug-in (`ContainerLanguage` in [`examples/container.go`](./examples/container.go)) compiles in the image of its eval fixture: the image on the `FROM <image> AS build` line of the fixture Dockerfile, or the only `FROM` line of a single-stage one, so examples and evals share 1 toolchain pin.
The validator assembles 1 project per rule file from that file's complete blocks and runs 1 container for it in 2 phases: a dependency fetch with network access, then the compile check with `--network none`, so third-party build scripts and macros never reach the network.
The package cache is a host directory under `agent-skills-evals-containers` in the user cache directory (override it with `EVAL_CONTAINER_CACHE_DIR`), and `go run ./cmd/validate-examples --container-cache-key` prints the CI cache key for it: the compile images plus a hash of the generated manifests.
Languages whose rule files carry no dependency manifest check against a pinned package set under [`examples/containers/<language>/`](./examples/containers), which Dependabot bumps.

| Language | Fence tags | Fixture image | Dependencies | Check |
| --- | --- | --- | --- | --- |
| Rust | `rust` | `rust-service` | the rule file's `toml` blocks | `cargo check --offline` |
| JavaScript and TypeScript | `javascript`, `typescript` | `nodejs-service` | [`examples/containers/javascript/`](./examples/containers/javascript) | `tsc --noEmit` |
| Python | `python` | `python-service` | [`examples/containers/python/`](./examples/containers/python) | `mypy` |
| Java | `java` | `java-service` | [`examples/containers/java/`](./examples/containers/java) | `mvn --offline compile` |
| Go | `go` | `go-service` | `OTEL_GO_CORE_VERSION` and `OTEL_GO_LOG_VERSION` in [`versions.env`](./versions.env) | `go build` and `go vet` |
| C# | `csharp` | `dotnet-service` | [`examples/containers/csharp/`](./examples/containers/csharp) | `dotnet build --no-restore` |
| Ruby | `ruby` | `ruby-service` | [`examples/containers/ruby/`](./examples/containers/ruby) | `ruby -wc` plus require, constant, and method checks |
| PHP | `php` | `php-service` | [`examples/containers/php/`](./examples/containers/php) | `php -l` and PHPStan level 2 |
| Scala | `scala` | `scala-service` | [`examples/containers/scala/`](./examples/containers/scala) | `sbt compile`, offline |

#### Rust

A Rust rule file becomes 1 crate checked with `cargo fetch`, then `cargo check --offline`:

1. A block whose first line is `// src/<path>.rs` is written to that path, and any other complete block (one declaring `fn main`) to `src/main.rs`.
2. Every `toml` block of the file feeds `Cargo.toml`: lines before any table header and the `[dependencies]`, `[dev-dependencies]`, and `[build-dependencies]` bodies merge per table, other tables such as `[[bin]]` are copied verbatim, and a `[package]` table replaces the generated one (edition 2024).
3. A dependency declared twice with different specifications, or 2 blocks naming the same path, fail the file's blocks.
4. A top-level `src/<name>.rs` module the crate root does not declare is declared automatically, so every written block is checked.
5. Compiler errors are reported on the block they occur in, at the rule-file line; the other blocks of the crate fail with a pointer to it.

Annotate a `toml` block that is not `Cargo.toml` content with `<!-- eval:skip -->`.

#### JavaScript and TypeScript

The JavaScript and TypeScript blocks of a rule file become 1 project, installed with `npm install --ignore-scripts`, then type-checked with `tsc --noEmit` and `checkJs` at non-strict settings:

1. A block whose first line is a `// <path>` comment naming a `.js`, `.mjs`, `.cjs`, `.jsx`, `.ts`, `.mts`, or `.tsx` file is written to that path, and any other complete block to `examples/block_<line>.<ext>`, with a `.jsx` or `.tsx` extension when it contains JSX.
2. The path alias `@/*` resolves to `src/*`, the Next.js default, so name the file of an imported helper with a `// src/<path>.ts` first line.
3. Imports resolve only against the pinned packages; add a package an example needs to [`examples/containers/javascript/package.json`](./examples/containers/javascript/package.json) and regenerate `package-lock.json` with `npm install --package-lock-only` in the `nodejs-service` image.
4. Names a block uses without defining them, such as `app` or `orderId`, are ignored (TS2304 and TS2552); every other diagnostic fails the block, at the rule-file line.

#### Python

The Python blocks of a rule file become 1 project, installed into a virtual environment with `pip install --only-binary=:all:`, then type-checked with `mypy` at default settings plus `check_untyped_defs`:

1. A block whose first line is a `# <path>.py` comment, with every path segment a Python identifier, is written to that path, and any other complete block to `examples/block_<line>.py`.
2. Module names follow the path, so another block imports `# app/telemetry.py` as `app.telemetry`.
3. Imports resolve only against the standard library and the pinned packages; add a package an example needs to [`examples/containers/python/requirements.txt`](./examples/containers/python/requirements.txt), with its transitive dependencies resolved by `pip install --only-binary=:all:` and `pip freeze` in the `python-service` image.
4. Names a block uses without defining them, such as `app` or `order`, are ignored (the `name-defined` error code is disabled); every other diagnostic fails the block, at the rule-file line.

#### Java

The Java blocks of a rule file become 1 Maven project, whose dependencies and plugins are fetched with `mvn dependency:go-offline`, then compiled with `mvn --offline compile` for Java 21:

1. A block whose first line is a `// src/main/java/<path>.java` comment is written to that path, and any other complete block to `src/main/java/<package path>/<Type>.java`, where the package path comes from its `package` declaration and `<Type>` is its `public` top-level type, or its first top-level type when none is `public`.
2. 2 blocks that resolve to the same path fail the file's blocks; give one a distinct `// src/main/java/<path>.java` first line, or declare a different type.
3. Imports resolve only against the JDK and the pinned dependencies; add an artifact an example imports to [`examples/containers/java/pom.xml`](./examples/containers/java/pom.xml) at its latest release.
4. Every `javac` error fails the block it occurs in, at the rule-file line, undefined names included: a complete block declares or imports every type, field, and method it uses, so declare stand-ins such as a `record Order(...)` for application types.
5. No plugin or annotation processor runs in the network phase; the Maven local repository is the cached package directory.

`xml` blocks are not merged into `pom.xml`; the pinned project is the only build configuration.

#### Go

The Go blocks of a rule file become 1 module, resolved with `go mod tidy`, then checked with `go build ./...` and `go vet ./...` under `GOPROXY=off` and `GOTOOLCHAIN=local`:

1. A block whose first line is a `// <path>.go` comment naming a relative path is written to that path, and any other complete block (one declaring a top-level `package`) to `examples/block_<line>/main.go`, so 2 `package main` blocks of 1 rule file never share a directory.
2. Blocks that name files in the same directory form 1 package and must declare the same package name; 2 blocks naming the same path fail the file's blocks.
3. The generated `go.mod` requires the OpenTelemetry Go modules at `OTEL_GO_CORE_VERSION` and `OTEL_GO_LOG_VERSION` from [`versions.env`](./versions.env); `go mod tidy` resolves any other import to its latest version in the network phase.
4. A pinned module whose `go` directive is newer than the Go of the `go-service` build image fails the network phase with `requires go >= <version>`; bump the fixture's `FROM golang:<version> AS build` line, never `GOTOOLCHAIN`.
5. Compiler and vet errors are reported on the block they occur in, at the rule-file line; the other blocks of the module fail with a pointer to it.

The module cache (`GOMODCACHE`) is the CI-cached `packages` directory, and the build cache (`GOCACHE`) stays local.

#### C#

The C# blocks of a rule file become 1 project, restored with `dotnet restore`, then compiled with `dotnet build --no-restore` against the pinned [`Examples.csproj`](./examples/containers/csharp/Examples.csproj), which mirrors the `dotnet-service` fixture: the Web SDK, `net8.0`, and nullable and implicit usings enabled.

1. A block whose first line is a `// <path>.cs` comment is written to that path, and any other complete block (one declaring a `namespace` or a top-level type) to `examples/Block<line>.cs`.
2. Only a block written to `Program.cs` may hold top-level statements, and that file makes the project an executable; without it the project is a library, so top-level statements elsewhere fail with CS8805 or CS8802.
3. All blocks of the rule file share 1 assembly, so every name a block uses must be declared in that block, in another complete block of the file, or in the pinned packages; declare each type once per rule file.
4. Packages resolve only against the pinned set; add a package an example needs to `Examples.csproj` at its latest version, resolved with `dotnet add package <id>` in the `dotnet-service` build image.
5. Compiler errors are reported on the block they occur in, at the rule-file line, and the other blocks of the project fail with a pointer to it; warnings never fail a block.

The container runs as the host user, so the plug-in mounts a generated `/etc/passwd` naming that user: without an entry, the .NET runtime cannot create the named mutex NuGet takes on every `dotnet` command.

#### Ruby

The Ruby blocks of a rule file become 1 project whose gems are downloaded as `.gem` files with network access, then installed with `bundle install --local` and checked by [`examples/containers/ruby/check.rb`](./examples/containers/ruby/check.rb) with `--network none`, so native extensions build offline.
Ruby has no standard type checker, and the check never executes example code:

1. A block whose first line is a `# <path>.rb` comment is written to that path, and any other complete block to `examples/block_<line>.rb`.
2. `ruby -wc` parses every file; a syntax error fails the block, and warnings are printed but do not fail it.
3. Every top-level `require '<library>'` with a literal argument must load against the pinned [`examples/containers/ruby/Gemfile`](./examples/containers/ruby/Gemfile) or the standard library; requiring runs library code, never example code.
4. Every constant path whose root a required library defines, such as `OpenTelemetry::Trace::Status`, must resolve, and every method called on it, such as `OpenTelemetry::Trace::Status.error(...)`, must exist and accept the call's positional and keyword arguments, unless the file guards the call with `<Constant>.respond_to?(:<method>)`.
5. Names rooted elsewhere, such as `order`, `logger`, `Rails`, or a class the block defines, are not checked, and neither are methods called on local variables or block parameters, such as `c.use_all` in `OpenTelemetry::SDK.configure do |c|`.

Add a gem an example requires to the `Gemfile` with an exact version and regenerate `Gemfile.lock` with `bundle lock --add-platform ruby x86_64-linux aarch64-linux` in the `ruby-service` image.
RBS and Steep were not adopted: the OpenTelemetry gems ship no type signatures, so a type checker would only add failures on the application state that examples leave undefined.

#### PHP

The PHP blocks of a rule file become 1 project, installed with `composer install --no-scripts --no-plugins`, then linted with `php -l` and analysed with PHPStan at level 2; no example code runs:

1. A block whose first line, or whose line after a bare `<?php` line, is a `// <path>.php` comment is written to that path, and any other complete block to `examples/block_<line>.php`.
2. A block without a `<?php` tag gets `<?php ` prepended on its first line, so reported lines stay the block's.
3. The `php-service` image has no Composer, so the fetch phase runs in the image the fixture Dockerfile copies its `composer` binary from (`COPY --from=<image>`); the check runs in the fixture image.
4. Imports resolve only against the pinned packages; add a package an example needs to [`examples/containers/php/composer.json`](./examples/containers/php/composer.json) with an exact version and regenerate `composer.lock` with `composer update --lock --no-scripts --no-plugins` in that Composer image.
5. Level 2 reports unknown classes, functions, constants, methods, and properties without type checks; undefined variables such as `$order` (`variable.undefined`) and unqualified functions an example calls without defining them, such as `saveOrder()` (`function.notFound`), are ignored, and every other error fails the block, at the rule-file line.

#### Scala

The Scala blocks of a rule file become 1 sbt build in the `scala-service` image (the fixture's Scala and sbt versions), resolved with `sbt update`, then compiled with `sbt compile` offline:

1. A block whose first line is `// src/main/scala/<path>.scala` is written to that path in the root project, and such blocks may reference each other.
2. Any other complete block is written to `examples/block_<line>/src/main/scala/Example.scala`, in a subproject of its own, so 2 standalone blocks may both declare `object OrderService`.
3. Imports resolve only against the libraries pinned in [`examples/containers/scala/build.sbt`](./examples/containers/scala/build.sbt); add a library an example needs there at its latest Maven Central release, resolved inside the `scala-service` image.
4. `Not found` errors (E006) for names a block uses without defining them, such as `Order`, `saveOrder`, or `logger`, are ignored unless they occur on an `import` line; every other compiler error fails the block, at the rule-file line.
5. sbt build definitions, such as a block starting with `// build.sbt`, are fragments and are not compiled.

Dependabot has no sbt ecosystem, so bump the Scala pins by hand.

Without Docker, the fixture Dockerfile, or the image, complete blocks of container languages report `skipped-no-toolchain` locally.
CI fails closed instead: the `validate-examples` job passes `--require-containers` (or set `EVAL_REQUIRE_CONTAINERS=1`), which makes any of those a failure.

`dockerfile` blocks are linted for the `ENV` and `LABEL` `name=value` contract: the legacy space-separated form fails validation because the classic (non-BuildKit) Docker builder rejects values containing spaces at parse time ([issue #120](https://github.com/dash0hq/agent-skills/issues/120)).
The same check gates every scenario's fixture workspace before `docker build`, so an agent-written Dockerfile using the legacy form fails the attempt even on BuildKit-backed hosts that would accept it.

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
- `EVAL_MODEL` — the default model ID passed to `--model`.
  A non-empty `EVAL_MODEL` environment variable overrides it, which is how the `model` dropdown of a manually dispatched `evals-matrix.yml` or `release.yml` run reaches the harness.
  The pull-request gate has no dropdown and always uses the pin.
- `OTELCOL_CONTRIB_VERSION` — the `otelcol-contrib` version used by example validation and the Collector scenarios.
- `OTELCOL_CONTRIB_SHA256_LINUX_AMD64`, `OTELCOL_CONTRIB_SHA256_LINUX_ARM64`, and `OTELCOL_CONTRIB_SHA256_DARWIN_ARM64` — SHA-256 checksums of the release archives, copied from the `opentelemetry-collector-releases_otelcol-contrib_checksums.txt` asset on the matching [opentelemetry-collector-releases](https://github.com/open-telemetry/opentelemetry-collector-releases/releases) release.
- `OTEL_GO_CORE_VERSION` — the version of the stable OpenTelemetry Go modules (`go.opentelemetry.io/otel` and friends) used to compile complete Go SDK code blocks in the `go-service` build image; its `go` directive must not exceed that image's Go version.
- `OTEL_GO_LOG_VERSION` — the version of the pre-release OpenTelemetry Go log modules (`go.opentelemetry.io/otel/log` and the log exporters), which track a separate v0.x line.

Bump pins only through a PR: `evals/custom/versions.env` is a full-matrix trigger in the registry, so the PR runs every scenario (requirement R19) and behavior drift surfaces before merge.
When bumping `OTELCOL_CONTRIB_VERSION`, update all 3 checksum keys from the new release's checksums file in the same PR.
The images that compile container-language examples are not pinned here: they come from the `FROM <image> AS build` line of each fixture Dockerfile, so bumping a fixture's build image also bumps the compiler for that language's examples.

## AWS Lambda scenarios

Lambda scenarios (`RequiresAWS: true`) deploy the agent's work to a real AWS account and judge the spans that reach a Dash0 dataset, instead of the local relay and sink.
They run in a dedicated `aws-scenarios` job in both workflows, never in the plain agent matrix.
The fixture contract lives in [`fixtures/lambda/README.md`](./fixtures/lambda/README.md).

### One-time account setup

1. Deploy [`aws/eval-access.yaml`](./aws/eval-access.yaml) as stack `AgentSkillsEvalsAccess`.
   It creates the function permissions boundary, the CloudFormation execution policy scoped to `AgentSkillsEval*` resources, and the `agent-skills-evals-github` role that GitHub OIDC assumes.
2. Bootstrap CDK with the scoped qualifier and execution policy:

   ```bash
   npx cdk bootstrap aws://<account>/<region> \
     --qualifier agskeval --toolkit-stack-name CDKToolkit-agskeval \
     --cloudformation-execution-policies arn:aws:iam::<account>:policy/AgentSkillsEvalCfnExecution
   ```

3. Store the Dash0 auth token as a plain-string secret named `agent-skills-evals/dash0-token`.
4. Set the repository variables `AWS_EVALS_ROLE_ARN`, `AWS_EVALS_REGION`, `DASH0_EVAL_OTLP_HTTP_ENDPOINT`, `DASH0_EVAL_API_ENDPOINT`, `DASH0_EVAL_DATASET`, and `DASH0_EVAL_TOKEN_SECRET_ARN`.
5. Set the secret `DASH0_EVAL_AUTH_TOKEN` twice, like `ANTHROPIC_API_KEY`: as a repository secret for the release matrix, and on the `evals` environment for the PR gate.
   The harness uses it only for the read-back query and removes it from the agent's environment.

### Running locally

You need AWS credentials for the account (for example `AWS_PROFILE`), plus `npx`, `aws`, and the [Dash0 CLI](https://github.com/dash0hq/dash0-cli) on `PATH`.
Run from `evals/custom/`:

```bash
EVAL_AWS_REGION=eu-west-1 \
EVAL_DASH0_OTLP_ENDPOINT=https://ingress.eu-west-1.aws.dash0-dev.com \
EVAL_DASH0_API_URL=https://api.eu-west-1.aws.dash0-dev.com \
EVAL_DASH0_DATASET=agent-skills-test \
EVAL_DASH0_TOKEN_SECRET_ARN=<secret ARN> \
DASH0_AUTH_TOKEN=<token> \
go test ./scenarios -run 'TestLambdaScenarios/lambda-nodejs-dash0-layer' -v -timeout 40m
```

The test skips when any of these variables or tools is missing.
Each run deploys a stack named `AgentSkillsEval-<runtime>-<suffix>` and destroys it when the test finishes; delete leftover `AgentSkillsEval-*` stacks by hand after an interrupted run.

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

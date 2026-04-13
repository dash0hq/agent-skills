# Dash0 Agent Skills

A collection of skills for AI coding agents to make applications observable with OpenTelemetry and [Dash0](https://www.dash0.com).
Skills are packaged instructions and scripts that extend agent capabilities, following the [Agent Skills](https://agentskills.io/) format.

## 🚀 How to use

**Install with [skills](https://skills.sh/) CLI** (universal, works with any [Agent Skills](https://agentskills.io)-compatible tool):

```bash
npx skills add https://github.com/dash0hq/agent-skills --all
# or a single skill:
npx skills add https://github.com/dash0hq/agent-skills --skill otel-semantic-conventions
```

<details>
<summary>Claude Code</summary>

```bash
/plugin marketplace add dash0hq/agent-skills
/plugin install dash0-agent-skills@dash0hq
```

</details>

<details>
<summary>Gemini CLI</summary>

```bash
gemini extensions install https://github.com/dash0hq/agent-skills
```

Update with `gemini extensions update dash0-agent-skills`.

</details>

<details>
<summary>Cursor</summary>

Copy skills into the cross-client discovery directory:

```bash
git clone https://github.com/dash0hq/agent-skills.git  ~/.cursor/skills/dash0-agent-skills
```

Cursor auto-discovers skills from `.agents/skills/` and `.cursor/skills/`.

</details>

<details>
<summary>Copilot</summary>

Copy skills into the cross-client discovery directory:

```bash
/plugin install https://github.com/dash0hq/agent-skills
# or
git clone https://github.com/dash0hq/agent-skills.git ~/.copilot/skills/dash0-agent-skills
```

Copilot auto-discovers skills from `.copilot/skills/`.

</details>

<details>
<summary>OpenCode</summary>

Copy skills into the cross-client discovery directory:

```bash
git clone https://github.com/dash0hq/agent-skills.git ~/.agents/skills/dash0-agent-skills
```

OpenCode auto-discovers skills from `.agents/skills/`, `.opencode/skills/`, and `.claude/skills/`.

</details>

<details>
<summary>Codex (OpenAI)</summary>

Clone into the cross-client discovery path:

```bash
git clone https://github.com/dash0hq/agent-skills.git ~/.agents/skills/dash0-agent-skills
```

Codex auto-discovers skills from `~/.agents/skills/` and `.agents/skills/`. Update with `git -C ~/.agents/skills/dash0-agent-skills pull`.

</details>

<details>
<summary>Antigravity</summary>

Clone and symlink into the cross-client discovery path:

```bash
git clone https://github.com/dash0hq/agent-skills.git ~/.antigravity/skills/dash0-agent-skills
```

Update with `git -C ~/.antigravity/skills/dash0-agent-skills pull`.

</details>

<details>
<summary>Openclaw</summary>

Copy skills into the cross-client discovery directory:

```bash
git clone https://github.com/dash0hq/agent-skills.git ~/.openclaw/skills/dash0-agent-skills
# or in workspace:
git clone https://github.com/dash0hq/agent-skills.git ~/.openclaw/workspace/skills/dash0-agent-skills
```

</details>

## Usage

Skills are automatically available once installed. The agent will use them when relevant tasks are detected.

**Examples:**
```
Add OpenTelemetry instrumentation to my app
```
```
My traces are broken — spans show up as separate roots instead of a connected trace
```
```
Set up an OpenTelemetry Collector pipeline that forwards to Dash0
```
```
Write an OTTL expression to redact credit card numbers from log bodies
```
```
Ensure that my HTTP server spans have the correct attributes
```
```
Help me fix high-cardinality metrics that are blowing up my costs
```

## Why semantic conventions matter

[OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/) define standardized names, types, and semantics for telemetry attributes, metric names, span names, and status codes.
Following them is the single highest-leverage thing you can do for observability quality.

When instrumentation follows semantic conventions:
- Auto-instrumentation libraries, dashboards, and alerting rules work out of the box.
- Service maps, operation grouping, and error tracking derive correct results without manual configuration.
- Cross-service queries return consistent results because every service speaks the same attribute language.

When conventions are missing or inconsistent, these capabilities degrade silently — no errors, just incomplete data, broken topology views, and fragmented queries.

## Instrumentation score

Some of the guidance in these skills is aligned with the [Instrumentation Score](https://github.com/instrumentation-score/spec) specification — a vendor-neutral corpus of guidance that quantifies how well a service follows OpenTelemetry best practices.
The spec defines impact-weighted rules across resources, spans, metrics, and logs.
Following the guidance in these skills helps your services score higher, which directly translates to better observability outcomes.

## Available skills

### [otel-instrumentation](./skills/otel-instrumentation/SKILL.md)

Expert guidance for implementing high-quality, cost-efficient OpenTelemetry telemetry.
Covers backend and browser instrumentation across multiple languages.

**Use when:**
- Setting up observability for a new service
- Adding traces, metrics, or logs to an application
- Debugging instrumentation issues
- Optimizing telemetry costs (cardinality, sampling)
- Connecting browser traces to backend traces

**Rules covered:**
- Telemetry (signal overview and correlation)
- Resources (service identity, environment, Kubernetes attributes)
- Metrics (instrument types, naming, units, cardinality)
- Logs (structured logging, severity, trace correlation)
- Node.js (auto-instrumentation, environment variables, Kubernetes)
- Go (SDK setup, instrumentation libraries, context propagation)
- Python (auto-instrumentation, Flask, Django, FastAPI)
- Java (javaagent, Spring Boot, JVM system properties)
- .NET (auto-instrumentation, ASP.NET Core, ActivitySource)
- Ruby (SDK setup, Rails, Sinatra)
- PHP (auto-instrumentation, Laravel, Symfony)
- Browser (Dash0 SDK, OpenTelemetry JS, server correlation)
- Next.js (App Router, full-stack instrumentation, common gotchas)

**Platforms:**
- Node.js (Express, Fastify, NestJS, etc.)
- Go (net/http, gin, echo, fiber, etc.)
- Python (Flask, Django, FastAPI, etc.)
- Java (Spring Boot, Servlet, JAX-RS, etc.)
- .NET (ASP.NET Core, Entity Framework, etc.)
- Ruby (Rails, Sinatra, etc.)
- PHP (Laravel, Symfony, etc.)
- Browser (React, Vue, Next.js, etc.)
- Dash0 or any OTLP-compatible backend

### [otel-semantic-conventions](./skills/otel-semantic-conventions/SKILL.md)

Expert guidance for selecting, applying, and reviewing OpenTelemetry semantic conventions — the standardized names, types, and semantics for telemetry attributes, span names, and status codes.

**Use when:**
- Choosing attributes for spans, metrics, or logs
- Naming spans or selecting span kinds
- Mapping HTTP status codes to span status
- Reviewing telemetry for semantic convention compliance
- Migrating from old to new attribute names
- Understanding Dash0 derived attributes

**Rules covered:**
- Attributes (registry, selection, placement, common attributes by domain, namespaces)
- Spans (naming patterns, span kind, status code mapping)
- Versioning (stability levels, migration, Dash0 auto-upgrades)
- Dash0 (derived attributes, feature dependencies)

### [otel-collector](./skills/otel-collector/SKILL.md)

Expert guidance for configuring and deploying the OpenTelemetry Collector to receive, process, and export telemetry.
Covers pipeline configuration, deployment patterns, and forwarding to Dash0.

**Use when:**
- Setting up an OpenTelemetry Collector pipeline
- Configuring receivers, processors, or exporters
- Deploying the Collector to Kubernetes or Docker
- Forwarding telemetry to Dash0 or another OTLP backend
- Tuning Collector performance (memory, batching, queuing)

**Rules covered:**
- Receivers (OTLP, Prometheus, filelog, hostmetrics)
- Exporters (OTLP/gRPC to Dash0, debug, authentication, retry, queuing)
- Processors (memory limiter, batch, resource detection, Kubernetes attributes, ordering)
- Pipelines (service section, per-signal configuration, connectors, fan-out)
- Deployment (agent vs gateway, DaemonSet, Deployment, Docker Compose, health checks)

### [otel-ottl](./skills/otel-ottl/SKILL.md)

Expert guidance for writing and debugging OpenTelemetry Transformation Language (OTTL) expressions for the OpenTelemetry Collector's transform and filter processors.

**Use when:**
- Writing OTTL expressions to transform, filter, or enrich telemetry
- Redacting sensitive data from spans, metrics, or logs
- Configuring transform or filter processors in the Collector
- Debugging OTTL syntax or runtime errors
- Optimizing Collector pipeline performance

**Capabilities:**
- Transform (modify attributes and values)
- Filter (drop unwanted telemetry)
- Redact (hide sensitive information)
- Enrich (add contextual metadata)
- Convert (change data types and formats)

**Contexts:** resource, scope, span, spanevent, metric, datapoint, log

## Automation with Claude Code

You can configure [Claude Code](https://docs.anthropic.com/en/docs/claude-code) to apply these skills automatically — both in interactive sessions and in headless CI/CD pipelines.

### Project instructions via CLAUDE.md

Add a `CLAUDE.md` file to your repository root with instructions that tell Claude Code when to use the skills.
Claude Code loads this file at the start of every session.

```markdown
# Observability

This project uses OpenTelemetry for observability.
When adding or modifying instrumentation, follow the guidance from the installed `dash0hq/agent-skills` skills.

When working on application code or deployment specs, use the `otel-instrumentation` skill.
When working on Collector configuration, use the `otel-collector` skill.
When choosing or reviewing telemetry attributes, use the `otel-semantic-conventions` skill.
When writing or debugging OTTL expressions, use the `otel-ottl` skill.
```

### Headless mode in CI/CD

Use `claude -p` to run Claude Code non-interactively in a pipeline.
This enables automated instrumentation reviews, skill-guided code generation, and PR checks.

```bash
# Review instrumentation quality on a pull request
claude -p "Review the OpenTelemetry instrumentation changes in this PR. \
  Check for missing context propagation, incorrect span status handling, \
  and semantic convention violations." \
  --allowedTools "Read,Grep,Glob"
```

#### GitHub Actions example

```yaml
name: Instrumentation review

on: [pull_request]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install skills
        run: npx skills add dash0hq/agent-skills

      - name: Review instrumentation
        run: |
          claude -p "Review the OpenTelemetry instrumentation in this PR \
            for correctness and semantic convention compliance. \
            Post your findings as a summary." \
            --allowedTools "Read,Grep,Glob" \
            --output-format json > review.json

      - name: Comment on PR
        run: |
          findings=$(jq -r '.result' review.json)
          gh pr comment "$PR_NUMBER" --body "## Instrumentation review"$'\n\n'"$findings"
        env:
          PR_NUMBER: ${{ github.event.pull_request.number }}
```

## Skill structure

Each skill contains:
- `SKILL.md` - Instructions for the agent
- `rules/` - Focused guidance documents
- `README.md` - Human-readable documentation

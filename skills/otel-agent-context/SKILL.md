---
name: otel-agent-context
description: OpenTelemetry guidance for AI coding agent context-boundary evidence. Use when instrumenting or reviewing telemetry for AI agents, MCP clients, tool search, skills, memory retrieval, prompt assembly, context compaction, or privacy-safe GenAI traces. Covers span events and log records that prove what crossed the agent context boundary without storing raw prompts, secrets, tool results, or memory bodies.
metadata:
  author: dash0
  version: '1.0.0'
---

# OpenTelemetry agent context evidence

This skill governs privacy-safe telemetry for AI coding agent context boundaries.
Use it when an agent, MCP client, harness, plugin, or coding assistant assembles context from instructions, skills, tools, memory, search, compaction, or security scans.

For general application instrumentation, use the [otel-instrumentation](../otel-instrumentation/) skill.
For attribute naming and semantic convention decisions, use the [otel-semantic-conventions](../otel-semantic-conventions/) skill.

## Rules

### Record context-boundary facts as events, not raw prompt logs

Emit span events or structured log records for facts that can be verified without storing the context body.
Do not store raw prompts, raw tool outputs, raw memory bodies, secrets, credentials, customer text, or repository excerpts in telemetry.

Use this minimum event set when the agent can observe the boundary:

| Event | Use when | Required evidence |
|-------|----------|-------------------|
| `context.input.loaded` | A context item is delivered to the model or agent run | Source type, source hash, delivered hash, byte or token bucket, and load reason |
| `context.input.suppressed` | A candidate context item is intentionally not loaded | Source type, source hash, suppression reason, and policy version |
| `context.skill.invoked` | A skill or instruction bundle changes the session behavior | Skill id, trigger source, skill hash, and invocation reason |
| `context.memory.returned` | A memory or retrieval layer returns candidates | Retrieval run id, candidate count, result hashes, and score bucket |
| `context.memory.loaded` | Retrieved memory is actually placed into context | Retrieval run id, selected hashes, and selection policy |
| `context.compaction.completed` | Context is summarized, dropped, or rewritten | Input count, output hash, preserved objective hash, and audit gaps |
| `context.security_scan.completed` | A security scan affects the agent workflow | Scanner id, finding count by severity, redaction policy, and verification status |

Prefer span events when the context action occurs inside an active agent span.
Prefer log records when the action happens outside a single span or must be correlated across multiple components.

```javascript
// GOOD: records a privacy-safe boundary event.
span.addEvent('context.input.loaded', {
  'context.source.type': 'project_instruction',
  'context.source.hash': 'sha256:8f3c...',
  'context.delivered.hash': 'sha256:62ab...',
  'context.byte_count.bucket': '1kb_10kb',
  'context.load.reason': 'session_start',
  'context.policy.version': '2026-05-22'
});

// BAD: stores the instruction body in telemetry.
span.addEvent('context.input.loaded', {
  'context.body': readFileSync('CLAUDE.md', 'utf8')
});
```

### Separate loaded context from decision-relevant context

Do not infer that loaded context was relevant to the final decision.
Only emit decision-relevance evidence when a verifier, evaluator, tool, or harness can justify the outcome.

Use `context.decision.relevance.evaluated` as a derived event, not as a replacement for `context.input.loaded`.
Include counts and hashes, not raw snippets.

```javascript
span.addEvent('context.decision.relevance.evaluated', {
  'context.decision.id_hash': 'sha256:4b91...',
  'context.decision.selected_count': 3,
  'context.decision.suppressed_count': 9,
  'context.decision.input_hashes': ['sha256:8f3c...', 'sha256:ff10...'],
  'context.relevance.outcome': 'supported',
  'context.audit_gap': 'human_review_required'
});
```

If no evaluator exists, record only boundary events.
Do not invent relevance labels from model attention, prompt order, or confidence text.

### Use buckets and hashes for sensitive or high-cardinality values

Hash stable identifiers when correlation is necessary.
Bucket values when exact counts, paths, sizes, or scores could reveal sensitive information or create high-cardinality telemetry.

Use these patterns:

| Raw value type | Telemetry pattern |
|----------------|-------------------|
| File path | `context.source.path_hash` |
| Prompt or instruction text | `context.source.hash` and `context.delivered.hash` |
| Token count | `context.token_count.bucket` |
| Retrieval score | `context.retrieval.score.bucket` |
| Tool arguments | `tool.arguments.hash` |
| Secret scanning finding | `security.finding.fingerprint_hash` |

```python
# GOOD: bounded buckets and hashes.
span.add_event(
    'context.memory.returned',
    {
        'context.retrieval.run_id': 'ret_019ab',
        'context.retrieval.candidate_count': 12,
        'context.retrieval.score.bucket': '0.80_0.89',
        'context.memory.result_hashes': ['sha256:1130...', 'sha256:9a77...'],
    },
)

# BAD: high-cardinality and sensitive values.
span.add_event(
    'context.memory.returned',
    {
        'context.query.raw': 'customer Acme prod incident webhook secret',
        'context.memory.body': 'full memory text here',
        'context.file.path': '/private/work/acme/secrets.md',
    },
)
```

### Preserve trust boundaries between MCP, tools, memory, and the agent run

Correlate component traces with `traceparent`, `span links`, or a run id.
Do not merge server-side MCP spans, memory database spans, and agent context events into one ambiguous operation.

Record which boundary produced the evidence:

```yaml
context.boundary.kind: mcp_tool_definition
context.boundary.producer: mcp_client
context.boundary.consumer: agent_context
context.correlation.run_id: run_01hxy
```

Use separate spans or events for these boundaries:

1. Tool index loaded.
2. Tool search performed.
3. Tool definition loaded.
4. Tool call completed.
5. Tool result selected for context.

This separation lets operators debug “the tool existed,” “the tool was selected,” “the definition entered context,” and “the call result influenced the next step” as different facts.

### Review telemetry before shipping

Reject an implementation if any event stores raw agent context.
Check at least these fields before approving the change:

1. No raw prompt, memory body, tool output, repository excerpt, secret, or customer text appears in spans, logs, or metrics.
2. Every hash includes the algorithm prefix, such as `sha256:`.
3. Every unbounded count, score, path, or payload-size value is either bucketed or justified as safe.
4. Loaded, suppressed, selected, and decision-relevant evidence are represented as different fields or events.
5. The event names remain stable enough for queries and dashboards.
6. The implementation documents any audit gap that telemetry cannot prove.

```javascript
// GOOD: explicit audit gap.
span.addEvent('context.compaction.completed', {
  'context.compaction.input_count': 48,
  'context.compaction.output.hash': 'sha256:b318...',
  'context.objective.before_hash': 'sha256:72cc...',
  'context.objective.after_hash': 'sha256:72cc...',
  'context.audit_gap': 'summary_quality_not_proven'
});
```

## Official documentation

- [OpenTelemetry Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/)
- [OpenTelemetry Logs Bridge API](https://opentelemetry.io/docs/specs/otel/logs/bridge-api/)
- [OpenTelemetry Trace API](https://opentelemetry.io/docs/specs/otel/trace/api/)
- [OpenTelemetry GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/)

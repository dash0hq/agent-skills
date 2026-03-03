# Agent rules

## Audience

The skills in this repository are consumed operationally by agentic frameworks (AI coding agents, copilots, and autonomous developer tools).
Every piece of guidance must be written so that an agent can act on it without human interpretation.

When writing or editing skill content, follow these principles:
- **Be prescriptive, not descriptive.**
  Tell the agent what to do (`Use a Histogram`) rather than explaining concepts (`Histograms capture distributions`).
  Explanations are acceptable only when they inform the decision.
- **Make decisions enumerable.**
  When multiple options exist, provide a numbered decision process, a lookup table, or explicit criteria — not open-ended advice.
- **Include code examples for every actionable rule.**
  An agent generating code needs a template to follow.
  Show both the correct pattern and, where useful, the incorrect one labelled `// BAD`.
- **Avoid subjective conditions.**
  Do not write "if the user wants" or "it is likely that."
  State concrete, testable criteria the agent can evaluate from the code or configuration it has access to.
- **Keep rules self-contained.**
  An agent may load a single rule file without reading the rest of the skill.
  Each file must make sense on its own; use cross-references for detail, not for essential context.

## Skill separation of concerns

### `otel-instrumentation` vs `otel-collector`

These two skills have a strict boundary: **application code vs Collector configuration**.

- **`otel-instrumentation`** covers the application and its deployment: SDK setup, environment variables (`OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`, etc.), resource attributes, instrumentation libraries, and the Kubernetes workload resources and pod specs that deploy the application (e.g., downward API configuration, container env blocks, secret references for auth tokens in Kubernetes).
- **`otel-collector`** covers what happens inside the OpenTelemetry Collector process: receivers, processors (`memory_limiter`, `batch`, `k8sattributes`, `resourcedetection`, `resource`), exporters, pipelines, and Collector deployment manifests (e.g., DaemonSet and Deployment in Kubernetes, Docker Compose).

When content could live in either skill, apply this rule: if the guidance tells an agent how to modify **application code or deployment specs**, it belongs in `otel-instrumentation`; if it tells an agent how to modify **Collector YAML configuration or Collector deployment manifests**, it belongs in `otel-collector`.
Cross-reference the other skill for context, but do not duplicate Collector configuration in `otel-instrumentation` or SDK/pod-spec configuration in `otel-collector`.

## Prose rules

Follow these rules when writing or editing prose in this project.

### Line and Paragraph Structure
- **One sentence per line** (semantic line breaks).
  Each sentence starts on its own line; do not wrap mid-sentence.
- Separate paragraphs with a single blank line.
- Keep paragraphs between 2 and 5 lines (sentences).

### Section headers
Section headers should be written in sentence case, e.g., "This is an example".

### Links
- Use inline Markdown links: `[visible text](url)`.
- Link the most specific relevant term, not generic phrases like "click here" or "this page."

### Code Blocks
- Fence with triple backticks and a language identifier (e.g., ` ```yaml `).
- Use code blocks to provide illustrative examples.

### Punctuation and Typography
- End sentences with full stops.
- Use the **Oxford comma** (e.g., "error status, latency thresholds, rate limits, and so on").
- Use curly/typographic quotes in prose (`"..."`, `'...'`); straight quotes are fine inside code blocks.
- Write numbers as digits and spell out "percent" (e.g., "10 percent", not "10%" or "ten percent").

### OpenTelemetry terminology
- Write span status codes in all-caps: `UNSET`, `OK`, `ERROR`.
  Do not use mixed case like `Ok`, `Error`, or `Unset` in prose.
- Write log severity levels in all-caps: `TRACE`, `DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`, `UNSET`.
- In code examples, follow the casing of the SDK API (e.g., `SpanStatusCode.ERROR`, `SeverityNumber.INFO`).

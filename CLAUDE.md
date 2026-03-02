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

## Prose rules

Follow these rules when writing or editing prose in this project.

### Line and Paragraph Structure
- **One sentence per line** (semantic line breaks).
  Each sentence starts on its own line; do not wrap mid-sentence.
- Separate paragraphs with a single blank line.
- Keep paragraphs between 2 and 5 lines (sentences).

### Section headers
Seaction headers should be written in sentence case, e.g., "This is an example".

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

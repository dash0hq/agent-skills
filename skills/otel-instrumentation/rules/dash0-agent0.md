---
title: "Dash0 Agent0 AI Platform"
impact: HIGH
tags:
  - dash0
  - agent0
  - ai
  - incident-investigation
  - promql
---

# Dash0 Agent0

## What is Agent0?

Agent0 is Dash0's native AI-powered platform featuring five specialized agents designed
to transform how engineers interact with observability data. Each agent is purpose-built
for specific tasks, providing expert assistance across the observability workflow.

## The Five Agents

Agent0 is powered by five specialized agents that work behind the scenes. You don't
need to select or invoke agents manually—just ask your question naturally, and Agent0
automatically routes it to the right agent (or combination of agents) based on your query.

### The Seeker

Detects anomalies and pinpoints root causes across metrics, logs, and traces during
incident investigations.

- Anomaly detection across all signal types
- Root cause analysis with evidence chain
- Cross-service correlation
- Impact assessment

### The Oracle

Explains, translates, and optimizes PromQL queries—helping teams move from guesswork
to precision without deep query language expertise.

- PromQL query explanation in plain English
- Query optimization suggestions
- Natural language to PromQL translation
- Query debugging and error correction

### The Pathfinder

Guides teams through OpenTelemetry instrumentation and service onboarding.

- Instrumentation recommendations
- SDK setup guidance
- Best practices for attribute naming
- Coverage gap identification

### The Threadweaver

Analyzes traces and builds narratives to reveal bottlenecks and service dependencies.

- Trace analysis and summarization
- Bottleneck identification
- Service dependency mapping
- Critical path analysis

### The Artist

Designs custom dashboards and alert rules to visualize complex systems.

- Dashboard design recommendations
- Alert rule creation
- Visualization best practices
- Panel layout optimization

## How to Use

### Accessing Agent0

Press `Cmd+Shift+0` (macOS) or `Ctrl+Shift+0` (Windows/Linux) anywhere in the Dash0 UI
to open Agent0.

### Context-Aware Assistance

Agent0 automatically infers your current context:
- Active filters and time ranges
- Selected services and traces
- Current dashboard or view
- Recent queries and actions

This means you can ask questions like "Why is this slow?" when viewing a trace, and
Agent0 understands which trace you're referring to.

### Follow-Up Questions

Agent0 maintains conversation context, so you can ask follow-up questions:
1. "What errors occurred in the last hour?"
2. "Focus on the checkout service"
3. "Show me a sample trace"
4. "What's causing this error?"

## Example Queries

Ask Agent0 anything related to observability—it will automatically route to the
appropriate agent:

```
"What caused the spike in errors at 2pm?"
"Explain this PromQL query"
"How do I instrument my Go service?"
"What's causing this trace to be slow?"
"Create a dashboard for my API service"
"Find anomalies in the checkout service"
"Write a query for p99 latency by endpoint"
"Show me the dependencies of the user service"
```

## Best Practices

1. **Start with existing context** - Select a trace, filter to a service, or navigate
   to a dashboard before asking questions

2. **Use follow-up questions** - Narrow your investigation iteratively rather than
   asking complex questions upfront

3. **Combine multiple signal types** - Ask Agent0 to correlate across metrics, logs,
   and traces for comprehensive analysis

4. **Review cited sources** - Agent0 provides transparency by citing the data sources
   used in its analysis

## Resources

- [Agent0 Documentation](https://www.dash0.com/documentation/dash0/agent0)
- [Practical Guide to Agent0](https://www.dash0.com/guides/get-the-most-out-of-agent0)
- [Introducing Agent0 Blog](https://www.dash0.com/blog/introducing-agent0-dash0-s-agentic-ai-platform-for-observability)

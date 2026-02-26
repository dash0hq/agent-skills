---
title: "Dash0 Platform Overview"
impact: HIGH
tags:
  - dash0
  - observability
  - ai
  - agent0
  - mcp
---

# Dash0 Platform Guide

Overview of Dash0's AI-powered observability features.

## What is Dash0?

[Dash0](https://dash0.com) is an OpenTelemetry-native observability platform that provides
AI-powered tools to help engineers understand, query, and act on their telemetry data.

## AI-Powered Features

### Agent0

Agent0 is Dash0's native AI platform featuring five specialized agents for incident
investigation, query optimization, and dashboard creation.

**Key Features:**
- Natural language queries across metrics, logs, and traces
- Context-aware assistance based on your current view
- Full transparency with source citations

**Access:** Press `Cmd+Shift+0` (macOS) or `Ctrl+Shift+0` (Windows/Linux) in the Dash0 UI

See [dash0-agent0](./dash0-agent0.md) for detailed documentation.

### MCP Server

The Dash0 MCP Server enables AI coding assistants to query your observability data directly
using natural language.

**Key Features:**
- Connect Claude Code, Cursor, Windsurf, and other AI assistants
- Query errors, services, metrics, and alerts
- Build and execute PromQL queries

See [dash0-mcp-server](./dash0-mcp-server.md) for setup instructions.

## When to Use Which

| Use Case | Recommended Tool |
|----------|------------------|
| Investigating incidents in Dash0 UI | Agent0 |
| Building/optimizing PromQL queries | Agent0 (The Oracle) |
| Understanding traces and bottlenecks | Agent0 (The Threadweaver) |
| Querying telemetry from your IDE | MCP Server |
| Debugging code with observability context | MCP Server |
| Creating dashboards and alerts | Agent0 (The Artist) |
| Getting instrumentation guidance | Agent0 (The Pathfinder) |

## OTLP Integration

### Endpoint Configuration

```yaml
# Dash0 ingress endpoint
endpoint: https://ingress.${REGION}.dash0.com:4317

# Available regions
regions:
  - eu-west-1
  - us-east-1
```

### Authentication

```yaml
# Bearer token authentication
headers:
  authorization: "Bearer ${DASH0_AUTH_TOKEN}"

# Environment variable
OTEL_EXPORTER_OTLP_HEADERS: "authorization=Bearer ${DASH0_AUTH_TOKEN}"
```

### Quick Setup

```bash
# Set environment variables
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
export OTEL_EXPORTER_OTLP_HEADERS="authorization=Bearer your-dash0-token"
export OTEL_SERVICE_NAME="my-service"
```

## Resources

- [Dash0 Documentation](https://www.dash0.com/documentation)
- [Agent0 Guide](https://www.dash0.com/documentation/dash0/agent0)
- [MCP Server Docs](https://www.dash0.com/documentation/dash0/mcp)
- [Integration Hub](https://www.dash0.com/hub/integrations)

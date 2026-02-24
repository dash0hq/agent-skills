# Dash0 MCP Server

## What is MCP?

Model Context Protocol (MCP) is an open protocol that allows AI models to access external
tools and data sources. It provides a standardized way for AI assistants to interact with
APIs, databases, and other services.

## What is the Dash0 MCP Server?

The Dash0 MCP Server is a cloud-hosted interface that enables any MCP-compatible AI assistant
to interact with your observability data. This allows you to query errors, services, metrics,
and alerts using natural language directly from your development environment.

## Setup

### Step 1: Get Your Endpoint

1. Log into [Dash0](https://app.dash0.com)
2. Go to **Settings** → **Endpoints** → **MCP**
3. Copy the endpoint URL for your region

Available regions:
- EU West: `https://mcp.eu-west-1.dash0.com/mcp`
- US East: `https://mcp.us-east-1.dash0.com/mcp`

### Step 2: Create Auth Token

1. Go to **Settings** → **Auth Tokens**
2. Click **Create Token**
3. Select "All permissions" on the datasets you want to query
4. Copy and securely store the token

### Step 3: Configure Your AI Assistant

#### Claude Code

Add to your Claude Code settings (`~/.claude/settings.json`):

```json
{
  "mcpServers": {
    "dash0": {
      "url": "https://mcp.eu-west-1.dash0.com/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_AUTH_TOKEN"
      }
    }
  }
}
```

#### Cursor

Add to Cursor MCP settings (`.cursor/mcp.json` or global settings):

```json
{
  "mcpServers": {
    "dash0": {
      "url": "https://mcp.eu-west-1.dash0.com/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_AUTH_TOKEN"
      }
    }
  }
}
```

#### Windsurf

Add to Windsurf MCP configuration:

```json
{
  "mcpServers": {
    "dash0": {
      "url": "https://mcp.eu-west-1.dash0.com/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_AUTH_TOKEN"
      }
    }
  }
}
```

#### Generic MCP Client

For other MCP-compatible clients, use:
- **URL:** `https://mcp.{region}.dash0.com/mcp`
- **Auth:** Bearer token in Authorization header
- **Protocol:** Server-Sent Events (SSE) transport

## Capabilities

| Capability | Description |
|------------|-------------|
| Error Analysis | Triage errors across logs and spans with context |
| Service Inventory | Access catalogs of services, operations, and metrics |
| Alert Investigation | Review details about failed checks and alerts |
| PromQL Queries | Build and execute complex metric queries |
| RED Metrics | View Rate, Errors, Duration for any service |
| Trace Analysis | Search and analyze distributed traces |
| Log Search | Query logs with filters and aggregations |

## Example Queries

### Error Investigation

```
"What errors occurred in the checkout service in the last hour?"
"Show me the most common error messages in production"
"Find traces with errors in the payment flow"
```

### Performance Analysis

```
"Show me the slowest endpoints for the api-gateway"
"What's the p99 latency for the user service?"
"Find traces taking longer than 5 seconds"
```

### Service Discovery

```
"What services depend on the user-service?"
"List all services sending data to Dash0"
"Show me the RED metrics for the order service"
```

### Query Building

```
"Help me write a PromQL query for p99 latency"
"Create a query to show error rate by endpoint"
"Write a query for request rate grouped by status code"
```

### Alert Investigation

```
"What alerts fired in the last 24 hours?"
"Show me details about the high latency alert"
"Why did the error rate alert trigger?"
```

## Best Practices

1. **Be specific about time ranges** - Include timeframes in your queries for more
   relevant results

2. **Name services explicitly** - Use the exact service name when querying specific
   services

3. **Start broad, then narrow** - Begin with general queries and follow up with
   more specific questions

4. **Combine with code context** - Reference your code when asking about errors
   to get more actionable insights

## Troubleshooting

### Connection Issues

- Verify your endpoint URL matches your Dash0 region
- Check that your auth token is valid and not expired
- Ensure your token has permissions on the datasets you're querying

### No Results

- Verify data exists for the time range you're querying
- Check that the service names match exactly
- Ensure your token has access to the relevant datasets

### Rate Limiting

- The MCP server has rate limits for API calls
- Space out large queries or batch requests
- Contact Dash0 support if you need higher limits

## Resources

- [MCP Server Documentation](https://www.dash0.com/documentation/dash0/mcp)
- [GitHub Repository](https://github.com/dash0hq/mcp-dash0)
- [MCP Protocol Specification](https://modelcontextprotocol.io/)
- [Integration Hub](https://www.dash0.com/hub/integrations)

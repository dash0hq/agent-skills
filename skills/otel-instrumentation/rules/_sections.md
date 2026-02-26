# OpenTelemetry Instrumentation Sections

| # | Section | Prefix | Impact | Description |
|---|---------|--------|--------|-------------|
| 1 | Signal Types | `signal-` | CRITICAL | Core telemetry signal types: spans, metrics, logs |
| 2 | Core Concepts | `core-` | CRITICAL | SDK setup, collector configuration, telemetry quality |
| 3 | Dash0 Integration | `dash0-` | HIGH | Dash0 platform features: Agent0, MCP Server |
| 4 | Node.js Implementation | `nodejs-` | HIGH | Server-side instrumentation for Node.js |
| 5 | Browser Implementation | `browser-` | HIGH | Client-side instrumentation for web applications |

## How to Use

Each rule can be loaded independently based on your needs:

- **New to OTel?** Start with `core-setup` and `signal-spans`
- **Adding metrics?** Load `signal-metrics` and `core-telemetry-quality`
- **Browser app?** Load `browser-sdk-setup` and `browser-auto-instrumentation`
- **Node.js backend?** Load `nodejs-sdk-setup` and `nodejs-auto-instrumentation`
- **Using Dash0?** Load `dash0-overview` for AI features

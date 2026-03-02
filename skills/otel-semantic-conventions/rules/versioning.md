---
title: "Versioning, Stability, and Migration"
impact: MEDIUM
tags:
  - versioning
  - migration
  - stability
  - semconv-upgrades
---

# Versioning

Use the rename table below when writing or reviewing attribute names.
Always use the current (new) name — not the deprecated one.
If you encounter a deprecated name in existing code, replace it with the current name.

## Stability levels

| Level | Meaning |
|---|---|
| **Stable** | Will not change in breaking ways. Safe to depend on. |
| **Experimental** | May change or be removed. Use with caution. |
| **Deprecated** | Replaced by a newer attribute. Migrate away. |

Always use stable attributes.
Only use experimental attributes when no stable alternative exists, and add a code comment noting the attribute is experimental so it can be updated when it stabilizes.

## Key renames to know

| Old Name | New Name |
|---|---|
| `http.method` | `http.request.method` |
| `http.status_code` | `http.response.status_code` |
| `http.url` | `url.full` |
| `http.target` | `url.path` + `url.query` |
| `http.scheme` | `url.scheme` |
| `http.flavor` | `network.protocol.version` |
| `http.user_agent` | `user_agent.original` |
| `http.client_ip` | `client.address` |
| `net.peer.name` | `server.address` |
| `net.peer.port` | `server.port` |
| `net.host.name` | `server.address` |
| `net.host.port` | `server.port` |
| `db.system` | `db.system.name` |
| `db.name` | `db.namespace` |
| `db.statement` | `db.query.text` |
| `db.operation` | `db.operation.name` |
| `deployment.environment` | `deployment.environment.name` |
| `http.server.duration` | `http.server.request.duration` (unit: ms → s) |
| `http.client.duration` | `http.client.request.duration` (unit: ms → s) |

For the full list of attribute changes, see the [common attributes reference](./attributes.md#most-common-span-attributes).

## Dash0 semantic convention upgrades

Dash0 automatically normalizes incoming telemetry to a configured semantic convention version at ingestion time. This means:

- **Attribute renames** are applied automatically (e.g., `http.status_code` → `http.response.status_code`)
- **Attribute relocation** moves attributes to their correct level (e.g., `service.name` from span to resource)
- **Metric names** are standardized (e.g., `http.server.duration` → `http.server.request.duration`)
- **Type conversions** are applied where required by the spec

Dash0 offers three upgrade strategies per dataset:

| Strategy | Behavior |
|---|---|
| **Latest** | Continuously migrates to the most recent semantic convention version |
| **Specific version** | Locks to a stable version (e.g., 1.20.0) for consistency |
| **Disabled** | Preserves raw telemetry without transformation |

This means you can upgrade your instrumentation libraries at your own pace — Dash0 handles the normalization. See [Dash0 Semantic Convention Upgrades](https://www.dash0.com/documentation/dash0/semantic-conventions/opentelemetry-semantic-convention-upgrades) for configuration details.

## References

- [Semantic Conventions Repository](https://github.com/open-telemetry/semantic-conventions) — source YAML models, version tags, and changelogs
- [Dash0 Semantic Convention Upgrades](https://www.dash0.com/documentation/dash0/semantic-conventions/opentelemetry-semantic-convention-upgrades) — automatic convention version management

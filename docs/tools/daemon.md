---
tool: get_daemon_identity
domain: daemon
category: discovery
stability: experimental
read_only: true
---

# `get_daemon_identity`

Returns a bounded identity and compatibility view for the local OpenSVC daemon,
its node, and its cluster.

Implementation:

- business logic: `internal/core/daemon.go`;
- MCP declaration: `internal/tools/daemon.go`.

## When to use it

Use this tool first when the agent needs to confirm which daemon, node, and
cluster it is connected to, or when it needs the agent/API compatibility
versions before selecting another operation.

Do not use it to assess cluster health, enumerate objects, read configuration,
or inspect resource state. Those concerns belong to dedicated tools.

## MCP properties

| Property | Value |
|---|---|
| Title | Get daemon identity |
| Read-only | Yes |
| Destructive | No |
| Open world | No; it contacts only the configured OpenSVC daemon |
| Side effects | None |

Annotations are protocol hints for MCP clients. Authorization is still enforced
by the delegated OpenSVC JWT and the daemon.

## OpenSVC API

```text
GET /api/cluster/status?selector=**
```

The daemon serves cluster status from data cached for up to approximately two
seconds. The result is therefore a point-in-time operational view, not a
transactional snapshot.

The tool finds the local node using `daemon.nodename`. It rejects a response
that does not contain the local node or its agent version.

## Authorization and visibility

Every MCP request must carry an OpenSVC access JWT. The daemon endpoint accepts
`guest` or a higher operational role. Namespace-scoped grants can filter object
data in the underlying status response, but this tool exposes only daemon,
cluster, node, and listener identity fields.

An invalid or expired JWT is rejected by the MCP middleware before the tool is
called. Insufficient OpenSVC grants are rejected by the daemon.

## Input

The tool has no input fields.

```json
{}
```

Unknown input properties are rejected by the generated input schema.

## Output

| Field | Meaning |
|---|---|
| `daemon.nodename` | Name reported by the local daemon |
| `daemon.pid` | Local daemon process identifier |
| `daemon.started_at` | Daemon start timestamp reported by OpenSVC |
| `daemon.routines` | Number of daemon goroutines |
| `cluster.id` | Stable OpenSVC cluster identifier |
| `cluster.name` | Configured cluster name |
| `cluster.nodes` | Configured cluster node names |
| `cluster.quorum` | Whether quorum is enabled in cluster configuration |
| `node.agent_version` | OpenSVC agent version reported by the local node |
| `node.api_version` | Daemon API compatibility version |
| `node.compat_version` | OpenSVC compatibility version |
| `node.is_leader` | Whether the local node reports itself as leader |
| `node.is_overloaded` | Whether the local node reports overload |
| `node.booted_at` | Node boot timestamp reported by OpenSVC |
| `listener.address` | Configured daemon listener address |
| `listener.port` | Configured daemon listener port |

The full `/api/cluster/status` response is deliberately discarded. In
particular, object, instance, resource, heartbeat, and private configuration
payloads are not returned.

Example:

```json
{
  "daemon": {
    "nodename": "node-a",
    "pid": 2610,
    "started_at": "2026-07-10T17:23:35+09:00",
    "routines": 121
  },
  "cluster": {
    "id": "cluster-123",
    "name": "prod",
    "nodes": ["node-a", "node-b"],
    "quorum": true
  },
  "node": {
    "agent_version": "v3.0.0",
    "api_version": 1,
    "compat_version": 2,
    "is_leader": true,
    "is_overloaded": false,
    "booted_at": "2026-07-10T17:23:14+09:00"
  },
  "listener": {
    "address": "::",
    "port": 1215
  }
}
```

## Errors

| Condition | Result |
|---|---|
| Missing, invalid, expired, or non-access JWT | MCP HTTP `401` |
| Valid JWT with insufficient OpenSVC grants | Tool error containing daemon HTTP `403` |
| Daemon unavailable or malformed response | Tool error with transport or decoding context |
| Missing `daemon.nodename` | Tool error; no partial identity is returned |
| Local node absent from cluster status | Tool error; no partial identity is returned |
| Local node missing its agent version | Tool error; no partial identity is returned |

Errors never include the delegated JWT.

## Compatibility

The contract targets the OpenSVC v3 daemon API. Endpoint behavior and RBAC were
verified against OpenSVC `3.0.0-rc21`; compatibility with other v3 builds must
remain covered by representative response tests.

# Daemon Tools

This document describes the OpenSVC Daemon MCP tools for local daemon identity.

Daemon business logic lives in `internal/core/daemon.go`.
MCP tool definitions live in `internal/tools/daemon.go`.

## Tools

### `get_daemon_identity`

Returns identity information reported by the local OpenSVC daemon.

The tool calls:

```text
GET /api/cluster/status?selector=**
```

It finds the local node from `daemon.nodename`, validates that the node and its
agent version are present, and returns only the identity fields required by the
tool contract. The full cluster status payload is not exposed.

The tool has no input parameters.

Example input:

```json
{}
```

Example structured output:

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

Output fields:

```text
daemon
cluster
node
listener
```

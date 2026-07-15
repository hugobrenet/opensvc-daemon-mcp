---
domain: daemon
tools:
  - get_daemon_identity
stability: experimental
---

# Daemon Tools

This document describes tools that identify the local OpenSVC daemon and its
cluster context.

Implementation:

- business logic: `internal/core/daemon.go`;
- MCP definitions: `internal/tools/daemon.go`.

## Tools

### `get_daemon_identity`

Returns a bounded identity and compatibility view for the local daemon, its
node, and its cluster.

Use this tool first to confirm which daemon an agent is connected to and which
OpenSVC agent/API versions it exposes. Do not use it for health assessment,
object inventory, configuration, or resource state.

#### OpenSVC API

```text
GET /api/cluster/status?selector=**
```

The tool selects the local node using `daemon.nodename`. It rejects a response
that does not contain the local node or its agent version. The large object,
instance, resource, heartbeat, and private configuration portions of the daemon
response are discarded.

The endpoint accepts `guest` or a higher operational role. Namespace grants can
filter the underlying object payload, but this tool returns only identity and
compatibility fields.

#### MCP properties

| Property | Value |
|---|---|
| Title | Get daemon identity |
| Read-only | Yes |
| Destructive | No |
| Open world | No; only the configured daemon is contacted |
| Side effects | None |

#### Input example

The tool has no input fields:

```json
{}
```

Unknown properties are rejected by the generated input schema.

#### Lab output example

```json
{
  "cluster": {
    "id": "11111111-2222-3333-4444-555555555555",
    "name": "lab-cluster",
    "nodes": ["lab-node-01"],
    "quorum": false
  },
  "daemon": {
    "nodename": "lab-node-01",
    "pid": 2610,
    "routines": 135,
    "started_at": "2026-07-10T17:23:35.844939787+09:00"
  },
  "listener": {
    "address": "",
    "port": 1215
  },
  "node": {
    "agent_version": "v3.0.0-rc21-0-gc979e4c01",
    "api_version": 0,
    "booted_at": "2026-07-10T17:23:14+09:00",
    "compat_version": 0,
    "is_leader": true,
    "is_overloaded": false
  }
}
```

`listener.address` can be empty when OpenSVC uses its default bind behavior.
Version and timestamp values are reported by the daemon and are not generated
by the MCP.

#### Errors

| Condition | Result |
|---|---|
| Invalid MCP JWT | MCP HTTP `401` |
| Insufficient daemon grants | Tool error containing daemon HTTP `403` |
| Daemon unavailable or malformed response | Tool error with transport or decoding context |
| Missing local nodename, node status, or agent version | Tool error; no partial identity |

Errors never include the delegated JWT.

## Compatibility

Verified against OpenSVC `3.0.0-rc21` `GET /api/cluster/status` behavior.

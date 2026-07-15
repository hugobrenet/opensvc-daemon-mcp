---
domain: cluster
tools:
  - get_cluster_health
stability: experimental
---

# Cluster Tools

This document describes tools that assess the current OpenSVC cluster view.

Implementation:

- business logic: `internal/core/cluster.go`;
- MCP definitions: `internal/tools/cluster.go`.

## Tools

### `get_cluster_health`

Computes a deterministic, point-in-time health assessment for the cluster, its
nodes, and actor objects visible to the delegated caller.

Use it for the first operational diagnosis: leadership, compatibility, frozen
or missing nodes, non-idle monitors, overload, and a bounded list of problematic
objects. Continue with object and instance tools for a focused diagnosis.

This is an MCP-defined assessment derived from OpenSVC fields, not a canonical
health flag returned by the daemon.

#### OpenSVC API and freshness

```text
GET /api/cluster/status?selector=**
```

The daemon serves a cached cluster view. Refreshing that cache does not execute
resource status drivers. Consequently, `healthy=true` means no issue is present
in the visible last-known OpenSVC state; it does not prove that every resource
was probed during this call.

The endpoint accepts `guest` or a higher role. Object summaries cover only
namespaces visible to the delegated JWT. A healthy result makes no assertion
about inaccessible namespaces.

#### Assessment rules

Cluster issues include:

- incompatible nodes;
- a frozen cluster;
- no node reporting itself as leader;
- more than one node reporting itself as leader.

Leader names are sorted lexicographically.

A node is unhealthy when it is missing, has no agent version or monitor state,
has a non-empty monitor state other than `idle`, is frozen, or reports overload.
An unparseable non-empty `frozen_at` is treated conservatively as frozen. The
evaluated node set is the union of configured and reported nodes.

Only objects with an `avail` field are treated as actors. An actor is
problematic when at least one condition holds:

- availability is not `up`, `stdby up`, or `n/a`;
- overall status is `down`, `warn`, `undef`, or `stdby down`;
- a non-empty placement state is neither `optimal` nor `n/a`;
- a non-empty freeze state is not `unfrozen`;
- provisioned state is `false`, `mixed`, or `undef`.

Availability counters use these normalized values:

| Counter | Values |
|---|---|
| `up` | `up`, `stdby up` |
| `down` | `down`, `stdby down` |
| `warn` | `warn` |
| `not_applicable` | `n/a` |
| `other` | Any other value |

Problem objects are sorted by path. At most 100 are returned and
`problem_objects_truncated` indicates whether additional problems were omitted.

The top-level `healthy` field is true only when the cluster has no issue, every
evaluated node is healthy, and no visible actor object is problematic.

#### MCP properties

| Property | Value |
|---|---|
| Title | Assess cluster health |
| Read-only | Yes |
| Destructive | No |
| Open world | No; only the configured daemon is contacted |
| Side effects | None |

#### Input example

```json
{}
```

#### Lab output example

```json
{
  "cluster": {
    "id": "11111111-2222-3333-4444-555555555555",
    "is_compatible": true,
    "is_frozen": false,
    "issues": [],
    "leader_nodes": ["lab-node-01"],
    "name": "lab-cluster"
  },
  "healthy": true,
  "node_summary": {
    "frozen": 0,
    "healthy": 1,
    "missing": 0,
    "non_idle": 0,
    "overloaded": 0,
    "total": 1
  },
  "nodes": [
    {
      "healthy": true,
      "is_frozen": false,
      "is_leader": true,
      "is_overloaded": false,
      "issues": [],
      "monitor_state": "idle",
      "name": "lab-node-01",
      "reported": true
    }
  ],
  "object_summary": {
    "down": 0,
    "not_applicable": 0,
    "other": 0,
    "problems": 0,
    "total": 1,
    "up": 1,
    "warn": 0
  },
  "problem_objects": [],
  "problem_objects_truncated": false
}
```

`problem_objects` is sorted by canonical object path. Node and leader names are
also sorted for deterministic output.

#### Errors

| Condition | Result |
|---|---|
| Invalid MCP JWT | MCP HTTP `401` |
| Insufficient daemon grants | Tool error containing daemon HTTP `403` |
| Daemon unavailable or malformed status | Tool error; no partial assessment |

## Compatibility

Verified against OpenSVC `3.0.0-rc21`. Health rules must be reviewed whenever
OpenSVC adds or changes status values.

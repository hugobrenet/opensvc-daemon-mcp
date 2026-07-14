# Cluster Tools

This document describes the OpenSVC Daemon MCP tools for cluster health.

Cluster business logic lives in `internal/core/cluster.go`.
MCP tool definitions live in `internal/tools/cluster.go`.

## Tools

### `get_cluster_health`

Returns a deterministic health assessment for the cluster, its configured and
reported nodes, and actor objects.

The tool calls:

```text
GET /api/cluster/status?selector=**
```

The tool has no input parameters.

Example input:

```json
{}
```

Cluster-wide issues include incompatible nodes, a frozen cluster, no reported
leader, or multiple reported leaders.

Node issues include missing status data, a missing agent version or monitor
state, a non-idle monitor state, a frozen node, or an overloaded node.

Only actor objects carrying an `avail` field are included in the object health
summary. Availability, overall status, provisioning, freeze, and placement
states are evaluated using explicit OpenSVC values. Non-actor support objects
are ignored by the object health assessment.

Problem objects are sorted by path and limited to 100 entries. The
`problem_objects_truncated` field reports whether more problem objects exist.
The top-level `healthy` field is true only when there are no cluster-wide
issues, every evaluated node is healthy, and no actor object has a problem.

Example structured output:

```json
{
  "healthy": true,
  "cluster": {
    "id": "cluster-123",
    "name": "prod",
    "is_compatible": true,
    "is_frozen": false,
    "leader_nodes": ["node-a"],
    "issues": []
  },
  "node_summary": {
    "total": 1,
    "healthy": 1,
    "missing": 0,
    "frozen": 0,
    "overloaded": 0,
    "non_idle": 0
  },
  "nodes": [
    {
      "name": "node-a",
      "reported": true,
      "healthy": true,
      "monitor_state": "idle",
      "is_leader": true,
      "is_frozen": false,
      "is_overloaded": false,
      "issues": []
    }
  ],
  "object_summary": {
    "total": 1,
    "up": 1,
    "down": 0,
    "warn": 0,
    "not_applicable": 0,
    "other": 0,
    "problems": 0
  },
  "problem_objects": [],
  "problem_objects_truncated": false
}
```

Output fields:

```text
healthy
cluster
node_summary
nodes
object_summary
problem_objects
problem_objects_truncated
```

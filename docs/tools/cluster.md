---
tool: get_cluster_health
domain: cluster
category: diagnostics
stability: experimental
read_only: true
---

# `get_cluster_health`

Computes a deterministic, point-in-time health assessment for the OpenSVC
cluster, its nodes, and the actor objects visible to the delegated caller.

Implementation:

- business logic: `internal/core/cluster.go`;
- MCP declaration: `internal/tools/cluster.go`.

## When to use it

Use this tool to obtain a first operational diagnosis, identify unhealthy or
missing nodes, detect leadership or compatibility issues, and obtain a bounded
list of problematic actor objects.

This is an MCP-defined assessment derived from OpenSVC status fields. It is not
a single canonical health flag returned by the daemon, and it does not replace
object-specific diagnosis.

## MCP properties

| Property | Value |
|---|---|
| Title | Assess cluster health |
| Read-only | Yes |
| Destructive | No |
| Open world | No; it contacts only the configured OpenSVC daemon |
| Side effects | None |

Annotations are protocol hints for MCP clients. The daemon remains responsible
for authorization and namespace filtering.

## OpenSVC API and freshness

```text
GET /api/cluster/status?selector=**
```

The daemon serves cluster status from data cached for up to approximately two
seconds. The result describes one observed state and can change immediately
after the call.

## Authorization and visibility

Every MCP request must carry an OpenSVC access JWT. The daemon endpoint accepts
`guest` or a higher operational role. OpenSVC filters object data according to
the namespaces granted to the JWT subject.

Consequently, `object_summary`, `problem_objects`, and the top-level `healthy`
decision cover only actor objects visible to the caller. A healthy result does
not assert that inaccessible namespaces contain no problems.

## Input

The tool has no input fields.

```json
{}
```

Unknown input properties are rejected by the generated input schema.

## Assessment rules

### Cluster

The following conditions add a cluster issue:

- nodes are reported as incompatible;
- the cluster is frozen;
- no node reports itself as leader;
- more than one node reports itself as leader.

Leader node names are sorted lexicographically.

### Nodes

The evaluated node set is the union of configured nodes and nodes present in
the status response. Node names are sorted lexicographically.

A node is unhealthy when any of these conditions is true:

- a configured node has no status data;
- the agent version is missing;
- the monitor state is missing;
- the normalized monitor state is not `idle`;
- `frozen_at` is a non-zero timestamp;
- the node reports overload.

An unparseable non-empty `frozen_at` value is treated conservatively as frozen.

### Actor objects

Only objects carrying an `avail` field are treated as actor objects. Support
objects without `avail`, such as configuration or secret objects, are ignored.

Availability counters use these normalized values:

| Counter | Values |
|---|---|
| `up` | `up`, `stdby up` |
| `down` | `down`, `stdby down` |
| `warn` | `warn` |
| `not_applicable` | `n/a` |
| `other` | Any other value |

An actor object is considered problematic when at least one condition holds:

- availability is not `up`, `stdby up`, or `n/a`;
- overall status is `down`, `warn`, `undef`, or `stdby down`;
- a non-empty placement state is neither `optimal` nor `n/a`;
- a non-empty freeze state is not `unfrozen`;
- provisioned state is `false`, `mixed`, or `undef`.

Problem objects are sorted by path. At most 100 are returned;
`problem_objects_truncated` indicates that additional problems exist.

### Top-level decision

`healthy` is true only when all these conditions hold:

- there is no cluster-wide issue;
- every evaluated node is healthy;
- no visible actor object is problematic.

## Output

| Field | Meaning |
|---|---|
| `healthy` | Combined result of all documented checks |
| `cluster` | Cluster identity, compatibility, freeze, leaders, and issues |
| `node_summary` | Counts of total, healthy, missing, frozen, overloaded, and non-idle nodes |
| `nodes` | Per-node health details and issue messages |
| `object_summary` | Availability and problem counts for visible actor objects |
| `problem_objects` | Sorted details for up to 100 problematic actor objects |
| `problem_objects_truncated` | Whether more problematic objects were omitted |

All issue strings are deterministic descriptions intended for diagnosis. They
are part of the current experimental contract and may be refined before the
tool becomes stable.

Example healthy result:

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

## Errors

| Condition | Result |
|---|---|
| Missing, invalid, expired, or non-access JWT | MCP HTTP `401` |
| Valid JWT with insufficient OpenSVC grants | Tool error containing daemon HTTP `403` |
| Daemon unavailable or malformed status payload | Tool error with transport or decoding context |

No partial health result is returned when the daemon request or decoding fails.
Errors never include the delegated JWT.

## Compatibility

The contract targets the OpenSVC v3 daemon API. Endpoint behavior and RBAC were
verified against OpenSVC `3.0.0-rc21`. Health rules must be reviewed whenever
OpenSVC adds or changes status values.

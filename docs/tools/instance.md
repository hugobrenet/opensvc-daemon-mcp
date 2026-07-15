---
tool: list_object_instances
domain: instance
category: diagnostics
stability: experimental
read_only: true
---

# `list_object_instances`

Returns a sorted, paginated status and monitor view for the instances of one
exact OpenSVC object.

## When to use it

Use this after `get_object_status` to determine which node instance explains an
aggregate object problem. It exposes availability, monitor state and targets,
orchestration state, leadership, and resource status counts without returning
full configuration or resource payloads.

## MCP properties

The tool is read-only, non-destructive, closed-world, and has no side effects.

## OpenSVC API

```text
GET /api/instance?path=<exact-path>[&node=<node>]
```

OpenSVC filters instances according to the delegated JWT namespace grants.

## Freshness

The endpoint returns each instance's last-known daemon status. It does not
execute the instance `status` action. `updated_at` is the authoritative age
indicator in this contract. External runtime changes may remain invisible until
an explicit or scheduled OpenSVC status refresh updates the daemon dataset.

## Input

| Field | Required | Default | Bounds | Meaning |
|---|---:|---:|---:|---|
| `path` | Yes | — | 512 characters | Exact canonical object path |
| `node` | No | Empty | 255 characters | Exact node filter |
| `limit` | No | 50 | 1..100 | Maximum instances in this page |
| `cursor` | No | Empty | 255 characters | Previous `next_cursor` with unchanged filters |

The cursor is the last node name returned. Pagination is recalculated from the
current daemon inventory and is not snapshot-based.

## Output

Instances are sorted by node. Each record contains instance status timestamps,
monitor state, global and local expectations, orchestration identity and
completion, leader flags, and counts of resource states. The resource summary
normalizes `up`, `stdby up`, `down`, `stdby down`, `warn`, and `n/a`; unknown
values are counted as `other`.

## Errors

Invalid paths, limits, or cursors fail before daemon access. Daemon
authorization, transport, and decoding failures are returned as tool errors.

## Compatibility

Verified against OpenSVC `3.0.0-rc21` `GET /api/instance` behavior.

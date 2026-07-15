---
domain: object
tools:
  - list_cluster_objects
  - get_object_status
stability: experimental
---

# Object Tools

This document describes tools that discover OpenSVC objects and inspect their
aggregate status.

Implementation:

- business logic: `internal/core/object.go` and `internal/core/object_status.go`;
- MCP definitions: `internal/tools/object.go`.

## Tool selection

Use `list_cluster_objects` to discover canonical paths visible to the caller.
Use `get_object_status` after selecting one exact path. Continue with
`list_object_instances` when an aggregate object state requires node-level
diagnosis.

## Tools

### `list_cluster_objects`

Returns a sorted, paginated inventory of object references visible to the
delegated caller. It deliberately omits status, configuration, instance,
resource, schedule, and secret payloads.

#### OpenSVC API

```text
GET /api/object/path?path=<selector>
```

The MCP removes duplicate paths, parses them into stable references, sorts them
by canonical path, and then applies pagination.

OpenSVC returns only paths visible through the caller's namespace grants. An
empty result can mean either no matching object or no visibility; the MCP does
not distinguish these cases because doing so could disclose unauthorized
inventory.

#### MCP properties

This tool is read-only, non-destructive, closed-world, and has no side effects.

#### Input

| Field | Required | Default | Bounds | Meaning |
|---|---:|---:|---:|---|
| `selector` | No | `**` | 512 characters | Native OpenSVC object selector |
| `limit` | No | 100 | 1..200 | Maximum references in this page |
| `cursor` | No | Empty | 1024 characters | Previous `next_cursor` for the same selector |

Common selectors:

| Selector | Meaning |
|---|---|
| `**` | All visible objects |
| `*/svc/*` | All visible service objects |
| `lab/**` | All visible objects in namespace `lab` |
| `lab/svc/redis` | One exact object |

Lab input example:

```json
{
  "selector": "lab/**",
  "limit": 100
}
```

#### Lab output example

```json
{
  "count": 1,
  "objects": [
    {
      "kind": "svc",
      "name": "redis",
      "namespace": "lab",
      "path": "lab/svc/redis"
    }
  ],
  "selector": "lab/**",
  "total": 1,
  "truncated": false
}
```

When `truncated=true`, call the tool again with the same selector and limit and
pass `next_cursor` as `cursor`. Pagination is recalculated from current daemon
inventory and is not snapshot-based.

Path normalization follows OpenSVC conventions:

| Source path | Returned reference |
|---|---|
| `cluster` | namespace `root`, kind `ccfg`, name `cluster` |
| `redis` | namespace `root`, kind `svc`, name `redis` |
| `cfg/app` | namespace `root`, kind `cfg`, name `app` |
| `lab/svc/redis` | namespace `lab`, kind `svc`, name `redis` |
| `lab/` | namespace `lab`, kind `nscfg`, name `namespace` |

Malformed paths returned by the daemon are rejected rather than silently
rewritten or omitted.

### `get_object_status`

Returns a bounded aggregate status, placement, topology, scope, and instance
location view for one exact object.

It does not expose object configuration, resource details, logs, or data stored
in cfg, sec, or usr objects.

#### OpenSVC API and freshness

```text
GET /api/object?path=<exact-path>
```

The MCP requires exactly one returned object. This read does not execute status
drivers: compare `updated_at` with the current time before relying on the state.
Use `list_object_instances` to inspect per-node timestamps and
`refresh_instance_status` when an explicit probe is necessary.

#### MCP properties

This tool is read-only, non-destructive, closed-world, and has no side effects.

#### Lab input example

```json
{
  "path": "lab/svc/redis"
}
```

Wildcard paths are intentionally unsupported.

#### Lab output example

```json
{
  "availability": "up",
  "frozen": "unfrozen",
  "instance_count": 1,
  "instance_nodes": ["lab-node-01"],
  "is_actor": true,
  "object": {
    "kind": "svc",
    "name": "redis",
    "namespace": "lab",
    "path": "lab/svc/redis"
  },
  "orchestrate": "no",
  "overall": "up",
  "placement_policy": "nodes order",
  "placement_state": "optimal",
  "priority": 50,
  "provisioned": "n/a",
  "scope": ["lab-node-01"],
  "topology": "failover",
  "up_instances_count": 1,
  "updated_at": "2026-07-15T14:31:02.780259555+09:00"
}
```

`is_actor` distinguishes svc and vol actors from support objects that do not
publish availability. Scope and instance node names are sorted.

## Errors

| Condition | Result |
|---|---|
| Invalid MCP JWT | MCP HTTP `401` |
| Invisible or unauthorized object | Tool error from the daemon; commonly HTTP `403` or an empty selection |
| Invalid selector, path, limit, or cursor | Tool validation or daemon error |
| Missing object or unexpected selection | Tool error; no partial result |
| Malformed daemon path or response | Tool error with parsing context |

Errors preserve bounded OpenSVC RFC 7807 details and never include the JWT.

## Compatibility

Verified against OpenSVC `3.0.0-rc21` `/api/object/path` and `/api/object`
behavior, including namespace visibility and selector handling.

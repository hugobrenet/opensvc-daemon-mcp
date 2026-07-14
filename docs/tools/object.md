# Object Tools

This document describes the OpenSVC Daemon MCP tools for cluster object
inventory.

Object business logic lives in `internal/core/object.go`.
MCP tool definitions live in `internal/tools/object.go`.

## Tools

### `list_cluster_objects`

Returns a sorted, paginated inventory of OpenSVC object paths visible to the
delegated caller.

The tool calls:

```text
GET /api/object/path?path=<selector>
```

This endpoint returns paths only. Instance configuration, monitor data, and
status payloads are not exposed by this inventory tool. The OpenSVC daemon
applies the grants carried by the delegated JWT before returning paths.

Input fields:

```text
selector  optional OpenSVC object selector; default **; maximum 512 characters
limit     optional page size; default 100; minimum 1; maximum 200
cursor    optional next_cursor from the previous page using the same selector;
          maximum 1024 characters
```

Selector examples:

```text
**                  all visible objects
*/svc/*             all visible service objects
lab/**              all visible objects in the lab namespace
lab/svc/redis       one exact object
```

Example input:

```json
{
  "selector": "lab/**",
  "limit": 100
}
```

Paths are deduplicated and sorted lexicographically before pagination. When
`truncated` is true, pass `next_cursor` as `cursor` in the next call and keep
the same selector. Each path is parsed into a stable object reference. The
special `cluster` path is returned as namespace `root`, kind `ccfg`, and name
`cluster`.

Example structured output:

```json
{
  "selector": "lab/**",
  "total": 1,
  "count": 1,
  "objects": [
    {
      "path": "lab/svc/redis",
      "namespace": "lab",
      "kind": "svc",
      "name": "redis"
    }
  ],
  "truncated": false
}
```

Output fields:

```text
selector
total
count
objects
next_cursor
truncated
```

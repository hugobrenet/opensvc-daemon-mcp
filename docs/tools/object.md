---
tool: list_cluster_objects
domain: object
category: inventory
stability: experimental
read_only: true
---

# `list_cluster_objects`

Returns a sorted, paginated inventory of OpenSVC object paths visible to the
delegated caller.

Implementation:

- business logic: `internal/core/object.go`;
- MCP declaration: `internal/tools/object.go`.

## When to use it

Use this tool to discover canonical object paths before invoking an
object-specific inspection tool, or to build a bounded inventory for a
namespace or object kind.

This tool returns references only. It does not return object status, instance
state, resource information, configuration, schedules, or secret data.

## MCP properties

| Property | Value |
|---|---|
| Title | List cluster objects |
| Read-only | Yes |
| Destructive | No |
| Open world | No; it contacts only the configured OpenSVC daemon |
| Side effects | None |

Annotations are protocol hints for MCP clients. The daemon remains responsible
for authorization and path visibility.

## OpenSVC API

```text
GET /api/object/path?path=<selector>
```

The MCP retrieves the matching visible paths, removes duplicates, parses them
into stable references, sorts them lexicographically by canonical path, and
then applies pagination.

## Authorization and visibility

Every MCP request must carry an OpenSVC access JWT. OpenSVC returns only paths
for namespaces where the subject has `guest`, `operator`, or `admin`, while a
`root` grant can see every path.

An empty result can therefore mean either that no object matches the selector
or that the caller has no visibility on matching namespaces. The tool does not
attempt to distinguish these cases because doing so would leak unauthorized
inventory information.

## Input

| Field | Required | Default | Bounds | Meaning |
|---|---:|---:|---:|---|
| `selector` | No | `**` | Maximum 512 characters | Native OpenSVC object selector |
| `limit` | No | `100` | Minimum 1, maximum 200 | Maximum number of references in this page |
| `cursor` | No | Empty | Maximum 1024 characters | `next_cursor` returned by the previous page for the same selector |

Examples of selectors:

| Selector | Meaning |
|---|---|
| `**` | All visible objects |
| `*/svc/*` | All visible service objects |
| `lab/**` | All visible objects in namespace `lab` |
| `lab/svc/redis` | One exact object path |

Example input:

```json
{
  "selector": "lab/**",
  "limit": 100
}
```

Input size and limit validation happens before contacting the daemon.

## Pagination semantics

The cursor is the last canonical path returned by the previous page. The next
page starts at the first sorted path strictly greater than that value.

When `truncated` is true:

1. call the tool again with the same selector and limit;
2. pass the returned `next_cursor` as `cursor`;
3. stop when `truncated` becomes false.

Pagination is not snapshot-based. The MCP fetches and sorts the current object
inventory on every call, and `total` is recalculated. If objects are added or
removed between pages, results can move between pages; a newly inserted path
that sorts before or at the cursor will not appear in later pages of that
sequence.

## Path interpretation

| Source path | Returned reference |
|---|---|
| `cluster` | namespace `root`, kind `ccfg`, name `cluster` |
| `redis` | namespace `root`, kind `svc`, name `redis` |
| `cfg/app` | namespace `root`, kind `cfg`, name `app` |
| `lab/svc/redis` | namespace `lab`, kind `svc`, name `redis` |
| `lab/` | namespace `lab`, kind `nscfg`, name `namespace` |

Malformed paths returned by the daemon are rejected instead of being silently
rewritten or omitted.

## Output

| Field | Meaning |
|---|---|
| `selector` | Normalized selector used for the daemon request |
| `total` | Current number of unique visible references matching the selector |
| `count` | Number of references returned in this page |
| `objects` | Sorted object references for this page |
| `objects[].path` | Canonical OpenSVC object path |
| `objects[].namespace` | Parsed namespace, or `root` for root-namespace objects |
| `objects[].kind` | Parsed OpenSVC object kind |
| `objects[].name` | Parsed object name |
| `next_cursor` | Cursor for the next page; omitted on the last page |
| `truncated` | Whether more paths currently follow this page |

`objects` is always an array, including when no matching path is visible.

Example:

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

## Errors

| Condition | Result |
|---|---|
| Missing, invalid, expired, or non-access JWT | MCP HTTP `401` |
| `limit` outside 1..200 | Tool validation error before daemon access |
| Selector longer than 512 characters | Tool validation error before daemon access |
| Cursor longer than 1024 characters | Tool validation error before daemon access |
| Invalid selector rejected by OpenSVC | Tool error from the daemon request |
| Malformed path returned by the daemon | Tool error naming the rejected path |
| Daemon unavailable or malformed response | Tool error with transport or decoding context |

Errors never include the delegated JWT.

## Compatibility

The contract targets the OpenSVC v3 daemon API. Endpoint behavior and RBAC were
verified against OpenSVC `3.0.0-rc21`. Selector behavior remains owned by
OpenSVC and must be rechecked when the daemon selector grammar changes.

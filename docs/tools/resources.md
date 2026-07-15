---
domain: resource
tools:
  - list_object_resources
stability: experimental
---

# Resource Tools

This document describes tools that inspect resource status for an OpenSVC
object.

Implementation:

- business logic: `internal/core/resource.go`;
- MCP definitions: `internal/tools/resource.go`.

## Tools

### `list_object_resources`

Returns sorted, paginated resource status records for one exact object.

Use it after `list_object_instances` to identify the resource responsible for
an unhealthy instance. The tool returns status, provisioning, monitoring flags,
restart state, tags, and bounded status messages. It does not expose resource
configuration values, stream logs, or execute resource actions.

#### OpenSVC API and freshness

```text
GET /api/resource?path=<exact-path>[&node=<node>][&resource=<rid>]
```

Resource data comes from the last-known instance status and does not trigger a
driver probe. Use the parent instance `updated_at` from
`list_object_instances` to assess freshness, and refresh that exact instance
first when necessary.

OpenSVC filters resources according to delegated JWT namespace grants. This
tool is read-only, non-destructive, closed-world, and has no side effects.

#### Input

| Field | Required | Default | Bounds | Meaning |
|---|---:|---:|---:|---|
| `path` | Yes | — | 512 characters | Exact canonical object path |
| `node` | No | Empty | 255 characters | Exact node filter |
| `rid` | No | Empty | 255 characters | Resource id or OpenSVC resource match expression |
| `limit` | No | 100 | 1..200 | Maximum resources in this page |
| `cursor` | No | Empty | 1024 characters | Previous `next_cursor` with unchanged filters |

Lab input example:

```json
{
  "path": "lab/svc/redis",
  "node": "lab-node-01",
  "limit": 100
}
```

#### Lab output example

```json
{
  "count": 1,
  "node_filter": "lab-node-01",
  "object": {
    "kind": "svc",
    "name": "redis",
    "namespace": "lab",
    "path": "lab/svc/redis"
  },
  "resources": [
    {
      "is_disabled": false,
      "is_encap": false,
      "is_monitored": false,
      "is_optional": false,
      "is_standby": false,
      "label": "docker redis:7-alpine",
      "logs": [],
      "logs_truncated": false,
      "node": "lab-node-01",
      "provisioned": "n/a",
      "provisioned_at": "0001-01-01T00:00:00Z",
      "restart_remaining": 0,
      "rid": "container#redis",
      "status": "up",
      "tags": ["mcp-test"],
      "type": "container.docker"
    }
  ],
  "total": 1,
  "truncated": false
}
```

Resources are sorted by node, encapsulated node, and resource id. At most 20
status messages are returned per resource; `logs_truncated=true` signals that
additional messages were omitted. Pagination is not snapshot-based, so callers
must preserve all filters between pages.

#### Errors

Invalid paths, filters, limits, or cursors fail before daemon access. Missing
visibility, daemon authorization, transport failures, and malformed responses
are MCP tool errors. Errors preserve bounded RFC 7807 details and never include
the delegated JWT.

## Compatibility

Verified against OpenSVC `3.0.0-rc21` `GET /api/resource` behavior.

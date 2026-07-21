---
domain: resource
tools:
  - get_container_logs
  - list_object_resources
stability: experimental
---

# Resource Tools

This document describes tools that inspect resource status and bounded
container output for an OpenSVC object.

Implementation:

- business logic: `internal/core/resource.go` and
  `internal/core/container_logs.go`;
- MCP definitions: `internal/tools/resource.go`.

## Tools

### `get_container_logs`

Returns bounded recent stdout and stderr output for one exact OpenSVC container
resource. Use it after `list_object_resources` identifies a `container.*`
resource whose workload output is needed for diagnosis.

Do not use this tool for OpenSVC daemon, monitor, or orchestration records; use
`get_instance_logs` for those. The tool performs a finite historical read and
never follows the stream.

#### OpenSVC API and stream behavior

```text
GET /api/node/name/<node>/instance/path/<namespace>/<kind>/<name>/container/log
  ?rid=<container#name>&follow=false&lines=<lines>
Accept: text/event-stream
```

The OpenSVC endpoint declares `text/event-stream`, but its response body is raw
container log bytes rather than SSE `data:` records. The MCP therefore consumes
it as a bounded opaque stream, normalizes it to safe UTF-8 text, and returns one
ordinary structured result.

The endpoint executes the container runtime's finite log command. It does not
run status drivers or change object state. The MCP tool is annotated read-only,
non-destructive, and closed-world. These annotations are client hints; OpenSVC
authorization remains authoritative.

OpenSVC `3.0.0-rc21` requires the global `root` grant for this endpoint.

Container output is application-controlled and may contain credentials,
personal data, or other sensitive values. Request only the smallest useful
line count and do not copy results into unrelated contexts.

#### Input

| Field | Required | Default | Bounds | Meaning |
|---|---:|---:|---:|---|
| `path` | Yes | — | Exact object path | Canonical OpenSVC object path |
| `node` | Yes | — | 255 characters | Exact node hosting the container |
| `resource_id` | Yes | — | Exact `container#…`, 255 characters | Container RID returned by `list_object_resources` |
| `lines` | No | 50 | 1..200 | Maximum recent records requested from the runtime |

Selectors such as `container#*` are rejected. `lines=0` is the omitted Go zero
value and selects 50; it never enables an unbounded read. The daemon may omit
older records according to `lines`, independently of the MCP output bound.

Lab input example:

```json
{
  "path": "lab/svc/redis",
  "node": "lab-node-01",
  "resource_id": "container#redis",
  "lines": 10
}
```

#### Lab output example

```json
{
  "object": {
    "path": "lab/svc/redis",
    "namespace": "lab",
    "kind": "svc",
    "name": "redis"
  },
  "node": "lab-node-01",
  "resource_id": "container#redis",
  "lines": 10,
  "line_count": 3,
  "content": "1:M * Running mode=standalone, port=6379.\n1:M * Server initialized\n1:M * Ready to accept connections tcp",
  "truncated": false
}
```

#### Output and bounds

| Field | Meaning |
|---|---|
| `object` | Canonical object reference |
| `node` | Exact queried node |
| `resource_id` | Exact queried container RID |
| `lines` | Effective maximum requested from OpenSVC |
| `line_count` | Normalized text lines present in `content` |
| `content` | Combined recent container stdout and stderr text |
| `truncated` | Whether the MCP shortened content to its output bound |

Content is valid UTF-8 and limited to 65,536 Unicode code points. NUL, terminal
control, and formatting characters are replaced; line feeds and tabs are
preserved. `truncated=false` does not mean the complete lifetime log was
returned: `lines` still limits the daemon request.

Invalid paths, nodes, RIDs, or line counts fail before daemon access.
Authorization failures, unexpected content types, oversized transport
responses, interrupted streams, and caller cancellation become MCP tool
errors. Daemon RFC 7807 errors remain bounded and never expose the delegated
JWT.

In the current OpenSVC implementation, the daemon sends HTTP `200` and flushes
headers before starting the local container-log command. A runtime failure
after that point can therefore appear as empty or partial successful content;
the MCP cannot reconstruct an HTTP error that the daemon did not send.

### `list_object_resources`

Returns sorted, paginated resource status records for one exact object.

Use it after `list_object_instances` to identify the resource responsible for
an unhealthy instance. The tool returns status, provisioning, monitoring flags,
restart state, tags, and bounded status messages. It does not expose resource
configuration values, container output, or execute resource actions. Continue
with `get_container_logs` only for one exact container RID when workload output
is necessary.

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

Verified against OpenSVC `3.0.0-rc21` `GET /api/resource` and
`GetInstanceContainerLog` behavior.

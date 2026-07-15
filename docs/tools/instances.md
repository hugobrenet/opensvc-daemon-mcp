---
domain: instance
tools:
  - get_instance_logs
  - list_object_instances
  - refresh_instance_status
stability: experimental
---

# Instance Tools

This document describes tools that inspect logs and status, and actively
refresh the status of one OpenSVC object instance.

Implementation:

- business logic: `internal/core/instance.go` and
  `internal/core/instance_logs.go`;
- MCP definitions: `internal/tools/instance.go`.

## Tool selection

Use `list_object_instances` after `get_object_status` to locate the node behind
an aggregate problem and inspect status age. Use `get_instance_logs` to inspect
recent OpenSVC activity for that exact node instance. Use
`refresh_instance_status` only when the selected instance's `updated_at` is too
old for the diagnosis.

## Tools

### `get_instance_logs`

Returns recent OpenSVC daemon and orchestration log records associated with one
exact object instance. Use it to correlate monitor transitions, resource
checks, daemon API activity, and orchestration identifiers after identifying
the affected node with `list_object_instances`.

This tool does not return the stdout or stderr of the Redis process, container,
or another workload. Container output is exposed by a different daemon
endpoint and is outside this tool's contract.

#### OpenSVC API and stream behavior

```text
GET /api/node/name/<node>/instance/path/<namespace>/<kind>/<name>/log?follow=false&lines=<lines+1>
Accept: text/event-stream
```

OpenSVC uses Server-Sent Events for this endpoint even when `follow=false`.
Each historical journal record is delivered as one finite SSE `log` event. The
MCP consumes the stream to EOF and returns ordinary structured JSON; it does
not follow, reconnect, or expose SSE envelopes.

The request reads journal records available when the call starts. It does not
run resource drivers, refresh instance status, or change daemon state. The tool
is read-only, non-destructive, and closed-world.

OpenSVC `3.0.0-rc21` delegates this instance route to its node-log handler,
which requires the global `root` grant. A namespace `guest`, `operator`, or
`admin` grant is not sufficient in this version, even though the outer
instance handler first checks namespace visibility.

#### Input

| Field | Required | Default | Bounds | Meaning |
|---|---:|---:|---:|---|
| `path` | Yes | — | Exact object path | Canonical object path |
| `node` | Yes | — | 255 characters | Exact node hosting the instance |
| `lines` | No | 50 | 1..100 | Maximum recent entries returned |

The MCP requests one extra daemon record to determine whether older entries
were omitted. It then retains at most the requested number of most recent
entries in chronological order.

Lab input example:

```json
{
  "path": "lab/svc/redis",
  "node": "lab-node-01",
  "lines": 3
}
```

#### Lab output example

```json
{
  "count": 3,
  "entries": [
    {
      "component": "daemon/daemonapi",
      "event_id": "b04fe7aa-c70c-4c30-a4a9-e7d2ef477061",
      "level": "info",
      "message": "daemon: api: inet: GET /api/object/path/lab/svc/redis/config/file: serve config file lab/svc/redis to opensvc-daemon-mcp",
      "message_truncated": false,
      "request_id": "7ae41b79-d9fd-4c0b-80c1-9368e88dbd93",
      "session_id": "e24f692e-fd28-4b4a-a606-13e037e03c36",
      "timestamp": "2026-07-15T14:57:09.712819569+09:00"
    },
    {
      "component": "daemon/daemonapi",
      "event_id": "b04fe7aa-c70c-4c30-a4a9-e7d2ef477061",
      "level": "info",
      "message": "daemon: api: ux: GET /api/object/path/lab/svc/redis/config/file: serve config file lab/svc/redis to root",
      "message_truncated": false,
      "request_id": "fd76415e-a716-4fd2-a765-a20f32b9d0cc",
      "session_id": "e24f692e-fd28-4b4a-a606-13e037e03c36",
      "timestamp": "2026-07-15T15:11:08.635069636+09:00"
    },
    {
      "component": "daemon/daemonapi",
      "event_id": "b04fe7aa-c70c-4c30-a4a9-e7d2ef477061",
      "level": "info",
      "message": "daemon: api: ux: GET /api/object/path/lab/svc/redis/config/file: serve config file lab/svc/redis to root",
      "message_truncated": false,
      "request_id": "5af7dc44-923d-48fd-8bb6-a327b1c7a453",
      "session_id": "e24f692e-fd28-4b4a-a606-13e037e03c36",
      "timestamp": "2026-07-15T16:18:11.055054155+09:00"
    }
  ],
  "lines": 3,
  "node": "lab-node-01",
  "object": {
    "kind": "svc",
    "name": "redis",
    "namespace": "lab",
    "path": "lab/svc/redis"
  },
  "truncated": true
}
```

#### Output and bounds

| Field | Meaning |
|---|---|
| `object` | Canonical object reference |
| `node` | Exact queried node |
| `lines` | Effective requested maximum, including the default |
| `count` | Number of entries returned |
| `entries` | Recent entries in chronological order |
| `truncated` | Older entries or message content were omitted |

Each entry can contain `timestamp`, normalized `level`, bounded `message`,
`message_truncated`, `component`, `resource_id`, `session_id`, `event_id`,
`request_id`, and `orchestration_id`. Optional identifiers are omitted when the
record does not provide them.

Messages are limited to 2,048 Unicode code points each and 64 Ki Unicode code
points across the response. Correlation fields are limited to 255 code points.
Control and formatting characters are normalized. Raw journald metadata such
as UID/GID, machine identifiers, systemd fields, and raw grant data is never
returned.

Malformed SSE, unexpected event types, invalid nested OpenSVC JSON, object or
node mismatches, oversized streams, transport failures, and caller
cancellation become MCP tool errors. Daemon `401` and `403` responses preserve
their bounded RFC 7807 details.

### `list_object_instances`

Returns a sorted, paginated status and monitor view for instances of one exact
object. It exposes availability, monitor targets, orchestration state,
leadership, timestamps, and resource status counts without returning full
resource or configuration payloads.

#### OpenSVC API and freshness

```text
GET /api/instance?path=<exact-path>[&node=<node>]
```

This endpoint returns the last-known daemon status and does not execute the
instance `status` action. `updated_at` is the authoritative age indicator.

OpenSVC filters instances according to delegated JWT namespace grants. The tool
is read-only, non-destructive, closed-world, and has no side effects.

#### Input

| Field | Required | Default | Bounds | Meaning |
|---|---:|---:|---:|---|
| `path` | Yes | — | 512 characters | Exact canonical object path |
| `node` | No | Empty | 255 characters | Exact node filter |
| `limit` | No | 50 | 1..100 | Maximum instances in this page |
| `cursor` | No | Empty | 255 characters | Previous `next_cursor` with unchanged filters |

Lab input example:

```json
{
  "path": "lab/svc/redis",
  "node": "lab-node-01",
  "limit": 50
}
```

#### Lab output example

```json
{
  "count": 1,
  "instances": [
    {
      "availability": "up",
      "frozen_at": "0001-01-01T00:00:00Z",
      "global_expect": "none",
      "is_ha_leader": true,
      "is_leader": true,
      "last_started_at": "2026-07-15T13:36:32.905515501+09:00",
      "local_expect": "started",
      "monitor_state": "idle",
      "node": "lab-node-01",
      "orchestration_id": "00000000-0000-0000-0000-000000000000",
      "orchestration_is_done": false,
      "overall": "up",
      "provisioned": "n/a",
      "resource_summary": {
        "down": 0,
        "not_applicable": 0,
        "other": 0,
        "total": 1,
        "up": 1,
        "warn": 0
      },
      "updated_at": "2026-07-15T14:31:02.747625761+09:00"
    }
  ],
  "node_filter": "lab-node-01",
  "object": {
    "kind": "svc",
    "name": "redis",
    "namespace": "lab",
    "path": "lab/svc/redis"
  },
  "total": 1,
  "truncated": false
}
```

Instances are sorted by node. Resource status counters normalize `up`,
`stdby up`, `down`, `stdby down`, `warn`, and `n/a`; other values increment
`other`. Pagination is recalculated from current daemon inventory and is not a
snapshot.

### `refresh_instance_status`

Actively runs the OpenSVC status probe for one exact object instance, waits for
a newer `status.updated_at`, and returns that refreshed instance.

Use it after a read-only status tool when an out-of-band failure or recovery may
not yet be reflected. It never fans out automatically. The tool is
non-destructive but is not read-only or idempotent: every call executes resource
status drivers and updates daemon state.

#### OpenSVC API and authorization

```text
GET  /api/instance?path=<path>&node=<node>
POST /api/node/name/<node>/instance/path/<namespace>/<kind>/<name>/action/status
GET  /api/instance?path=<path>&node=<node>
```

The delegated subject needs `operator`, `admin`, or `root` access for the
namespace. A `guest` JWT can inspect instances but cannot trigger the action.

#### Input

| Field | Required | Default | Bounds | Meaning |
|---|---:|---:|---:|---|
| `path` | Yes | — | 512 characters | Exact canonical object path |
| `node` | Yes | — | 255 characters | Exact node hosting the instance |
| `timeout_seconds` | No | 30 | 5..120 | Maximum polling duration after action acceptance |

Discover the exact node with `list_object_instances`; do not guess it.

Lab input example:

```json
{
  "path": "lab/svc/redis",
  "node": "lab-node-01",
  "timeout_seconds": 30
}
```

#### Integrated workflow

1. Read the exact instance and capture `status.updated_at`.
2. Submit the status action and retain its `session_id`.
3. Poll after 250 ms, then 500 ms, then every second.
4. Stop when `updated_at` changes or the timeout expires.
5. Return the latest observed instance.

The timestamp change is the completion signal because this status action is not
a CRM orchestration and its session is not reliably represented in instance
monitor fields.

#### Lab output example

```json
{
  "current_updated_at": "2026-07-15T14:35:58.608127647+09:00",
  "duration_ms": 251,
  "instance": {
    "availability": "up",
    "frozen_at": "0001-01-01T00:00:00Z",
    "global_expect": "none",
    "is_ha_leader": true,
    "is_leader": true,
    "last_started_at": "2026-07-15T13:36:32.905515501+09:00",
    "local_expect": "started",
    "monitor_state": "idle",
    "node": "lab-node-01",
    "orchestration_id": "00000000-0000-0000-0000-000000000000",
    "orchestration_is_done": false,
    "overall": "up",
    "provisioned": "n/a",
    "resource_summary": {
      "down": 0,
      "not_applicable": 0,
      "other": 0,
      "total": 1,
      "up": 1,
      "warn": 0
    },
    "updated_at": "2026-07-15T14:35:58.608127647+09:00"
  },
  "node": "lab-node-01",
  "object": {
    "kind": "svc",
    "name": "redis",
    "namespace": "lab",
    "path": "lab/svc/redis"
  },
  "previous_updated_at": "2026-07-15T14:31:02.747625761+09:00",
  "refresh_observed": true,
  "session_id": "269b40e1-fe5c-4e61-81d4-aeb4a1629a8f",
  "timed_out": false
}
```

`refresh_observed=true` means a newer timestamp was read. If the accepted action
does not become visible before the deadline, the tool returns structured
success with `timed_out=true`, the accepted `session_id`, and the latest
observed instance. The action may still complete later.

#### Authorization error example

A caller holding only `guest` receives an MCP tool error similar to:

```text
request instance status refresh: OpenSVC daemon POST ... returned HTTP 403 Forbidden: need one of [operator:lab admin:lab operator admin root] grant
```

The message comes from the daemon's bounded RFC 7807 response. The MCP does not
invent or bypass grant decisions.

## Errors

Invalid paths, filters, limits, cursors, nodes, line counts, or timeouts fail
before daemon access where possible. Missing or invisible instances,
authorization failures, transport errors, malformed responses, missing action
session identifiers, and caller cancellation are MCP tool errors. No JWT or
raw error body is exposed.

## Compatibility

Verified against OpenSVC `3.0.0-rc21` `GET /api/instance`, `GetInstanceLogs`,
and `PostInstanceActionStatus` behavior. The status action executes `instance
status -r`.

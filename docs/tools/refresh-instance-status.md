---
tool: refresh_instance_status
domain: instance
category: diagnostics
stability: experimental
read_only: false
---

# `refresh_instance_status`

Actively refreshes the status of one exact OpenSVC object instance and returns
the newest instance state observed within a bounded polling period.

## When to use it

Use this tool when `get_object_status` or `list_object_instances` may be stale,
especially after an out-of-band runtime change. It targets exactly one object
path and one node. It never fans out to every instance automatically.

This tool executes resource status drivers and updates daemon state. It is
non-destructive, but it is not read-only or idempotent.

## MCP properties

| Property | Value |
|---|---|
| Title | Refresh instance status |
| Read-only | No |
| Destructive | No |
| Idempotent | No; every call executes another status probe |
| Open world | No; it contacts only the configured OpenSVC daemon |

Annotations are client hints. OpenSVC authorization is authoritative.

## OpenSVC API and authorization

```text
GET  /api/instance?path=<path>&node=<node>
POST /api/node/name/<node>/instance/path/<namespace>/<kind>/<name>/action/status
GET  /api/instance?path=<path>&node=<node>
```

The delegated subject must have `operator`, `admin`, or `root` access for the
object namespace. A guest JWT can use the read-only diagnostic tools but cannot
trigger this action.

## Input

| Field | Required | Default | Bounds | Meaning |
|---|---:|---:|---:|---|
| `path` | Yes | — | 512 characters | Exact canonical object path |
| `node` | Yes | — | 255 characters | Exact node hosting the instance |
| `timeout_seconds` | No | 30 | 5..120 | Maximum polling duration after action acceptance |

Discover the node with `list_object_instances`; do not guess it.

## Integrated workflow

1. Read the exact instance and capture `status.updated_at`.
2. Submit the status action and retain its `session_id`.
3. Poll after 250 ms, then 500 ms, then every second.
4. Stop when `status.updated_at` changes or the timeout expires.
5. Return the latest observed `ObjectInstanceStatus`.

The timestamp change is the completion signal because this status action is not
a CRM orchestration and its session is not reliably represented in instance
monitor fields.

## Output and timeout semantics

`refresh_observed=true` means a newer status timestamp was read after the POST.
`timed_out=true` means the POST was accepted but no newer timestamp was observed
within the requested period. A timeout is returned as structured success, not
as an MCP error, because the daemon action may still complete later. In that
case, use `list_object_instances` to read the status again.

The output also contains the action `session_id`, previous and current
timestamps, polling duration, and latest instance state.

## Errors

Invalid inputs fail before daemon access. Missing or invisible instances,
insufficient grants, rejected POST requests, missing session identifiers,
transport failures, malformed responses, and caller cancellation are tool
errors. The delegated JWT is never returned or logged.

## Compatibility

Verified against OpenSVC `3.0.0-rc21` `PostInstanceActionStatus`, which requires
operator access and executes `instance status -r`.

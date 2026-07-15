---
tool: list_object_resources
domain: resource
category: diagnostics
stability: experimental
read_only: true
---

# `list_object_resources`

Returns sorted, paginated resource status records for one exact OpenSVC object.

## When to use it

Use this after `list_object_instances` to identify the resource responsible for
an unhealthy instance. Results include status, provisioning, monitoring flags,
automatic restart state, tags, and bounded resource status messages.

The tool does not stream daemon logs, expose resource configuration values, or
perform resource actions.

## MCP properties

The tool is read-only, non-destructive, closed-world, and has no side effects.

## OpenSVC API

```text
GET /api/resource?path=<exact-path>[&node=<node>][&resource=<rid>]
```

OpenSVC filters resources according to delegated JWT namespace grants.

## Freshness

The endpoint reads resource data embedded in the last-known instance status and
does not probe resource drivers. Use the parent instance `updated_at` returned
by `list_object_instances` to assess freshness. Resource state can lag an
out-of-band failure or recovery until OpenSVC refreshes the instance status.

## Input

| Field | Required | Default | Bounds | Meaning |
|---|---:|---:|---:|---|
| `path` | Yes | — | 512 characters | Exact canonical object path |
| `node` | No | Empty | 255 characters | Exact node filter |
| `rid` | No | Empty | 255 characters | Resource id or OpenSVC resource match expression |
| `limit` | No | 100 | 1..200 | Maximum resources in this page |
| `cursor` | No | Empty | 1024 characters | Opaque previous `next_cursor` with unchanged filters |

Pagination is sorted by node, encapsulated node, and resource id. It is not a
snapshot; callers must preserve all filters across pages.

## Output

Each resource record contains node identity, rid, driver type, label, state,
provisioning, disabled/monitored/optional/standby/encap flags, subset, sorted
tags, restart counters, and at most 20 status messages. `logs_truncated` reports
when additional messages were omitted.

## Errors

Invalid paths, filters, limits, or cursors fail before daemon access. Daemon
authorization, transport, and decoding failures are returned as tool errors.

## Compatibility

Verified against OpenSVC `3.0.0-rc21` `GET /api/resource` behavior.

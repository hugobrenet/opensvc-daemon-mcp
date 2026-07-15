---
tool: get_object_status
domain: object
category: diagnostics
stability: experimental
read_only: true
---

# `get_object_status`

Returns a bounded aggregate status and placement view for one exact OpenSVC
object.

## When to use it

Use this after `list_cluster_objects` to confirm an object's availability,
placement, provisioning, topology, scope, and instance locations. Continue with
`list_object_instances` when the aggregate state requires node-level diagnosis.

The tool does not return object configuration, resource details, logs, or data
stored in cfg, sec, or usr objects.

## MCP properties

The tool is read-only, non-destructive, closed-world, and has no side effects.
Annotations are client hints; authorization remains enforced by OpenSVC.

## OpenSVC API

```text
GET /api/object?path=<exact-path>
```

The MCP requires exactly one returned object and rejects an empty or unexpected
selection. The delegated JWT controls namespace visibility.

## Freshness

This GET reads the last-known status held by the daemon and does not run status
drivers. The daemon can temporarily report the previous state after an
out-of-band failure or recovery. Compare `updated_at` with the current time;
continue with `list_object_instances` to locate the timestamp and state of each
instance. A future explicit refresh tool may update this data, but refresh is
not an implicit side effect of this read-only tool.

## Input

| Field | Required | Meaning |
|---|---:|---|
| `path` | Yes | Exact canonical path returned by `list_cluster_objects` |

Selectors and wildcard paths are intentionally unsupported.

## Output

The output contains the parsed object reference, actor states when applicable,
placement and orchestration settings, sorted scope, last update time, reported up
instance count, and sorted instance node names. `is_actor` distinguishes svc
and vol actors from support objects that do not publish availability.

## Errors

The call fails for invalid paths, invisible or missing objects, unexpected
daemon selections, authorization failures, transport errors, or malformed
responses. No partial result is returned.

## Compatibility

Verified against OpenSVC `3.0.0-rc21` `GET /api/object` behavior and namespace
filtering.

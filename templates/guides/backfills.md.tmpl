---
page_title: "Backfills"
description: |-
  How to backfill bindings, and which changes require a backfill.
---

# Backfills

## Backfill Control

A backfill reads a binding's data again: a capture binding captures its whole resource again, and a materialization binding materializes its whole collection again. Each binding has a `backfill` block, and raising its `trigger` backfills the binding:

```hcl
{
  resource = { database = "shop", collection = "orders" }
  backfill = {
    trigger = 2 # was 1: fix missing rows (2026-10-22)
  }
}
```

> **Important:** `backfill.trigger` isn't Estuary's backfill counter: each raise adds one to the live counter. The two never need to match.
>
> - Each raise is one backfill, even by more than one (2 → 5).
> - Start at 1. It only goes up, and must be known at plan time: `timestamp()` would backfill on every apply.
> - Setting it on an existing binding that didn't have it backfills once. A new binding only records it.
> - Removing it does nothing. Keep it, with the reason in a comment, so the Git history tells why each backfill happened.

The top-level `backfill.trigger` of a capture or materialization backfills **every enabled** binding, including those Terraform doesn't manage. The plan warns with the list.

## Capture Strategy

A capture's `backfill.strategy` sets what a raised trigger does, and a binding's `backfill.strategy` overrides it:

| `strategy` | What a raised trigger does |
|---|---|
| `incremental` (default) | Captures the resource again into the existing collection. Destination tables are kept. |
| `dataflowReset` | Resets the collection: its data is replaced by a fresh backfill from the source, its inferred schema is cleared, and every materialization of it is backfilled, truncating its destination tables. See [Dataflow reset](https://docs.estuary.dev/reference/backfilling-data/#dataflow-reset). The plan warns about every collection it resets. |

```hcl
resource "estuary_source_aurora_postgres" "sales" {
  # ...
  backfill = {
    trigger  = 2               # was 1: rebuild every binding (2026-10-22)
    strategy = "dataflowReset" # default of every binding
  }

  bindings = [
    { resource = { namespace = "public", stream = "sales" } }, # reset
    {
      resource = { namespace = "public", stream = "audit_log" }
      backfill = { strategy = "incremental" } # captured again, destination tables kept
    },
  ]
}
```

## Changes That Require a Backfill

The plan fails unless the binding's trigger, or the capture's or materialization's, is raised in the same change:

- On SQL captures (PostgreSQL, MySQL), changing a binding's `backfill.mode`: `Normal`, `Precise`, `Only Changes`, `Without Primary Key`, or unset for the connector's choice. See [Backfill Modes](https://docs.estuary.dev/reference/backfilling-data/#resource-configuration-backfill-modes).
- On materializations, switching a binding's `resource.delta_updates` from `true` to `false`, including when delta updates were turned on outside Terraform. A delta updates table holds several rows per key, which standard updates can't merge. Switching from `false` to `true` needs no backfill.

> **Caution:** Removing a binding drops its live counter, so a binding added back starts over. To stop a binding and keep its progress, set `disable = true`.

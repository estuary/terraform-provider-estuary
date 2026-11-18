---
page_title: "Existing Pipelines"
description: |-
  How the provider handles changes made outside Terraform, and how to import existing captures and materializations.
---

# Existing Pipelines

## Changes Made Outside Terraform

Plans refresh every field from Estuary: the endpoint `config`, `image`, `auto_discover`, the bindings and their settings. The one exception is the shard `flags` Estuary sets and your configuration doesn't, which applies keep. See [Task Settings](https://registry.terraform.io/providers/estuary/estuary/latest/docs/guides/task-settings).

- Dashboard changes to fields you set show as changes that the apply reverts.
- Live values you don't declare, such as a `priority` set in the dashboard, show as removed. Declare the ones to keep.
- A declared binding deleted outside Terraform is added back.
- A live spec the provider can't read fails the plan until it's fixed in Estuary.
- If anything else publishes the entry while an apply runs, such as a dashboard edit or an auto-discovery, the apply fails instead of overwriting it. Plan and apply again.

## Importing

```bash
terraform import estuary_source_mongodb.shop acmeCo/mongodb/source-mongodb
```

- The tenant and connector name can be left out of the ID, such as `mongodb`.
- Bindings aren't imported: Terraform manages only the bindings you declare, so the others stay unmanaged and applies keep them, as `keep_unmanaged_bindings` sets. The first apply takes over each declared binding with the collection and resource of a live one, after a plan warning listing its field changes. With `keep_unmanaged_bindings = false`, it removes the live bindings you don't declare, after a plan warning listing them.
- Secrets aren't imported, so the first apply publishes the ones in your configuration. Shard `flags` aren't imported either, and applies keep them.
- Backfill triggers aren't imported either, so the first apply after the import records the ones in your configuration as their starting values and backfills nothing. Raise them afterwards to backfill. A change that requires a backfill, such as a binding's backfill mode, can't be part of that first apply. See [Backfills](https://registry.terraform.io/providers/estuary/estuary/latest/docs/guides/backfills).
- Each binding Terraform manages gets the `resource_path` Estuary recorded for it, which identifies it from then on. States written by earlier provider versions get theirs on the next refresh.
- Creating a capture or materialization that already exists in Estuary fails: import it instead.
- Live fields the provider doesn't manage are named in an import warning. The apply keeps them at the top level of the spec and in the endpoint, unless it publishes the endpoint, and removes them from declared bindings.

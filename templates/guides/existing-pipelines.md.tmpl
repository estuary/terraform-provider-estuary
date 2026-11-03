---
page_title: "Existing Pipelines"
description: |-
  How the provider handles changes made outside Terraform, and how to import existing captures and materializations.
---

# Existing Pipelines

## Changes Made Outside Terraform

Plans refresh every field from Estuary: the endpoint `config`, `image`, `auto_discover`, the bindings and their settings.

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
- A capture with `auto_discover` or a materialization with `source.capture` is imported without bindings, which stay unmanaged until you declare them. Anything else is imported with all its bindings.
- Secrets aren't imported, so the first apply publishes the ones in your configuration.
- Creating a capture or materialization that already exists in Estuary fails: import it instead.
- Live fields the provider doesn't manage are named in an import warning. The apply keeps them at the top level of the spec and in the endpoint, unless it publishes the endpoint, and removes them from declared bindings.

---
page_title: "Task Settings"
description: |-
  How to set the shard settings and connector version of captures and materializations.
---

# Task Settings

## Shard Settings

`shards` sets how Estuary runs a capture or materialization, such as pausing it or raising its log level. Write Estuary's own keys, in camelCase. See [Configuring Task Shards](https://docs.estuary.dev/reference/Configuring-task-shards/).

```hcl
shards = {
  logLevel       = "debug" # error, warn, info (default), debug or trace
  disable        = false   # true pauses the task
  minTxnDuration = "30s"
  maxTxnDuration = "4m"
}
```

- **Unset:** Estuary manages them, and applies keep the live settings. A task disabled outside Terraform gets a plan warning.
- **Set:** Terraform manages every shard setting. Live keys your block leaves out, such as `flags`, show as removed: declare the ones to keep.
- **Removed:** the settings go back to Estuary, as they are live.
- **Unknown keys** aren't checked by the plan: Estuary rejects them on apply.

## Connector Versions

Each resource with an `image` attribute (all but Dekaf) defaults to the connector's current major version tag, such as `ghcr.io/estuary/source-mongodb:v3`, which receives Estuary's fixes. Only the tag can change: the plan rejects any other image name. Change it only to move to another major version, after testing it outside production.

> **Warning:** Don't pin a per-commit tag. Estuary ships fixes only to major version tags, and some features, such as auto discovery, may not work on other tags.

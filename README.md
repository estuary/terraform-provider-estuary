# Estuary Terraform Provider

<p align="center">
  <a href="https://estuary.dev"><img src="https://storage.googleapis.com/estuary-marketing-strapi-uploads/uploads//estuary_logo_a373037876/estuary_logo_a373037876.png" width="200" alt="Estuary"></a>
</p>
<p align="center">
    <b>Build millisecond-latency, scalable, future-proof data pipelines in minutes</b>
</p>
<div align="center">

[![Estuary's Flow Repo](https://img.shields.io/github/stars/estuary/flow?label=Estuary)](https://github.com/estuary/flow)
[![Subscribe to Estuary's YouTube Channel](https://img.shields.io/youtube/channel/subscribers/UCJ9JIjh7uaUdjcFR6xTkJXQ?label=Subscribe)](https://www.youtube.com/@estuarydev?sub_confirmation=1)
[![Join Estuary's Slack](https://img.shields.io/badge/Slack-Estuary-blue)](https://go.estuary.dev/slack)
[![Follow Estuary on LinkedIn](https://img.shields.io/badge/Follow_on-LinkedIn-lightblue)](https://www.linkedin.com/company/estuary-tech/)

</div>

The Estuary Terraform provider manages your Estuary captures (sources) and materializations (destinations) as code.

Estuary is a data pipeline platform that combines batch and real-time data. It offers hundreds of low- and no-code connectors, transforms and aggregates data, and powers real-time applications.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Provider Configuration](#provider-configuration)
- [Quick Start](#quick-start)
- [Resources](#resources)
- [Common Patterns](#common-patterns)
  - [Bindings Lifecycle](#bindings-lifecycle)
  - [Field Selection](#field-selection)
  - [Backfill Control](#backfill-control)
  - [Shard Settings](#shard-settings)
  - [Connector Versions](#connector-versions)
- [Multi-Environment Best Practices](#multi-environment-best-practices)
  - [Define Pipelines Once](#define-pipelines-once)
  - [Give Each Environment Its Own State](#give-each-environment-its-own-state)
  - [Separate Environments with `tenant`](#separate-environments-with-tenant)
  - [Keep Secrets Out of Code](#keep-secrets-out-of-code)
  - [Share Pipelines with Modules](#share-pipelines-with-modules)

## Prerequisites

| Requirement | Version |
|---|---|
| Terraform | >= 1.11 |
| [`flowctl`](https://docs.estuary.dev/concepts/flowctl/) | Latest, on `PATH` or set through `flowctl_path` |
| Estuary account | [Sign up for free](https://dashboard.estuary.dev) |

> [!IMPORTANT]
> The provider currently acts on Estuary through `flowctl`, so it must be installed on the host where Terraform runs.

---

## Installation

Add the provider to your `terraform` block, then run `terraform init`:

```hcl
terraform {
  required_version = ">= 1.11"

  required_providers {
    estuary = {
      source  = "estuary/estuary"
      version = "~> 0.1"
    }
  }
}
```

---

## Provider Configuration

```hcl
provider "estuary" {
  tenant = "acmeCo/" # the token comes from FLOW_AUTH_TOKEN
}
```

Every attribute but `flowctl_path` can also be set through an environment variable.

> [!TIP]
> Pass the token through `FLOW_AUTH_TOKEN` rather than in `.tf` files. See [Keep Secrets Out of Code](#keep-secrets-out-of-code).

| Attribute | Environment Variable | Description |
|---|---|---|
| `token` | `FLOW_AUTH_TOKEN` | **Required.** Estuary refresh token, generated in the dashboard under Admin > [CLI-API](https://dashboard.estuary.dev/admin/api). Access tokens also work but expire, so use a refresh token in CI/CD. |
| `tenant` | `ESTUARY_TENANT` | Prefix added to every capture and materialization name that doesn't already start with it, such as `acmeCo/` or `acmeCo/dev/`. A missing trailing slash is added. Collection names you write in a binding `target` or `source` aren't prefixed, see [Separate Environments with `tenant`](#separate-environments-with-tenant). |
| `data_plane` | `ESTUARY_DATA_PLANE` | Data plane new captures and materializations are created in, and capture discovery runs on (e.g., `ops/dp/public/example-data-plane`). Existing entries keep their data plane, so changing it never moves them. Required for BYOC or a non-default public data plane. Defaults to the first data plane of your storage mapping. Find IDs in the dashboard under Admin > Settings > Data Planes. |
| `flowctl_path` | — | Path to the `flowctl` binary. Defaults to `flowctl` on `PATH`. |
| `backend` | `ESTUARY_BACKEND` | How the provider reaches Estuary. Defaults to `flowctl`, the only backend available for now. |

---

## Quick Start

A capture that reads two MongoDB collections into Estuary:

```hcl
provider "estuary" {
  tenant = "acmeCo/"
}

variable "mongodb_password" {
  type      = string
  sensitive = true
}

resource "estuary_source_mongodb" "shop" {
  name = "mongodb"

  config = {
    address  = "mongodb+srv://cluster0.123456789.mongodb.net?authSource=admin"
    user     = "flow_capture"
    password = var.mongodb_password
    database = "shop"
  }

  bindings = [
    { resource = { database = "shop", collection = "users" } },
    { resource = { database = "shop", collection = "orders" } },
  ]
}
```

```bash
export FLOW_AUTH_TOKEN="<refresh token>"
export TF_VAR_mongodb_password="<password>"
```

Then plan, review, and apply:

```bash
terraform init
terraform plan -out tfplan
terraform apply tfplan
```

> [!IMPORTANT]
> Read every plan before applying it. The provider makes Estuary match your configuration, so an apply can remove bindings, overwrite changes made in the dashboard, or trigger backfills.

This creates:

| Object | Name | Built from |
|---|---|---|
| Capture | `acmeCo/mongodb/source-mongodb` | `tenant` + `name` + the connector name |
| Collections | `acmeCo/mongodb/shop/users`, `acmeCo/mongodb/shop/orders` | the capture name without the connector name + each resource, see [Bindings Lifecycle](#bindings-lifecycle) |

---

## Resources

Each resource manages a capture or a materialization of one Estuary connector. Connectors configured the same way share one documentation page, with examples and the full list of attributes.

| Documentation | Type | Resources |
|---|---|---|
| [Dekaf](internal/resources/materializationdekaf/README.md) | Materialization | `estuary_materialization_dekaf_generic`, `estuary_materialization_dekaf_bytewax`, `estuary_materialization_dekaf_clickhouse`, `estuary_materialization_dekaf_imply_polaris`, `estuary_materialization_dekaf_materialize`, `estuary_materialization_dekaf_risingwave`, `estuary_materialization_dekaf_singlestore`, `estuary_materialization_dekaf_startree`, `estuary_materialization_dekaf_tinybird` |
| [Amazon DynamoDB](internal/resources/sourcedynamodb/README.md) | Capture | `estuary_source_dynamodb` |
| [MongoDB](internal/resources/sourcemongodb/README.md) | Capture | `estuary_source_mongodb` |
| [MySQL](internal/resources/sourcemysql/README.md) | Capture | `estuary_source_aurora_mysql`, `estuary_source_rds_mysql` |
| [MySQL](internal/resources/materializationmysql/README.md) | Materialization | `estuary_materialization_mysql`, `estuary_materialization_aurora_mysql`, `estuary_materialization_rds_mysql` |
| [PostgreSQL](internal/resources/sourcepostgres/README.md) | Capture | `estuary_source_aurora_postgres`, `estuary_source_rds_postgres` |
| [PostgreSQL](internal/resources/materializationpostgres/README.md) | Materialization | `estuary_materialization_postgres`, `estuary_materialization_aurora_postgres`, `estuary_materialization_rds_postgres` |

Each capture resource also has a data source of the same name, reading the live bindings of a capture managed elsewhere. See [Bindings Lifecycle](#bindings-lifecycle).

---

## Common Patterns

### Bindings Lifecycle

A binding links one resource, such as a MongoDB collection or a PostgreSQL table, to one Estuary collection: the `target` of a capture binding, or the `source` of a materialization binding. That collection identifies the binding, and the read-only `name` attribute shows it in every plan so you can tell which binding changes. Changing a binding's collection removes the old binding and adds a new one.

Estuary can add bindings on its own, through discovery (`auto_discover`) on a capture or `source.capture` on a materialization, and bindings can be added in the dashboard. `keep_unmanaged_bindings` decides what Terraform does with the bindings you don't declare:

| `keep_unmanaged_bindings` | Bindings Terraform manages | Bindings you don't declare |
|---|---|---|
| `true` (default) | Only the ones in your configuration | Kept, and not shown in plans. Removing `auto_discover` or `source.capture` keeps the ones Estuary added. |
| `false` | All of them | Shown in the plan and removed by the next apply. Can't be set with `auto_discover` (any block, even with `add_new_bindings = false`) or `source.capture`, as Estuary would add bindings that each apply removes. |

> [!WARNING]
> With the default, Terraform never removes a binding it doesn't manage, so bindings Estuary or the dashboard adds stay in the capture or materialization until you remove them. To remove them, set `keep_unmanaged_bindings = false`: the plan warns with the list of undeclared bindings the apply removes, and from then on shows them as regular changes. Captures list every live binding in `live_bindings`.

**Manage every binding (no auto discovery):**
```hcl
keep_unmanaged_bindings = false

bindings = [
  { resource = { database = "shop", collection = "users" } },
  { resource = { database = "shop", collection = "orders" } },
]
```

**Let discovery add new collections, managing only the declared ones:**
```hcl
auto_discover = {
  add_new_bindings = true
}

bindings = [
  { resource = { database = "shop", collection = "users" } },
]
```

**Stop capturing a collection:**
```hcl
bindings = [
  {
    resource = { database = "shop", collection = "sessions" }
    disable  = true # with auto_discover, removing the binding instead would let discovery add it back
  },
]
```

**Take over a binding added by discovery or in the dashboard:**
```hcl
bindings = [
  {
    resource = { database = "shop", collection = "events" } # its live backfill counter is kept
  },
]
```

**Materialize every collection of a capture, managed by the capture or not:**
```hcl
resource "estuary_materialization_postgres" "shop" {
  # ...

  bindings = [
    for binding in estuary_source_mongodb.shop.live_bindings : {
      source   = binding.target
      resource = { table = binding.resource.collection }
    } if binding.disable != true
  ]
}
```

**Follow a capture, declaring the bindings that need their own settings:**
```hcl
resource "estuary_materialization_postgres" "shop" {
  # ...

  source = {
    capture            = "acmeCo/mongodb/source-mongodb"
    fields_recommended = "unlimited" # bindings Estuary adds get only these settings and target_naming
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { table = "orders" }
      fields   = { recommended = "unlimited", require = ["flow_document"] }
    },
  ]
}
```

**Lifecycle rules:**
- **Default target:** a capture binding without `target` keeps its current collection. A new one gets the collection the connector discovery names, so the provider never replicates the connector naming rules. For example, the capture `acmeCo/mongodb/source-mongodb` writes `shop.users` to `acmeCo/mongodb/shop/users`. Discovery runs during `terraform plan` only for new bindings without `target`, from Estuary's data plane, so the source must be reachable and the plan fails naming each binding whose resource discovery doesn't find. Set `target` to skip discovery.
- **Changes made outside Terraform:** plans refresh every field from the live spec, on the capture or materialization and on its bindings, such as the endpoint `config`, `image` or `auto_discover`. Dashboard edits to the fields you set show as changes the next apply reverts, and the live values you don't declare, such as a `priority` set in the dashboard, show as removed, as the apply would drop them: declare the ones to keep. Empty live values (`false`, `""`, `0`, empty lists) are left out, and write-only secrets are never read back. A declared binding deleted outside Terraform is added back. A materialization binding whose live `source` holds a key other than `name`, `notBefore` or `notAfter` can't be refreshed: the plan warns about it and doesn't show that materialization's outside changes.
- **Bindings Estuary adds:** creating a capture with `add_new_bindings = true` runs a discovery and adds a binding for each discovered resource you didn't declare, as the dashboard does, and later discoveries keep adding new ones; updates never run this discovery. A materialization with `source.capture` gets a binding for each collection of the capture you didn't declare, and for each collection the capture adds later. Those materialization bindings get only `source.delta_updates`, `source.fields_recommended` and `target_naming`, not per-binding overrides such as `fields.require`. The plan warns when either setting is turned on, on creation or on an update. Terraform doesn't manage these bindings: declare one to manage it and give it its own settings.
- **Removing a binding:** with `add_new_bindings = true` on a capture, discovery may add it back while the resource still exists in the source. With `source.capture` on a materialization, Estuary may add it back while the capture still has its collection. The plan warns about it. Use `disable = true` to stop it for good.
- **Taking over a binding:** the plan shows it as added, but the apply updates the live binding in place. A warning lists the fields the apply changes, so copy over the live values you want to keep.
- **Every live binding:** captures list all their live bindings in the read-only `live_bindings`, with the attributes of `bindings`: the declared ones and those discovery added. Build other bindings from it, such as a materialization of every collection of a capture, without declaring them all in the capture. It's refreshed on each plan, and when the capture's bindings change the plan shows the bindings the apply publishes, so plans built from it only show the bindings that change. When creating a capture with `add_new_bindings = true`, it stays unknown until the apply, as discovery adds bindings Terraform can't foresee. Bindings discovery adds later show up on the next plan; use `source.capture` for Estuary to add them right away. Empty live values are left out, so test optional fields like `binding.disable != true`.
- **Captures managed elsewhere:** each capture resource has a data source of the same name, such as `data "estuary_source_mongodb"`, reading the `live_bindings` of a capture by name. Don't use it for a capture in the same configuration: referencing the resource defers the read to apply whenever the capture has changes, so everything built from it shows as known after apply. It fails when the capture doesn't exist, rather than listing no bindings.
- **Importing:** a capture with `auto_discover` or a materialization with `source.capture` is imported without its bindings, which stay unmanaged until you declare them. Any other capture or materialization is imported with all of its bindings. Import by catalog name, such as `terraform import estuary_source_mongodb.shop acmeCo/mongodb/source-mongodb`; the tenant and connector name can be left out. Write-only secrets aren't imported, so the first apply publishes the ones in your configuration. Live fields the provider doesn't manage are named in an import warning. The next apply keeps the unknown top-level fields, and the unknown endpoint fields until it publishes the endpoint, which it does when the configuration sets secrets or changes the endpoint. It removes the unknown fields of the bindings declared in the configuration. Credentials found in a connection string, such as a MongoDB `address`, show as `***`.

### Field Selection

Each materialization binding can set `fields` to choose which collection fields it materializes. Omit it to use Estuary's default selection.

> [!NOTE]
> Estuary writes its default selection into each binding that omits `fields`, usually `recommended = "unlimited"`. The provider keeps that value in state, so plans show no diff for it, and publishes it back unchanged. Once you set `fields` on a binding, your value replaces it.

| Attribute | Behavior |
|---|---|
| `recommended` | Required when `fields` is set. Depth of the fields selected by default: `"0"` selects only the root document and the required fields, such as the collection key, `"1"` the top-level fields, `"2"` the second-level fields too, and so on, and `"unlimited"` every field. |
| `require` | Fields always materialized, whatever `recommended` selects. |
| `exclude` | Fields never materialized. |
| `group_by` | Fields keying the table instead of the collection key. Only fields with a scalar type (boolean, integer or string) can be keys, and changing them backfills the binding. |

A materialization's `source.fields_recommended` sets the `recommended` depth of the bindings Estuary adds itself, such as through `source.capture`.

**Include only specific fields:**
```hcl
{
  source   = "acmeCo/mongodb/shop/orders"
  resource = { table = "orders_slim" }

  fields = {
    recommended = "0"
    require     = ["customer_id", "status", "total"]
  }
}
```

**Exclude fields from the selection:**
```hcl
{
  source   = "acmeCo/mongodb/shop/orders"
  resource = { table = "orders" }

  fields = {
    recommended = "1"
    exclude     = ["internal_notes"]
  }
}
```

**Key the table by fields other than the collection key:**
```hcl
{
  source   = "acmeCo/mongodb/shop/order_lines"
  resource = { table = "order_lines" }

  fields = {
    recommended = "1"
    group_by    = ["order_id", "line_number"]
  }
}
```

> [!NOTE]
> **`flow_document`:** This special field holds the full source document.
> - With standard updates, it's always materialized, because the connector reads it back to merge each update. The exception is when the connector's `no_flow_document` advanced option is enabled.
> - With `delta_updates = true`, it's only selected with `recommended = "0"`. Add it to `require` to keep it at other depths.

### Backfill Control

A backfill re-reads a binding's data: a capture binding captures its whole resource again, and a materialization binding materializes its whole collection again. Use it to fill new columns after changing a field selection, to recreate a dropped table, or to reprocess data after a schema change.

Each binding has a `backfill` block. To backfill the binding, raise its `trigger`:

```hcl
{
  resource = { database = "shop", collection = "orders" }
  backfill = {
    trigger = 2 # was 1: fix missing rows (2026-10-22)
  }
}
```

> [!IMPORTANT]
> **`backfill.trigger` is not Estuary's backfill counter.** It's an internal control of the provider, which Estuary never sees: each time you raise it, the apply raises Estuary's counter by one from its live value. The two never need to match.
> - Each raise is one backfill, even by more than one (2 → 5). Leave the new value in place afterwards.
> - Start at 1. It only goes up: the plan fails when it's lower than before, or when it's only known at apply time, such as `timestamp()`, which would backfill on every apply.
> - Setting it where it wasn't set before counts as a raise: on a binding already in Estuary, managed or taken over, `trigger = 1` backfills it once. A binding new to Estuary only records it, as creating it backfills anyway.
> - Removing it does nothing, but setting it again later backfills. Keep it in place, with the reason in a comment, so the Git history tells why each backfill happened.

Terraform doesn't show Estuary's counter: the apply reads it from the live spec and never lowers it. Backfills started elsewhere, from the dashboard, a dataflow reset or a schema evolution, never show as changes, and you never copy the counter over, even when taking over a live binding.

On SQL captures (PostgreSQL, MySQL), a binding's `backfill.mode` also sets how its table is backfilled: `Normal`, `Precise`, `Only Changes` or `Without Primary Key`, or unset for the connector's automatic choice. See [Backfill Modes](https://docs.estuary.dev/reference/backfilling-data/#resource-configuration-backfill-modes). The connector requires a backfill along with any mode change, so the plan fails when `mode` changes without raising the binding's `trigger`, or the capture's, in the same change. Taking over a binding with a changed mode needs a `trigger` set on it, or a raised capture `trigger`.

Captures also take a `backfill` block at the top level, as the dashboard's **Backfill** button and **Backfill mode**:

- Raising the capture's `backfill.trigger` backfills **every** binding of the capture, those added by discovery that Terraform doesn't manage included. A binding whose own trigger rises in the same apply still backfills once. Plans warn with the list of bindings it backfills, and list those it resets in a separate warning.
- The capture's `backfill.strategy` is the default of its bindings, and a binding's `backfill.strategy` overrides it. Changing a strategy alone publishes nothing.

| `strategy` | What a raised trigger does to a capture binding |
|------------|-------------------------------------------------|
| `incremental` (default when unset) | Captures the resource again into the existing collection. Destination tables are kept and the data merges into them. |
| `dataflowReset` | Resets the binding's collection instead: its data is replaced by a fresh backfill from the source, its inferred schema is cleared, and every materialization of it backfills, truncating its destination tables. See [Dataflow reset](https://docs.estuary.dev/reference/backfilling-data/#dataflow-reset). |

```hcl
resource "estuary_source_aurora_postgres" "sales" {
  # ...
  backfill = {
    trigger  = 2               # was 1: rebuild every binding (2026-10-22)
    strategy = "dataflowReset" # default of every binding
  }

  bindings = [
    {
      resource = { namespace = "public", stream = "sales" } # resets
    },
    {
      resource = { namespace = "public", stream = "audit_log" }
      backfill = {
        strategy = "incremental" # captures again, keeping its destination tables
      }
    },
    {
      resource = { namespace = "public", stream = "orders" }
      backfill = {
        trigger = 3 # was 2: raised on its own, it backfills only this binding, resetting it
      }
    },
  ]
}
```

Plans warn about every collection the apply resets, those of unmanaged bindings included. A materialization with `on_incompatible_schema_change = "abort"` makes the reset fail.

Materializations also take a `backfill` block at the top level, as the dashboard's **Backfill** button under **Source Collections → Advanced options**. Raising its `trigger` backfills **every** enabled binding of the materialization, those Estuary adds through `source.capture` included: each table is truncated and materialized again from the collection, without reading the source again. Plans warn with the list of bindings it backfills. It has no `strategy`: to read the source again, backfill the capture instead.

```hcl
resource "estuary_materialization_postgres" "warehouse" {
  # ...
  backfill = {
    trigger = 2 # was 1: rebuild every table after the field selection change (2026-10-28)
  }
}
```

> [!CAUTION]
> Removing a binding drops its live counter, so a binding added back starts over. To stop a binding while keeping its progress, set `disable = true` instead.

### Shard Settings

`shards` sets how Estuary runs a capture or materialization, such as pausing it or raising its log level. It's free-form: write Estuary's own keys, in camelCase, and they're published as written. See [Configuring Task Shards](https://docs.estuary.dev/reference/Configuring-task-shards/) for the available settings.

```hcl
shards = {
  logLevel       = "debug" # error, warn, info (default), debug or trace
  disable        = false   # true pauses the task without deleting it
  minTxnDuration = "30s"
  maxTxnDuration = "4m"
}
```

- **Unset `shards`:** Estuary manages them. Every apply keeps the live settings, such as a paused task or the flags Estuary adds, and plans never show them as changes. A task disabled outside Terraform gets a plan warning, so it doesn't stay paused unnoticed: set `disable = false` to enable it, or `disable = true` to manage it as paused.
- **Set `shards`:** Terraform manages every shard setting. Plans diff your block against all the live settings: dashboard changes to your keys show as reverted, and the live keys your block leaves out, such as `flags`, show as removed, so declare the ones to keep.
- **Removing the block** hands the settings back to Estuary, keeping them as they are live.
- **Unknown keys** aren't checked by the plan: Estuary rejects them when the apply publishes.

### Connector Versions

Each resource defaults to the connector's current major version tag (e.g., `ghcr.io/estuary/source-mongodb:v3`), which receives Estuary's fixes and improvements. Set `image` only to move to another major version on purpose, and test it in a non-production environment first, as major versions may not be drop-in replacements.

Only the tag can change: the image name is fixed per resource, and the plan rejects any other image, so a configuration can't hand its secrets to a connector other than the resource's own.

```hcl
resource "estuary_source_mongodb" "shop" {
  name  = "mongodb"
  image = "ghcr.io/estuary/source-mongodb:v3" # the default, set it only to change the major version
  # ...
}
```

> [!WARNING]
> Don't pin a per-commit image tag. Estuary ships fixes only to the major version tags, so a task pinned to a commit stops getting them, and some features, such as auto discovery, may not work on unregistered tags.

---

## Multi-Environment Best Practices

### Define Pipelines Once

Write your captures and materializations once, in a single root module, and apply it once per environment. Production then runs exactly what was tested in dev, from one copy of the code. Only values change between environments, through a variable file per environment:

```
main.tf
variables.tf
backend.tf
dev.tfvars
production.tfvars
```

```hcl
# variables.tf
variable "tenant" {
  type = string
}

variable "data_plane" {
  type    = string
  default = null # Estuary's default data plane
}

variable "mongodb_address" {
  type = string
}
```

```hcl
# dev.tfvars
tenant          = "acmeCo/dev/"
mongodb_address = "mongodb+srv://dev-cluster.123456789.mongodb.net?authSource=admin"
```

```hcl
# production.tfvars
tenant          = "acmeCo/prod/"
data_plane      = "ops/dp/public/example-data-plane"
mongodb_address = "mongodb+srv://prod-cluster.123456789.mongodb.net?authSource=admin"
```

Secrets don't go in these files, see [Keep Secrets Out of Code](#keep-secrets-out-of-code).

### Give Each Environment Its Own State

Terraform state records which captures and materializations a configuration manages. If dev and production shared one state, applying production would plan to replace what dev created. Pick one of these layouts. The first two keep a single copy of the code.

**Workspaces, if all environments can share one backend.** Each [Terraform workspace](https://developer.hashicorp.com/terraform/language/state/workspaces) keeps its own state in the same backend. Name each variable file after its workspace, so the selected workspace picks the right values:

```bash
terraform workspace new production # once per environment
terraform workspace select production
terraform apply -var-file="$(terraform workspace show).tfvars"
```

> [!WARNING]
> The `backend` block can't use variables, so all workspaces share one backend and its credentials: anyone who can read dev's state can also read production's.

**One backend per environment, if environments need isolated state.** Leave the environment-specific settings out of the `backend` block, and pass them to `terraform init` through a [partial configuration](https://developer.hashicorp.com/terraform/language/backend#partial-configuration) file per environment:

```hcl
# backend.tf
terraform {
  backend "s3" {} # bucket, key, and region come from the .tfbackend file
}
```

```hcl
# production.s3.tfbackend
bucket = "acme-terraform-prod"
key    = "estuary/terraform.tfstate"
region = "eu-west-1"
```

```bash
ENV=production
terraform init -reconfigure -backend-config="$ENV.s3.tfbackend"
terraform apply -var-file="$ENV.tfvars"
```

> [!IMPORTANT]
> Terraform remembers the last backend you initialized in the `.terraform` directory, so always pass `-reconfigure` when switching environments, or set a separate [`TF_DATA_DIR`](https://developer.hashicorp.com/terraform/cli/config/environment-variables#tf_data_dir) per environment.

**Separate root modules, only if environments run different pipelines.** For example, when a pipeline exists only in production. Still define each pipeline once, in a [module](#share-pipelines-with-modules), so each root module only chooses which pipelines to run and with which values.

### Separate Environments with `tenant`

Separate states keep Terraform's records apart, but not Estuary's names: all environments share one Estuary namespace. If dev and production both apply a capture named `mongodb`, they manage the same capture and undo each other's changes on every apply. To keep them apart, give each environment its own [`tenant`](#provider-configuration) prefix in its variable file:

```hcl
provider "estuary" {
  tenant     = var.tenant     # "acmeCo/dev/" in dev.tfvars, "acmeCo/prod/" in production.tfvars
  data_plane = var.data_plane # the storage mapping's first data plane when null
}

resource "estuary_source_mongodb" "shop" {
  name = "mongodb"
  # ...
}
```

Despite its name, `tenant` can be any prefix under your Estuary tenant, such as `acmeCo/dev/`. Your token must have access to it.

Changing `tenant` or `name` changes the catalog name, so Terraform replaces the capture or materialization: it deletes it and creates a new one. Writing the prefix or connector name in `name` yourself, such as `name = "acmeCo/dev/mongodb"`, names the same entry and never replaces it. Without `tenant`, `name` must hold the full prefix.

The prefix is added only to names the provider builds. Names you write out yourself are used exactly as written:

| Name | Prefixed | In dev | In production |
|---|---|---|---|
| Capture or materialization `name` | Yes | `acmeCo/dev/mongodb/source-mongodb` | `acmeCo/prod/mongodb/source-mongodb` |
| Capture binding without `target` | Yes, from the capture name | `acmeCo/dev/mongodb/shop/users` | `acmeCo/prod/mongodb/shop/users` |
| Capture binding `target` you set | No | as written | as written |
| Materialization binding `source` | No | as written | as written |

Don't hard-code a collection name in a binding. For example, `source = "acmeCo/prod/mongodb/shop/users"` would make the dev materialization read production data. Instead, take the collection from the capture so that it changes with the environment:

```hcl
resource "estuary_materialization_postgres" "shop" {
  name = "postgres"
  # ...

  bindings = [
    for binding in estuary_source_mongodb.shop.bindings : {
      source   = binding.target # acmeCo/dev/... in dev, acmeCo/prod/... in production
      resource = { table = binding.resource.collection }
    }
  ]
}
```

> [!TIP]
> `bindings` only holds the bindings declared in the capture, those Terraform manages. With `auto_discover`, loop over `live_bindings` instead to also materialize the collections discovery adds, see [Bindings Lifecycle](#bindings-lifecycle).

### Keep Secrets Out of Code

Never commit tokens or passwords. Pass them through environment variables: the provider reads `FLOW_AUTH_TOKEN`, and Terraform reads each `TF_VAR_<name>` as the variable `<name>`:

```yaml
# GitHub Actions
env:
  FLOW_AUTH_TOKEN: ${{ secrets.ESTUARY_REFRESH_TOKEN }}
  TF_VAR_mongodb_password: ${{ secrets.MONGODB_PASSWORD }}
  TF_VAR_postgres_password: ${{ secrets.POSTGRES_PASSWORD }}
```

Connector secrets such as `password` and `private_key` are write-only: Terraform sends them to Estuary but never stores them in the state or shows them in plans. State keeps only a salted hash of each one, such as `config.password_hash`, so a changed secret shows as a plan warning and is published on apply.

> [!WARNING]
> Run Terraform in a trusted environment, such as your workstation or an ephemeral CI runner, that no other user or job can read. While a command runs, the provider hands flowctl the spec, secrets included, in plain text in a private `estuary-tf-*` directory of the system temporary directory (`TMPDIR`), readable by your user only, and removes it once the command ends. A provider killed mid-command, such as by `kill -9` or the host running out of memory, leaves that directory behind. On shared or long-lived machines, point `TMPDIR` to a directory removed after each run.

An apply that changes neither the `config` nor a secret, such as adding a binding, republishes the config Estuary already encrypted, so its history shows no config change. Changing a config field or a secret publishes your config, which Estuary encrypts again.

### Share Pipelines with Modules

When several pipelines follow the same pattern, such as one MongoDB-to-PostgreSQL pipeline per application, write the pattern once in a module and call it once per pipeline. A fix to the module then reaches every pipeline. Modules also let [separate root modules](#give-each-environment-its-own-state) share pipelines without copying them:

```hcl
# modules/mongodb-to-postgres/main.tf
terraform {
  required_providers {
    estuary = {
      source = "estuary/estuary" # required in every module, or Terraform looks for hashicorp/estuary
    }
  }
}

# Variables: name, mongodb_address, mongodb_password, postgres_address,
# postgres_password, and collections (map(list(string)), by MongoDB database).

resource "estuary_source_mongodb" "source" {
  name = var.name

  config = {
    address  = var.mongodb_address
    user     = "flow_capture"
    password = var.mongodb_password
    database = join(",", keys(var.collections))
  }

  bindings = flatten([
    for database, collections in var.collections : [
      for collection in collections : {
        resource = { database = database, collection = collection }
      }
    ]
  ])
}

resource "estuary_materialization_postgres" "destination" {
  name = var.name

  config = {
    address  = var.postgres_address
    user     = "flow_materialize"
    database = "analytics"

    credentials = {
      auth_type = "UserPassword"
      password  = var.postgres_password
    }
  }

  bindings = [
    for binding in estuary_source_mongodb.source.bindings : {
      source   = binding.target
      resource = { table = binding.resource.collection }
    }
  ]
}
```

```hcl
# main.tf
module "shop" {
  source = "./modules/mongodb-to-postgres"

  name              = "shop"
  mongodb_address   = var.mongodb_address
  mongodb_password  = var.mongodb_password
  postgres_address  = var.postgres_address
  postgres_password = var.postgres_password

  collections = {
    shop = ["users", "orders", "products"]
  }
}
```

With `tenant = "acmeCo/prod/"`, this creates the capture `acmeCo/prod/shop/source-mongodb` and the materialization `acmeCo/prod/shop/materialize-postgres`.

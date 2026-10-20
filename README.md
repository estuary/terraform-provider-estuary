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

Estuary is a data pipeline platform that combines batch and real-time data. It offers hundreds of low- and no-code connectors, transforms and aggregates data, and powers real-time applications.

The Estuary Terraform provider manages your Estuary captures (sources) and materializations (destinations) as code.

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
  - [Connector Versions](#connector-versions)
- [Multi-Environment Best Practices](#multi-environment-best-practices)
  - [Define Pipelines Once](#define-pipelines-once)
  - [Give Each Environment Its Own State](#give-each-environment-its-own-state)
  - [Isolate Catalogs with `tenant`](#isolate-catalogs-with-tenant)
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
| `tenant` | `ESTUARY_TENANT` | Prefix added to every capture and materialization name that doesn't already start with it (e.g., `acmeCo/`). Collection names, such as a binding `target` or `source`, aren't prefixed. |
| `data_plane` | `ESTUARY_DATA_PLANE` | Data plane the connectors run in (e.g., `ops/dp/public/aws-eu-west-1-c2`). Required for BYOC or a non-default public data plane. Defaults to Estuary's default data plane. |
| `flowctl_path` | — | Path to the `flowctl` binary. Defaults to `flowctl` on `PATH`. |
| `backend` | `ESTUARY_BACKEND` | How the provider reaches Estuary. Defaults to `flowctl`, the only backend available for now. |

---

## Quick Start

A pipeline that captures two MongoDB collections and materializes each one into a PostgreSQL table:

```hcl
provider "estuary" {
  tenant = "acmeCo/"
}

variable "mongodb_password" {
  type      = string
  sensitive = true
}

variable "postgres_password" {
  type      = string
  sensitive = true
}

resource "estuary_source_mongodb" "shop" {
  name = "shop"

  config = {
    address  = "mongodb+srv://cluster0.abc123.mongodb.net?authSource=admin"
    user     = "flow_capture"
    password = var.mongodb_password
    database = "shop"
  }

  bindings = [
    { resource = { database = "shop", collection = "users" } },
    { resource = { database = "shop", collection = "orders" } },
  ]
}

resource "estuary_materialization_postgres" "shop" {
  name = "shop"

  config = {
    address  = "postgres.example.com:5432"
    user     = "flow_materialize"
    database = "analytics"

    credentials = {
      auth_type = "UserPassword"
      password  = var.postgres_password
    }
  }

  bindings = [
    for binding in estuary_source_mongodb.shop.bindings : {
      source   = binding.target
      resource = { table = binding.resource.collection }
    }
  ]
}
```

```bash
export FLOW_AUTH_TOKEN="<refresh token>"
export TF_VAR_mongodb_password="<password>"
export TF_VAR_postgres_password="<password>"
terraform init
terraform apply
```

This creates the capture `acmeCo/shop/source-mongodb` and the materialization `acmeCo/shop/materialize-postgres`. The materialization reads its bindings from the capture, so a collection added to the capture is materialized too.

---

## Resources

Each resource manages a capture or a materialization of one Estuary connector. See its documentation for examples and the full list of attributes.

| Resource | Type | Connector |
|---|---|---|
| [`estuary_source_mongodb`](internal/resources/capture/mongodb/README.md) | Capture | [MongoDB](https://docs.estuary.dev/reference/Connectors/capture-connectors/MongoDB/) |
| `estuary_materialization_postgres` | Materialization | [PostgreSQL](https://docs.estuary.dev/reference/Connectors/materialization-connectors/PostgreSQL/) |

---

## Common Patterns

### Bindings Lifecycle

Each binding is identified by its collection: the `target` of a capture binding, or the `source` of a materialization binding. The read-only `name` attribute shows that collection in every plan, so you can tell which binding changes. Changing a binding's collection removes the old binding and adds a new one.

Which bindings Terraform manages depends on whether Estuary can add bindings on its own, through discovery (`auto_discover`) or a source capture (`source.capture`):

| Task | Bindings Terraform manages | Bindings added outside Terraform |
|---|---|---|
| Capture without `auto_discover` | All of them | Shown in the plan and removed by the next apply. |
| Capture with `auto_discover` | Only the ones in your configuration | Kept, and not shown in plans. |
| Materialization without `source.capture` | All of them | Not shown in plans, but removed by the next apply. |
| Materialization with `source.capture` | Only the ones in your configuration | Kept, and not shown in plans. |

**Manage every binding (no auto discovery):**
```hcl
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
    disable  = true # removing the binding instead would let discovery add it back
  },
]
```

**Take over a binding added by discovery or in the dashboard:**
```hcl
bindings = [
  {
    resource = { database = "shop", collection = "events" }
    backfill = 3 # the live counter, see Backfill Control
  },
]
```

**Lifecycle rules:**
- **Default target:** a capture binding without `target` keeps its current collection. A new one gets the collection discovery would name: the capture name without its connector name, followed by the resource name (e.g., `acmeCo/mongodb/shop/users`).
- **Changes made outside Terraform:** plans refresh the fields you set on capture bindings, so dashboard edits show up as changes the next apply reverts. Fields you leave unset stay managed by Estuary, and a declared binding deleted outside Terraform is added back.
- **Removing a binding:** with `add_new_bindings = true`, discovery may add it back while the collection exists in the source, and the plan warns about it.
- **Taking over a binding:** the plan shows it as added, but the apply updates the live binding in place. A warning lists the fields the apply changes, so copy over the live values you want to keep.
- **Importing:** a task Estuary may add bindings to is imported without its bindings, which stay unmanaged until you declare them. Other tasks are imported with all of their bindings.

> [!WARNING]
> Materialization plans don't refresh bindings yet, so the next apply overwrites dashboard edits to a materialization's bindings without showing them in the plan.

### Field Selection

Each materialization binding can set `fields` to choose which collection fields it materializes. Omit it to use Estuary's default selection.

| Attribute | Behaviour |
|---|---|
| `recommended` | Required when `fields` is set. `true` selects every field of the collection. `false` selects only the fields the connector requires, such as the collection key, plus the ones in `require`. |
| `require` | Fields always materialized, whatever `recommended` selects. |
| `exclude` | Fields never materialized. |

**Include only specific fields:**
```hcl
{
  source   = "acmeCo/mongodb/shop/orders"
  resource = { table = "orders_slim" }

  fields = {
    recommended = false
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
    recommended = true
    exclude     = ["internal_notes"]
  }
}
```

> [!NOTE]
> **`flow_document`:** This special field holds the full source document. With standard updates it is always materialized, since the connector reads it back to merge each update, unless the connector's `no_flow_document` advanced option is enabled. With `delta_updates = true` it is excluded by default, so add it to `require` to keep it.

### Backfill Control

The `backfill` counter on each binding triggers a full re-read of its data: a capture binding captures its whole resource again, and a materialization binding materializes its whole collection again. Use it to fill new columns after changing a field selection, to recreate a dropped table, or to reprocess data after a schema change.

```hcl
{
  resource = { database = "shop", collection = "orders" }
  backfill = 2 # was 1 → increment by 1 to trigger a backfill
}
```

> [!CAUTION]
> **The counter never goes down.** Each increment triggers exactly one backfill. Leave the new value in place afterwards: don't reset it or remove the attribute, which sets it back to 0. Estuary keeps each binding's progress under its counter, so a lower counter can make a capture resume from stale progress, and materializations reject it. When you add back a removed binding or take over a live one, set `backfill` to the value it had.

### Connector Versions

Each resource defaults to the connector's current major version tag (e.g., `ghcr.io/estuary/source-mongodb:v3`), which receives Estuary's fixes and improvements. Set `image` only to move to another major version on purpose, and test it in a non-production environment first, as major versions may not be drop-in replacements.

```hcl
resource "estuary_source_mongodb" "shop" {
  name  = "mongodb"
  image = "ghcr.io/estuary/source-mongodb:v3" # the default, set it only to change the major version
  # ...
}
```

> [!WARNING]
> Avoid pinning a per-commit image tag: tasks running a tag Estuary doesn't register stop getting fixes, skip auto discovery, and can't be used with a materialization `source.capture`.

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
mongodb_address = "mongodb+srv://dev-cluster.abc123.mongodb.net?authSource=admin"
```

```hcl
# production.tfvars
tenant          = "acmeCo/prod/"
data_plane      = "ops/dp/public/aws-eu-west-1-c2"
mongodb_address = "mongodb+srv://prod-cluster.xyz789.mongodb.net?authSource=admin"
```

Secrets don't go in these files, see [Keep Secrets Out of Code](#keep-secrets-out-of-code).

### Give Each Environment Its Own State

Pick one of these layouts. The first two keep a single copy of the code.

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

### Isolate Catalogs with `tenant`

Give each environment its own [`tenant`](#provider-configuration) prefix, set from its variable file:

```hcl
provider "estuary" {
  tenant = var.tenant # or the ESTUARY_TENANT environment variable
}

resource "estuary_source_mongodb" "shop" {
  name = "mongodb"
  # ...
}
```

The same configuration creates the capture `acmeCo/dev/mongodb/source-mongodb` in dev and `acmeCo/prod/mongodb/source-mongodb` in production. Capture bindings that leave `target` unset follow the capture name, landing in `acmeCo/dev/mongodb/shop/users` and `acmeCo/prod/mongodb/shop/users`.

> [!CAUTION]
> Two states managing the same capture or materialization overwrite each other on every apply, so never give two environments the same tenant.

> [!NOTE]
> Collection names aren't prefixed: a binding `target` or `source` you write is used as is. Build it from the environment instead of hard-coding it, ideally from the capture itself, as in the [Quick Start](#quick-start):

```hcl
bindings = [
  for binding in estuary_source_mongodb.shop.bindings : {
    source   = binding.target
    resource = { table = binding.resource.collection }
  }
]
```

### Keep Secrets Out of Code

Never commit tokens or passwords. Pass them through environment variables: the provider reads `FLOW_AUTH_TOKEN`, and Terraform reads each `TF_VAR_<name>` as the variable `<name>`:

```yaml
# GitHub Actions
env:
  FLOW_AUTH_TOKEN: ${{ secrets.ESTUARY_REFRESH_TOKEN }}
  TF_VAR_mongodb_password: ${{ secrets.MONGODB_PASSWORD }}
  TF_VAR_postgres_password: ${{ secrets.POSTGRES_PASSWORD }}
```

Connector secrets such as `password` and `private_key` are write-only: Terraform sends them to Estuary but never stores them in the state or shows them in plans.

### Share Pipelines with Modules

When the same source → destination pattern repeats, across pipelines, tenants, or environments that can't mirror each other, extract it into a module:

> [!IMPORTANT]
> Every module must declare `estuary/estuary` in its `required_providers`, or Terraform looks for a `hashicorp/estuary` provider that doesn't exist.

```hcl
# modules/mongodb-to-postgres/main.tf
terraform {
  required_providers {
    estuary = {
      source = "estuary/estuary"
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

---
page_title: "Multiple Environments"
description: |-
  How to run the same pipelines in several environments, such as dev and production.
---

# Multiple Environments

## Define Pipelines Once

Write your captures and materializations once, in one root module, and apply it once per environment with a variable file each. Production then runs what was tested in dev.

```
main.tf
variables.tf
backend.tf
dev.tfvars
production.tfvars
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

## Give Each Environment Its Own State

Shared state would make applying production replace what dev created. Use one of:

**Workspaces**, when all environments can share one backend. Name each variable file after its workspace:

```bash
terraform workspace select production
terraform apply -var-file="$(terraform workspace show).tfvars"
```

All workspaces share the backend and its credentials, so anyone who can read dev's state can read production's.

**One backend per environment**, for isolated state. Pass the backend settings to `terraform init` with a [partial configuration](https://developer.hashicorp.com/terraform/language/backend#partial-configuration) file:

```hcl
# backend.tf
terraform {
  backend "s3" {} # settings come from the .tfbackend file
}
```

```bash
ENV=production
terraform init -reconfigure -backend-config="$ENV.s3.tfbackend"
terraform apply -var-file="$ENV.tfvars"
```

Always pass `-reconfigure` when switching environments, or set a separate [`TF_DATA_DIR`](https://developer.hashicorp.com/terraform/cli/config/environment-variables#tf_data_dir) per environment.

**Separate root modules**, only when environments run different pipelines. Define each pipeline once in a [module](#share-pipelines-with-modules).

## Separate Environments with `tenant`

Separate states don't separate Estuary names: if dev and production both apply a capture named `mongodb` under the same prefix, they manage the same capture. Give each environment its own `tenant`:

```hcl
provider "estuary" {
  tenant     = var.tenant     # "acmeCo/dev/" or "acmeCo/prod/"
  data_plane = var.data_plane # null for the default data plane
}
```

`tenant` can be any prefix under your Estuary tenant that your token can access.

| Name | Prefixed | In dev | In production |
|---|---|---|---|
| Capture or materialization `name` | Yes | `acmeCo/dev/mongodb/source-mongodb` | `acmeCo/prod/mongodb/source-mongodb` |
| Capture binding without `target` | Yes, from the capture name | `acmeCo/dev/mongodb/shop/users` | `acmeCo/prod/mongodb/shop/users` |
| Capture binding `target` | No | as written | as written |
| Materialization binding `source` | No | as written | as written |

Don't hard-code collection names in bindings, or dev could read production data. Take them from the capture:

```hcl
resource "estuary_materialization_postgres" "shop" {
  name = "postgres"
  # ...
  bindings = [
    for binding in estuary_source_mongodb.shop.bindings : {
      source   = binding.target # follows the environment's tenant
      resource = { table = binding.resource.collection }
    }
  ]
}
```

With `auto_discover`, loop over the data source's `live_bindings` instead, to include the collections discovery adds.

> **Warning:** Changing `tenant`, or a `name` that changes the catalog name, replaces the capture or materialization: the apply deletes it and creates a new one. Each resource keeps the name it was created under in the read-only `catalog_name`, and the plan warns about each replacement. To manage another prefix alongside, use a second provider configuration with an [alias](https://developer.hashicorp.com/terraform/language/providers/configuration#alias-multiple-provider-configurations).

## Use Several Data Planes

The provider `data_plane` is the default data plane of every capture and materialization the provider creates. To run some of them elsewhere, such as one capture in a private data plane next to its database, set `data_plane` on those resources:

```hcl
provider "estuary" {
  tenant     = "acmeCo/prod/"
  data_plane = "ops/dp/public/example-data-plane" # default for new resources
}

resource "estuary_source_postgres" "shop" {
  name = "shop" # created in the provider data plane
  # ...
}

resource "estuary_source_postgres" "ledger" {
  name       = "ledger"
  data_plane = "ops/dp/private/acmeCo/example-data-plane"
  # ...
}
```

A new capture or materialization is created in its own `data_plane`, else in the provider one, else in the first data plane of your storage mapping. The data plane must be one your storage mapping lists, or Estuary rejects the publication. A capture discovers its source from its data plane, and the collections it creates are created there too.

Estuary can't move a live capture or materialization to another data plane, so the data plane only applies when it's created:

- Each resource shows the data plane it runs in as `data_plane`, read from Estuary on every refresh and on import.
- Changing the provider `data_plane`, or removing `data_plane` from a resource, changes nothing for the captures and materializations that already exist. Only the ones created afterwards use the new default.
- Setting a resource's `data_plane` to another one than it runs in fails the plan, and nothing is published or replaced. To move it, ask Estuary support to migrate it, which keeps its state, then set `data_plane` to the data plane it runs in.
- Replacing a resource by changing its catalog name creates a new capture or materialization, in the data plane a new one gets.

Keep a materialization in the data plane of the collections it reads: Estuary doesn't support reading collections across data planes.

## Share Pipelines with Modules

Write a repeated pipeline once as a module, and call it once per pipeline:

```hcl
# modules/mongodb-to-postgres/main.tf
terraform {
  required_providers {
    estuary = {
      source = "estuary/estuary" # required in every module, or Terraform looks for hashicorp/estuary
    }
  }
}

resource "estuary_source_mongodb" "source" {
  name = var.name

  config = {
    address  = var.mongodb_address
    user     = "flow_capture"
    password = var.mongodb_password
    database = "shop"
  }

  bindings = [
    for collection in var.collections : {
      resource = { database = "shop", collection = collection }
    }
  ]
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
  collections       = ["users", "orders", "products"]
}
```

With `tenant = "acmeCo/prod/"`, this creates `acmeCo/prod/shop/source-mongodb` and `acmeCo/prod/shop/materialize-postgres`.

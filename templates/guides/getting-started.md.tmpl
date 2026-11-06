---
page_title: "Getting Started"
description: |-
  Install the provider and create your first capture.
---

# Getting Started

## Installation

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

Then run `terraform init`. The provider runs [`flowctl`](https://docs.estuary.dev/concepts/flowctl/) to reach Estuary, so it must be installed where Terraform runs.

## Your First Capture

A capture of two MongoDB collections:

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

terraform init
terraform plan -out tfplan
terraform apply tfplan
```

This creates the capture `acmeCo/mongodb/source-mongodb` (`tenant` + `name` + the connector name) and the collections `acmeCo/mongodb/shop/users` and `acmeCo/mongodb/shop/orders`.

> **Important:** Read every plan before applying it. The apply makes Estuary match your configuration, so it can remove bindings, revert dashboard changes or trigger backfills.

## Next Steps

- [Bindings](https://registry.terraform.io/providers/estuary/estuary/latest/docs/guides/bindings): how bindings Terraform doesn't declare are handled, and how to materialize what a capture captures.
- [Secrets](https://registry.terraform.io/providers/estuary/estuary/latest/docs/guides/secrets): how passwords and keys stay out of state.
- [Multiple Environments](https://registry.terraform.io/providers/estuary/estuary/latest/docs/guides/multiple-environments): running the same pipelines in dev and production.

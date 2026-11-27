variable "mysql_password" {
  type      = string
  sensitive = true
}

# An incremental table and a small table read in full every night, with auto discovery of the shop schema.
resource "estuary_source_mysql_batch" "shop" {
  name = "shop"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    address  = "mysql.example.com:3306"
    user     = "flow_capture"
    password = var.mysql_password

    advanced = {
      poll             = "6h"
      discover_schemas = ["shop"]
      sslmode          = "verify_identity"
    }
  }

  bindings = [
    {
      resource = {
        name   = "orders"
        schema = "shop"
        table  = "orders"
        cursor = ["updated_at"]
        poll   = "15m"
      }
    },
    {
      resource = {
        name   = "countries"
        schema = "shop"
        table  = "countries"
        poll   = "daily at 02:00Z"
      }
      target = "acmeCo/shop/countries"
    },
  ]
}

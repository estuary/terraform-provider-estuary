variable "dekaf_token" {
  type      = string
  sensitive = true
}

# Two collections exposed to Tinybird, with deletions emitted as documents so Tinybird can apply them as hard deletes.
resource "estuary_materialization_dekaf_tinybird" "analytics" {
  name = "analytics"

  config = {
    token              = var.dekaf_token
    strict_topic_names = false
    deletions          = "cdc"
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { topic_name = "orders" }
    },
    {
      source   = "acmeCo/mongodb/shop/users"
      resource = { topic_name = "users" }
      fields = {
        recommended = "1"
        exclude     = ["internal_notes"]
      }
    },
  ]
}

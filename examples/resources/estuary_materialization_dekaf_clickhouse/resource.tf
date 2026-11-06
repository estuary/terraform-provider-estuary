variable "dekaf_token" {
  type      = string
  sensitive = true
}

# Two collections exposed as Kafka topics, the consumer authenticating with the token.
resource "estuary_materialization_dekaf_clickhouse" "analytics" {
  name = "analytics"

  config = {
    token              = var.dekaf_token
    strict_topic_names = false
    deletions          = "kafka"
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { topic_name = "orders" }
    },
    {
      source   = "acmeCo/mongodb/shop/users"
      resource = { topic_name = "users" }
    },
  ]
}

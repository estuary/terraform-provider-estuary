variable "webhook_token" {
  type      = string
  sensitive = true
}

# HTTPS endpoint authenticated by a Bearer token header.
resource "estuary_materialization_webhook" "orders" {
  name = "hooks/orders"

  config = {
    address = "https://hooks.example.com/estuary/"

    headers = {
      custom_headers = [
        { name = "Authorization", value = "Bearer ${var.webhook_token}" },
        { name = "X-Source", value = "estuary" },
      ]
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { relative_path = "orders" }
    },
    {
      source   = "acmeCo/mongodb/shop/customers"
      resource = { relative_path = "crm/customers" }

      fields = {
        recommended = "1"
        require     = ["flow_document"]
      }
    },
  ]
}

# Every collection of a capture, sent to an internal server from 2026 on.
resource "estuary_materialization_webhook" "events" {
  name = "hooks/events"

  source = {
    capture = "acmeCo/aurora/source-amazon-aurora-postgres"
  }

  config = {
    address = "https://123.4.567.89:3000/webhook/"
  }

  bindings = [
    {
      source     = "acmeCo/aurora/public/events"
      not_before = "2026-01-01T00:00:00Z"
      resource   = { relative_path = "events" }
    },
  ]
}

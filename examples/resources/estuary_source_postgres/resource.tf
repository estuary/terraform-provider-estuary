variable "postgres_password" {
  type      = string
  sensitive = true
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

# Password authentication with automatic discovery.
resource "estuary_source_postgres" "shop" {
  name = "postgres"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    address      = "123.4.567.89:5432"
    user         = "flow_capture"
    database     = "shop"
    history_mode = false

    credentials = {
      auth_type = "UserPassword"
      password  = var.postgres_password
    }

    discovery_filters = {
      include_schemas = ["public"]
    }
  }

  bindings = [
    {
      resource = {
        namespace = "public"
        stream    = "orders"
      }
      target = "acmeCo/shop/orders"
    },
  ]
}

# Server in a private network, reached through an SSH tunnel.
resource "estuary_source_postgres" "private" {
  name = "postgres/private"

  config = {
    address      = "123.4.567.91:5432"
    user         = "flow_capture"
    database     = "shop"
    history_mode = false

    credentials = {
      auth_type = "UserPassword"
      password  = var.postgres_password
    }

    network_tunnel = {
      ssh_forwarding = {
        ssh_endpoint = "ssh://admin@123.4.567.90:22"
        private_key  = var.ssh_private_key
      }
    }

    advanced = {
      publication_name = "estuary_publication"
      slot_name        = "estuary_slot"
      sslmode          = "require"
      source_tag       = "shop-private"
    }
  }

  bindings = [
    {
      resource = {
        namespace = "public"
        stream    = "payments"
      }
    },
  ]
}

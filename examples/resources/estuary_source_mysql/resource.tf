variable "mysql_password" {
  type      = string
  sensitive = true
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

# Password authentication with automatic discovery.
resource "estuary_source_mysql" "shop" {
  name = "mysql"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    address      = "123.4.567.89:3306"
    user         = "flow_capture"
    history_mode = false

    credentials = {
      auth_type = "UserPassword"
      password  = var.mysql_password
    }

    discovery_filters = {
      include_schemas = ["shop"]
    }
  }

  bindings = [
    {
      resource = {
        namespace = "shop"
        stream    = "orders"
      }
      target = "acmeCo/shop/orders"
    },
  ]
}

# Server in a private network, reached through an SSH tunnel.
resource "estuary_source_mysql" "private" {
  name = "mysql/private"

  config = {
    address      = "123.4.567.91:3306"
    user         = "flow_capture"
    history_mode = false

    credentials = {
      auth_type = "UserPassword"
      password  = var.mysql_password
    }

    network_tunnel = {
      ssh_forwarding = {
        ssh_endpoint = "ssh://admin@123.4.567.90:22"
        private_key  = var.ssh_private_key
      }
    }

    advanced = {
      sslmode    = "verify_identity"
      source_tag = "shop-private"
    }
  }

  bindings = [
    {
      resource = {
        namespace = "shop"
        stream    = "payments"
      }
    },
  ]
}

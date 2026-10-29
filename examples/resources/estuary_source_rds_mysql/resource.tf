variable "mysql_password" {
  type      = string
  sensitive = true
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

# Password authentication with automatic discovery.
resource "estuary_source_rds_mysql" "shop" {
  name = "rds"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    address      = "shop.123456789.us-east-1.rds.amazonaws.com:3306"
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
        stream    = "users"
      }
    },
    {
      resource = {
        namespace = "shop"
        stream    = "orders"
      }
      target = "acmeCo/shop/orders"
    },
  ]
}

# IAM authentication capturing every change event.
resource "estuary_source_rds_mysql" "events" {
  name = "rds/events"

  config = {
    address      = "events.123456789.us-east-1.rds.amazonaws.com:3306"
    user         = "flow_capture"
    timezone     = "America/New_York"
    history_mode = true

    credentials = {
      auth_type    = "AWSIAM"
      aws_region   = "us-east-1"
      aws_role_arn = "arn:aws:iam::123456789012:role/estuary-rds-capture"
    }
  }

  bindings = [
    {
      resource = {
        namespace = "events"
        stream    = "accounts"
        priority  = 10
      }
    },
    {
      resource = {
        namespace = "events"
        stream    = "audit_log"
      }
      backfill = {
        mode = "Only Changes"
      }
    },
  ]
}

# SSH tunnel with a verified server certificate.
resource "estuary_source_rds_mysql" "private" {
  name = "rds/private"

  config = {
    address      = "private.123456789.us-east-1.rds.amazonaws.com:3306"
    user         = "flow_capture"
    history_mode = false

    credentials = {
      auth_type = "UserPassword"
      password  = var.mysql_password
    }

    network_tunnel = {
      ssh_forwarding = {
        ssh_endpoint = "ssh://ec2-user@ec2-123-4-567-89.compute-1.amazonaws.com:22"
        private_key  = var.ssh_private_key
      }
    }

    advanced = {
      sslmode        = "verify_identity"
      skip_backfills = "shop.events"
      source_tag     = "rds-private"
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

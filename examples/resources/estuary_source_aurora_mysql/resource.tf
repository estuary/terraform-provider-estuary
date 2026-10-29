variable "mysql_password" {
  type      = string
  sensitive = true
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

# Password authentication with automatic discovery.
resource "estuary_source_aurora_mysql" "shop" {
  name = "aurora"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    address      = "shop.cluster-123456789.us-east-1.rds.amazonaws.com:3306"
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
resource "estuary_source_aurora_mysql" "events" {
  name = "aurora/events"

  config = {
    address      = "events.cluster-123456789.us-east-1.rds.amazonaws.com:3306"
    user         = "flow_capture"
    timezone     = "America/New_York"
    history_mode = true

    credentials = {
      auth_type    = "AWSIAM"
      aws_region   = "us-east-1"
      aws_role_arn = "arn:aws:iam::123456789012:role/estuary-aurora-capture"
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
resource "estuary_source_aurora_mysql" "private" {
  name = "aurora/private"

  config = {
    address      = "private.cluster-123456789.us-east-1.rds.amazonaws.com:3306"
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
      source_tag     = "aurora-private"
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

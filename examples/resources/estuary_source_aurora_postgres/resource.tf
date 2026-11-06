variable "postgres_password" {
  type      = string
  sensitive = true
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

# Password authentication with automatic discovery.
resource "estuary_source_aurora_postgres" "shop" {
  name = "aurora"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    address      = "shop.cluster-123456789.us-east-1.rds.amazonaws.com:5432"
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
        stream    = "users"
      }
    },
    {
      resource = {
        namespace = "public"
        stream    = "orders"
      }
      target = "acmeCo/shop/orders"
    },
  ]
}

# IAM authentication capturing every change event.
resource "estuary_source_aurora_postgres" "events" {
  name = "aurora/events"

  config = {
    address      = "events.cluster-123456789.us-east-1.rds.amazonaws.com:5432"
    user         = "flow_capture"
    database     = "events"
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
        namespace = "public"
        stream    = "accounts"
        priority  = 10
      }
    },
    {
      resource = {
        namespace = "public"
        stream    = "audit_log"
      }
      backfill = {
        mode = "Only Changes"
      }
    },
  ]
}

# SSH tunnel with a dedicated publication and slot.
resource "estuary_source_aurora_postgres" "private" {
  name = "aurora/private"

  config = {
    address      = "private.cluster-123456789.us-east-1.rds.amazonaws.com:5432"
    user         = "flow_capture"
    database     = "shop"
    history_mode = false

    credentials = {
      auth_type = "UserPassword"
      password  = var.postgres_password
    }

    network_tunnel = {
      ssh_forwarding = {
        ssh_endpoint = "ssh://ec2-user@ec2-123-4-567-89.compute-1.amazonaws.com:22"
        private_key  = var.ssh_private_key
      }
    }

    advanced = {
      publication_name = "estuary_publication"
      slot_name        = "estuary_slot"
      sslmode          = "require"
      source_tag       = "aurora-private"
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

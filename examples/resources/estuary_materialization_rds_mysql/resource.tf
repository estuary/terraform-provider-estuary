variable "mysql_password" {
  type      = string
  sensitive = true
}

variable "mysql_server_ca" {
  type = string
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

variable "dbt_api_key" {
  type      = string
  sensitive = true
}

# Per-binding field selection and delta updates.
resource "estuary_materialization_rds_mysql" "analytics" {
  name = "analytics"

  config = {
    address  = "analytics.123456789.us-east-1.rds.amazonaws.com:3306"
    user     = "flow_materialize"
    password = var.mysql_password
    database = "analytics"
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/users"
      resource = { table = "users" }
    },
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { table = "orders" }

      fields = {
        recommended = "1"
        exclude     = ["internal_notes"]
        group_by    = ["customer_id", "order_id"]
      }
    },
    {
      source     = "acmeCo/mongodb/shop/events"
      not_before = "2026-01-01T00:00:00Z"
      resource = {
        table         = "events"
        delta_updates = true
      }
    },
  ]
}

# Verified TLS following a capture, with hard deletes.
resource "estuary_materialization_rds_mysql" "replica" {
  name = "rds/replica"

  source = {
    capture            = "acmeCo/rds/source-amazon-rds-mysql"
    delta_updates      = false
    fields_recommended = "1"
  }

  on_incompatible_schema_change = "disableBinding"

  config = {
    address     = "replica.123456789.us-east-1.rds.amazonaws.com:3306"
    user        = "flow_materialize"
    password    = var.mysql_password
    database    = "replica"
    timezone    = "+00:00"
    hard_delete = true

    advanced = {
      sslmode       = "verify_identity"
      ssl_server_ca = var.mysql_server_ca
    }
  }

  bindings = [
    {
      source = "acmeCo/rds/shop/orders"
      resource = {
        table                       = "orders"
        additional_table_create_sql = "CREATE INDEX orders_customer_id ON orders (customer_id);"
      }

      fields = { recommended = "unlimited" }
    },
  ]
}

# SSH tunnel with a dbt Cloud job trigger.
resource "estuary_materialization_rds_mysql" "private" {
  name = "warehouse"

  config = {
    address  = "warehouse.123456789.us-east-1.rds.amazonaws.com:3306"
    user     = "flow_materialize"
    password = var.mysql_password
    database = "warehouse"

    network_tunnel = {
      ssh_forwarding = {
        ssh_endpoint = "ssh://ec2-user@ec2-123-4-567-89.compute-1.amazonaws.com:22"
        private_key  = var.ssh_private_key
      }
    }

    dbt_job_trigger = {
      access_url = "https://cloud.getdbt.com"
      account_id = "67890"
      job_id     = "12345"
      api_key    = var.dbt_api_key
      mode       = "skip"
      interval   = "1h"
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { table = "orders" }
      backfill = { trigger = 1 }
    },
  ]
}

variable "postgres_password" {
  type      = string
  sensitive = true
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

variable "dbt_api_key" {
  type      = string
  sensitive = true
}

# Materialization of three collections with password authentication: one into
# another schema with a reduced field selection keyed by other fields, and one
# with delta updates that skips the documents published before 2026.
resource "estuary_materialization_postgres" "analytics" {
  name = "analytics"

  config = {
    address  = "123.4.567.89:5432"
    user     = "flow_materialize"
    database = "analytics"
    schema   = "estuary"

    credentials = {
      auth_type = "UserPassword"
      password  = var.postgres_password
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/users"
      resource = { table = "users" }
    },
    {
      source = "acmeCo/mongodb/shop/orders"
      resource = {
        table  = "orders"
        schema = "sales"
      }

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

# AWS IAM authentication with hard deletes, following a capture so its new
# collections are materialized as they're added, into tables named after their
# source schema and table. An incompatible schema change disables the affected
# binding instead of backfilling it, and one table materializes every field and
# gets an index.
resource "estuary_materialization_postgres" "replica" {
  name = "aurora/replica"

  source = {
    capture            = "acmeCo/aurora/source-amazon-aurora-postgres"
    delta_updates      = false
    fields_recommended = "1"
  }

  target_naming = {
    strategy = "matchSourceStructure"
  }

  on_incompatible_schema_change = "disableBinding"

  config = {
    address     = "replica.cluster-123456789.us-east-1.rds.amazonaws.com:5432"
    user        = "flow_materialize"
    database    = "replica"
    hard_delete = true

    credentials = {
      auth_type    = "AWSIAM"
      aws_region   = "us-east-1"
      aws_role_arn = "arn:aws:iam::123456789012:role/estuary-materialize"
    }
  }

  bindings = [
    {
      source = "acmeCo/aurora/public/orders"
      resource = {
        table                       = "orders"
        schema                      = "public"
        additional_table_create_sql = "CREATE INDEX orders_customer_id ON public.orders (customer_id);"
      }

      fields = { recommended = "unlimited" }
    },
  ]
}

# Connection through an SSH bastion host, triggering a dbt Cloud job when new
# data is materialized.
resource "estuary_materialization_postgres" "private" {
  name = "warehouse"

  config = {
    address  = "123.4.567.89:5432"
    user     = "flow_materialize"
    database = "warehouse"

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

    dbt_job_trigger = {
      access_url = "https://cloud.getdbt.com"
      account_id = "67890"
      job_id     = "12345"
      api_key    = var.dbt_api_key
      mode       = "skip"
      interval   = "1h"
    }

    advanced = {
      sslmode = "require"
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

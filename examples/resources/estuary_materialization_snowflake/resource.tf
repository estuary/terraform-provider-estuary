variable "snowflake_private_key" {
  type      = string
  sensitive = true
}

variable "dbt_api_key" {
  type      = string
  sensitive = true
}

# Key-pair authentication with a business-hours sync schedule, a clustered table and a delta updates table.
resource "estuary_materialization_snowflake" "warehouse" {
  name = "warehouse"

  config = {
    host           = "orgname-accountname.snowflakecomputing.com"
    database       = "ESTUARY_DB"
    schema         = "ESTUARY_SCHEMA"
    warehouse      = "ESTUARY_WH"
    role           = "ESTUARY_ROLE"
    timestamp_type = "TIMESTAMP_LTZ"

    credentials = {
      auth_type   = "jwt"
      user        = "ESTUARY_USER"
      private_key = var.snowflake_private_key
    }

    sync_schedule = {
      sync_frequency         = "15m"
      timezone               = "America/New_York"
      fast_sync_start_time   = "09:00"
      fast_sync_stop_time    = "17:00"
      fast_sync_enabled_days = "M-F"
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
        table      = "orders"
        schema     = "SALES"
        clustering = "order_date, customer_id"
      }

      fields = {
        recommended = "1"
        exclude     = ["internal_notes"]
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

# Near real-time delta updates following a capture, with hard deletes and a dbt Cloud job trigger.
resource "estuary_materialization_snowflake" "streaming" {
  name = "streaming"

  source = {
    capture            = "acmeCo/aurora/source-amazon-aurora-postgres"
    delta_updates      = true
    fields_recommended = "1"
  }

  target_naming = {
    strategy = "matchSourceStructure"
  }

  config = {
    host           = "orgname-accountname.snowflakecomputing.com"
    database       = "ESTUARY_DB"
    schema         = "RAW"
    warehouse      = "ESTUARY_WH"
    timestamp_type = "TIMESTAMP_TZ"
    hard_delete    = true

    credentials = {
      auth_type   = "jwt"
      user        = "ESTUARY_USER"
      private_key = var.snowflake_private_key
    }

    sync_schedule = {
      sync_frequency = "0s"
    }

    dbt_job_trigger = {
      job_id     = "12345"
      account_id = "67890"
      api_key    = var.dbt_api_key
      access_url = "https://cloud.getdbt.com"
      mode       = "skip"
      interval   = "1h"
    }
  }

  bindings = [
    {
      source   = "acmeCo/aurora/public/orders"
      resource = { table = "orders", delta_updates = true }
    },
  ]
}

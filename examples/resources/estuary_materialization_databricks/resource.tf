variable "databricks_client_id" {
  type      = string
  sensitive = true
}

variable "databricks_client_secret" {
  type      = string
  sensitive = true
}

variable "databricks_token" {
  type      = string
  sensitive = true
}

variable "dbt_api_key" {
  type      = string
  sensitive = true
}

# A service principal with a business-hours sync schedule, a table in another schema and a delta updates table.
resource "estuary_materialization_databricks" "lakehouse" {
  name = "lakehouse"

  config = {
    address      = "dbc-abcdef12-3456.cloud.databricks.com"
    http_path    = "/sql/1.0/warehouses/abcdef1234567890"
    catalog_name = "main"
    schema_name  = "estuary"

    credentials = {
      auth_type     = "OAuth2 M2M"
      client_id     = var.databricks_client_id
      client_secret = var.databricks_client_secret
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
        table                       = "orders"
        schema                      = "sales"
        additional_table_create_sql = "ALTER TABLE sales.orders SET TBLPROPERTIES ('delta.enableChangeDataFeed' = 'true')"
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

# Following a capture with a personal access token, hard deletes and a dbt Cloud job trigger.
resource "estuary_materialization_databricks" "raw" {
  name = "raw"

  source = {
    capture            = "acmeCo/aurora/source-amazon-aurora-postgres"
    fields_recommended = "1"
  }

  target_naming = {
    strategy = "matchSourceStructure"
  }

  config = {
    address      = "dbc-abcdef12-3456.cloud.databricks.com"
    http_path    = "/sql/1.0/warehouses/abcdef1234567890"
    catalog_name = "main"
    schema_name  = "raw"
    hard_delete  = true

    credentials = {
      auth_type             = "PAT"
      personal_access_token = var.databricks_token
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
      resource = { table = "orders" }
    },
  ]
}

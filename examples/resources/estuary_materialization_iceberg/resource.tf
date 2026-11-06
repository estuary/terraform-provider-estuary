variable "catalog_credential" {
  type      = string
  sensitive = true
}

variable "emr_secret_access_key" {
  type      = string
  sensitive = true
}

variable "catalog_secret_access_key" {
  type      = string
  sensitive = true
}

variable "dbt_api_key" {
  type      = string
  sensitive = true
}

# AWS Glue with IAM authentication shared by the EMR jobs, Glue optimizers and a sync schedule.
resource "estuary_materialization_iceberg" "lake" {
  name = "lake"

  config = {
    url           = "https://glue.us-east-1.amazonaws.com/iceberg"
    warehouse     = "123456789012"
    namespace     = "analytics"
    base_location = "s3://acme-lake/iceberg/"

    credentials = {
      auth_type    = "AWS IAM"
      aws_role_arn = "arn:aws:iam::123456789012:role/estuary-iceberg"
      aws_region   = "us-east-1"
    }

    compute = {
      compute_type       = "AWS EMR Serverless"
      region             = "us-east-1"
      application_id     = "00abcdefghijklmn"
      execution_role_arn = "arn:aws:iam::123456789012:role/estuary-emr-execution"
      bucket             = "acme-iceberg-staging"
      bucket_path        = "staging"

      credentials = {
        auth_type = "UseCatalogAuth"
      }
    }

    sync_schedule = {
      sync_frequency         = "5m"
      timezone               = "America/New_York"
      fast_sync_start_time   = "09:00"
      fast_sync_stop_time    = "17:00"
      fast_sync_enabled_days = "M-F"
    }

    glue_optimizers = {
      execution_role_arn      = "arn:aws:iam::123456789012:role/estuary-glue-optimizer"
      enable_compaction       = true
      enable_retention        = true
      snapshot_retention_days = 7
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
        table     = "orders"
        namespace = "sales"

        additional_table_properties = {
          "write.parquet.compression-codec" = "zstd"
        }
      }

      fields = {
        recommended = "1"
        exclude     = ["internal_notes"]
      }
    },
  ]
}

# Snowflake Open Catalog with OAuth client credentials, following a capture with upper-case identifiers.
resource "estuary_materialization_iceberg" "open_catalog" {
  name = "snowflake/open-catalog"

  source = {
    capture            = "acmeCo/aurora/source-amazon-aurora-postgres"
    fields_recommended = "1"
  }

  target_naming = {
    strategy = "matchSourceStructure"
  }

  config = {
    url         = "https://123456789.snowflakecomputing.com/polaris/api/catalog"
    warehouse   = "analytics_catalog"
    namespace   = "estuary"
    hard_delete = true

    credentials = {
      auth_type         = "OAuth 2.0 Client Credentials"
      oauth2_server_uri = "v1/oauth/tokens"
      credential        = var.catalog_credential
      scope             = "PRINCIPAL_ROLE:estuary"
    }

    compute = {
      compute_type           = "AWS EMR Serverless"
      region                 = "us-east-1"
      application_id         = "00abcdefghijklmn"
      execution_role_arn     = "arn:aws:iam::123456789012:role/estuary-emr-execution"
      bucket                 = "acme-iceberg-staging"
      systems_manager_prefix = "/estuary/"

      credentials = {
        auth_type             = "AWSAccessKey"
        aws_access_key_id     = "AKIAXXXXXXXXXXXXXXXX"
        aws_secret_access_key = var.emr_secret_access_key
      }
    }

    advanced = {
      table_identifier_case = "uppercase"
      field_name_case       = "uppercase"
    }
  }

  bindings = [
    {
      source = "acmeCo/aurora/public/orders"
      resource = {
        table     = "orders"
        namespace = "public"
      }
    },
  ]
}

# S3 Tables with AWS SigV4, lowercase columns, Spark overrides and a dbt Cloud job trigger.
resource "estuary_materialization_iceberg" "table_bucket" {
  name = "s3tables"

  config = {
    url       = "https://s3tables.us-east-1.amazonaws.com/iceberg"
    warehouse = "arn:aws:s3tables:us-east-1:123456789012:bucket/acme-tables"
    namespace = "analytics"

    credentials = {
      auth_type             = "AWS SigV4"
      aws_access_key_id     = "AKIAXXXXXXXXXXXXXXXX"
      aws_secret_access_key = var.catalog_secret_access_key
      aws_region            = "us-east-1"
    }

    compute = {
      compute_type       = "AWS EMR Serverless"
      region             = "us-east-1"
      application_id     = "00abcdefghijklmn"
      execution_role_arn = "arn:aws:iam::123456789012:role/estuary-emr-execution"
      bucket             = "acme-iceberg-staging"

      credentials = {
        auth_type = "UseCatalogAuth"
      }

      spark_job_properties = [
        {
          key   = "spark.dynamicAllocation.maxExecutors"
          value = "20"
        },
      ]
    }

    dbt_job_trigger = {
      access_url = "https://cloud.getdbt.com"
      account_id = "67890"
      job_id     = "12345"
      api_key    = var.dbt_api_key
    }

    advanced = {
      field_name_case = "lowercase"
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/events"
      resource = { table = "events" }
      backfill = { trigger = 1 }
    },
  ]
}

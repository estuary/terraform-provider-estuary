variable "aws_secret_access_key" {
  type      = string
  sensitive = true
}

variable "catalog_credential" {
  type      = string
  sensitive = true
}

variable "catalog_token" {
  type      = string
  sensitive = true
}

# Cross-account AWS Glue catalog with IAM authentication.
resource "estuary_materialization_s3_iceberg" "events" {
  name = "lake/events"

  config = {
    bucket          = "acme-lake"
    prefix          = "iceberg"
    region          = "us-east-1"
    namespace       = "events"
    upload_interval = "PT15M"

    credentials = {
      auth_type    = "AWSIAM"
      aws_role_arn = "arn:aws:iam::123456789012:role/estuary-s3-iceberg"
      aws_region   = "us-east-1"
    }

    catalog = {
      catalog_type = "AWS Glue"
      glue_id      = "123456789013"
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/page_views"
      resource = { table = "page_views" }
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

# REST catalog with an access key, following a capture with Iceberg format v3 options.
resource "estuary_materialization_s3_iceberg" "changes" {
  name = "lake/changes"

  source = {
    capture = "acmeCo/aurora/source-amazon-aurora-postgres"
  }

  config = {
    bucket          = "acme-lake"
    region          = "us-east-1"
    namespace       = "changes"
    upload_interval = "PT5M"

    credentials = {
      auth_type             = "AWSAccessKey"
      aws_access_key_id     = "AKIAXXXXXXXXXXXXXXXX"
      aws_secret_access_key = var.aws_secret_access_key
    }

    catalog = {
      catalog_type = "Iceberg REST Server"
      uri          = "https://123456789.snowflakecomputing.com/polaris/api/catalog"
      warehouse    = "lake_catalog"
      credential   = var.catalog_credential
      scope        = "PRINCIPAL_ROLE:ALL"
    }

    advanced = {
      nanosecond_timestamps = true
      variant_columns       = true
    }
  }

  bindings = [
    {
      source     = "acmeCo/aurora/public/orders"
      not_before = "2026-01-01T00:00:00Z"
      resource   = { table = "orders" }
    },
  ]
}

# S3-compatible storage with a REST catalog authenticated by a Bearer token.
resource "estuary_materialization_s3_iceberg" "minio" {
  name = "lake/minio"

  config = {
    bucket          = "lake"
    region          = "us-east-1"
    namespace       = "raw"
    upload_interval = "PT1H"
    s3_endpoint     = "https://123.4.567.89:9000"

    credentials = {
      auth_type             = "AWSAccessKey"
      aws_access_key_id     = "AKIAXXXXXXXXXXXXXXXX"
      aws_secret_access_key = var.aws_secret_access_key
    }

    catalog = {
      catalog_type = "Iceberg REST Server"
      uri          = "https://123.4.567.89:8181/catalog"
      warehouse    = "lake"
      token        = var.catalog_token
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

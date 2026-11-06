variable "aws_secret_access_key" {
  type      = string
  sensitive = true
}

# Access key authentication with automatic discovery.
resource "estuary_source_dynamodb" "shop" {
  name = "dynamodb"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    region = "us-east-1"

    credentials = {
      auth_type             = "AWSAccessKey"
      aws_access_key_id     = "AKIAXXXXXXXXXXXXXXXX"
      aws_secret_access_key = var.aws_secret_access_key
    }
  }

  bindings = [
    {
      resource = {
        table = "orders"
      }
    },
    {
      resource = {
        table = "customers"
      }
      target = "acmeCo/shop/customers"
    },
  ]
}

# IAM role authentication with a limited backfill read capacity.
resource "estuary_source_dynamodb" "events" {
  name = "dynamodb/events"

  config = {
    region = "eu-west-1"

    credentials = {
      auth_type    = "AWSIAM"
      aws_role_arn = "arn:aws:iam::123456789012:role/estuary-dynamodb"
      aws_region   = "eu-west-1"
    }
  }

  bindings = [
    {
      resource = {
        table          = "events"
        rcu_allocation = 1000
      }
    },
  ]
}

# DynamoDB-compatible endpoint, such as DynamoDB Local.
resource "estuary_source_dynamodb" "local" {
  name = "dynamodb/local"

  config = {
    region = "us-east-1"

    credentials = {
      auth_type             = "AWSAccessKey"
      aws_access_key_id     = "local"
      aws_secret_access_key = var.aws_secret_access_key
    }

    advanced = {
      endpoint          = "https://dynamodb.example.com"
      backfill_segments = 4
      scan_limit        = 500
      idle_backoff_max  = "1m"
    }
  }

  bindings = [
    {
      resource = {
        table = "orders"
      }
    },
  ]
}

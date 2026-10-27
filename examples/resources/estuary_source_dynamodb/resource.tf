variable "aws_secret_access_key" {
  type      = string
  sensitive = true
}

# Capture of two tables authenticated with an access key, adding every new
# table of the region as it's discovered.
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
      aws_access_key_id     = "AKIAIOSFODNN7EXAMPLE"
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

# Capture authenticated with an IAM role, limiting the read capacity the
# backfill of a large table consumes.
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

# Capture from a DynamoDB-compatible API, such as DynamoDB Local, with
# advanced backfill and polling options.
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

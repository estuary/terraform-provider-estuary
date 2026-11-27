variable "stripe_api_key" {
  type      = string
  sensitive = true
}

# Two resources, with auto discovery adding every new one.
resource "estuary_source_stripe" "payments" {
  name = "payments"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    start_date = "2024-01-01T00:00:00Z"

    credentials = {
      credentials_title = "Private App Credentials"
      access_token      = var.stripe_api_key
    }
  }

  bindings = [
    {
      resource = {
        name     = "Charges"
        interval = "PT5M"
      }
    },
    {
      resource = {
        name     = "Customers"
        interval = "PT5M"
      }
      target = "acmeCo/payments/stripe-customers"
    },
  ]
}

# A platform account with its connected accounts, a pinned API version and a nightly backfill.
resource "estuary_source_stripe" "platform" {
  name = "payments/platform"

  config = {
    capture_connected_accounts = true

    credentials = {
      credentials_title = "Private App Credentials"
      access_token      = var.stripe_api_key
    }

    advanced = {
      api_version             = "2024-06-20"
      incremental_window_size = "PT1H"
    }
  }

  bindings = [
    {
      resource = {
        name     = "Accounts"
        interval = "PT5M"
        schedule = "0 0 * * *"
      }
    },
  ]
}

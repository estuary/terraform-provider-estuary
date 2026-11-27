variable "meta_app_id" {
  type      = string
  sensitive = true
}

variable "meta_app_secret" {
  type      = string
  sensitive = true
}

variable "meta_access_token" {
  type      = string
  sensitive = true
}

# Two ad accounts with the access token of your own Meta app and a custom insights report.
resource "estuary_source_facebook_marketing" "ads" {
  name = "ads"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    account_ids              = "123456789,987654321"
    start_date               = "2024-01-01T00:00:00Z"
    insights_lookback_window = 28

    custom_insights = [
      {
        name              = "spend_by_age"
        level             = "campaign"
        fields            = "account_id,campaign_id,impressions,clicks,spend"
        breakdowns        = "age,gender"
        action_breakdowns = "action_type"
      },
    ]

    credentials = {
      credentials_title = "OAuth Credentials"
      client_id         = var.meta_app_id
      client_secret     = var.meta_app_secret
      access_token      = var.meta_access_token
    }
  }

  bindings = [
    {
      resource = {
        name     = "campaigns"
        interval = "PT1H"
      }
    },
    {
      resource = {
        name     = "spend_by_age"
        interval = "PT1H"
      }
      target = "acmeCo/ads/spend-by-age"
    },
  ]
}

# A capture created through the dashboard login, imported with its secrets left out.
resource "estuary_source_facebook_marketing" "imported" {
  name = "ads/imported"

  config = {
    account_ids = "123456789"
    start_date  = "2024-01-01T00:00:00Z"

    credentials = {
      credentials_title = "OAuth Credentials"
    }
  }

  bindings = []
}

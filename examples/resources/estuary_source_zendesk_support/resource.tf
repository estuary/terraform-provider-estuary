variable "zendesk_api_token" {
  type      = string
  sensitive = true
}

# An agent's email and API token, with auto discovery adding every new resource.
resource "estuary_source_zendesk_support" "support" {
  name = "support"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    subdomain  = "acme"
    start_date = "2024-01-01T00:00:00Z"

    credentials = {
      credentials_title = "Email & API Token"
      username          = "support-bot@acme.example"
      password          = var.zendesk_api_token
    }
  }

  bindings = [
    {
      resource = {
        name     = "tickets"
        interval = "PT5M"
      }
    },
    {
      resource = {
        name     = "users"
        interval = "PT5M"
      }
      target = "acmeCo/support/zendesk-users"
    },
  ]
}

# A capture created through the dashboard login, imported with its secrets left out.
resource "estuary_source_zendesk_support" "imported" {
  name = "support/imported"

  config = {
    subdomain = "acme"

    credentials = {
      credentials_title = "OAuth Credentials"
    }
  }

  bindings = []
}

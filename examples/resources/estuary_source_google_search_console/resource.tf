variable "search_console_service_account" {
  type      = string
  sensitive = true
}

variable "google_client_id" {
  type      = string
  sensitive = true
}

variable "google_client_secret" {
  type      = string
  sensitive = true
}

variable "google_refresh_token" {
  type      = string
  sensitive = true
}

# Two properties with a service account key and a custom report.
resource "estuary_source_google_search_console" "seo" {
  name = "seo"

  config = {
    site_urls  = ["https://www.acme.example/", "sc-domain:acme.example"]
    start_date = "2024-01-01"

    custom_reports = jsonencode([
      { name = "by_country_and_device", dimensions = ["date", "country", "device"] },
    ])

    credentials = {
      auth_type            = "Service"
      service_account_info = var.search_console_service_account
      email                = "admin@acme.example"
    }
  }

  bindings = [
    {
      resource = {
        stream       = "search_analytics_by_date"
        sync_mode    = "incremental"
        cursor_field = ["date"]
      }
    },
    {
      resource = {
        stream       = "by_country_and_device"
        sync_mode    = "incremental"
        cursor_field = ["date"]
      }
      target = "acmeCo/seo/by-country-and-device"
    },
    {
      resource = {
        stream    = "sites"
        sync_mode = "full_refresh"
      }
    },
  ]
}

# The OAuth client of your own Google Cloud project.
resource "estuary_source_google_search_console" "oauth" {
  name = "seo/oauth"

  config = {
    site_urls  = ["sc-domain:acme.example"]
    start_date = "2024-01-01"

    credentials = {
      auth_type     = "Client"
      client_id     = var.google_client_id
      client_secret = var.google_client_secret
      refresh_token = var.google_refresh_token
    }
  }

  bindings = [
    {
      resource = {
        stream       = "search_analytics_by_page"
        sync_mode    = "incremental"
        cursor_field = ["date"]
      }
    },
  ]
}

# A capture created through the dashboard login, imported with its secrets left out.
resource "estuary_source_google_search_console" "imported" {
  name = "seo/imported"

  config = {
    site_urls  = ["sc-domain:acme.example"]
    start_date = "2024-01-01"

    credentials = {
      auth_type = "Client"
    }
  }

  bindings = []
}

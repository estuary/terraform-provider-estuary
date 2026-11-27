variable "google_play_service_account" {
  type      = string
  sensitive = true
}

# The installs and reviews reports of a developer account.
resource "estuary_source_google_play" "apps" {
  name = "apps"

  config = {
    bucket     = "pubsite_prod_1234567890123456789"
    start_date = "2024-01-01T00:00:00Z"

    credentials = {
      credentials_title = "Google Service Account"
      service_account   = var.google_play_service_account
    }
  }

  bindings = [
    {
      resource = {
        name     = "installs"
        interval = "PT1H"
      }
    },
    {
      resource = {
        name     = "reviews"
        interval = "PT1H"
      }
      target = "acmeCo/apps/play-reviews"
    },
  ]
}

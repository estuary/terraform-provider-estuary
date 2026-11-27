variable "app_store_private_key" {
  type      = string
  sensitive = true
}

variable "app_store_key_id" {
  type      = string
  sensitive = true
}

variable "app_store_issuer_id" {
  type      = string
  sensitive = true
}

# The customer reviews of two apps, with auto discovery adding every other stream.
resource "estuary_source_apple_app_store" "apps" {
  name = "apps"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    app_ids = ["1234567890", "9876543210"]

    credentials = {
      credentials_title = "Private App Credentials"
      private_key       = var.app_store_private_key
      key_id            = var.app_store_key_id
      issuer_id         = var.app_store_issuer_id
    }
  }

  bindings = [
    {
      resource = {
        name     = "app_reviews"
        interval = "PT1H"
      }
    },
  ]
}

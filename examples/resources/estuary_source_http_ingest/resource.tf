variable "webhook_token" {
  type      = string
  sensitive = true
}

variable "zoom_secret_token" {
  type      = string
  sensitive = true
}

variable "webhook_signing_secret" {
  type      = string
  sensitive = true
}

# Two paths behind a bearer token, with the delivery ID of one sender as document ID.
resource "estuary_source_http_ingest" "webhooks" {
  name = "webhooks"

  config = {
    paths                = ["/github", "/vendors/{vendorId}/products"]
    allowed_cors_origins = []
    require_auth_token   = var.webhook_token
  }

  bindings = [
    {
      resource = {
        stream         = "/github"
        path           = "/github"
        id_from_header = "X-GitHub-Delivery"
      }
      target = "acmeCo/webhooks/github"
    },
    {
      resource = {
        stream = "/vendors/{vendorId}/products"
        path   = "/vendors/{vendorId}/products"
      }
      target = "acmeCo/webhooks/vendor-products"
    },
  ]
}

# Verifying the signature of Zoom webhooks.
resource "estuary_source_http_ingest" "zoom" {
  name = "webhooks/zoom"

  config = {
    paths = ["/zoom"]

    signature_config = {
      provider   = "zoom"
      public_key = var.zoom_secret_token
    }
  }

  bindings = [
    {
      resource = {
        stream = "/zoom"
        path   = "/zoom"
      }
      target = "acmeCo/webhooks/zoom"
    },
  ]
}

# Verifying signatures a sender computes its own way, an HMAC of the timestamp and the body.
resource "estuary_source_http_ingest" "signed" {
  name = "webhooks/signed"

  config = {
    paths                = ["/events"]
    allowed_cors_origins = ["https://app.acme.example"]

    signature_config = {
      provider                = "custom"
      algorithm               = "hmac_sha256"
      public_key              = var.webhook_signing_secret
      signature_header        = "X-Signature"
      signature_encoding      = "base64"
      signing_string_template = "{TIMESTAMP}:{PAYLOAD}"
      timestamp_header        = "X-Timestamp"
      max_signature_age       = 120
    }
  }

  bindings = [
    {
      resource = {
        stream = "/events"
        path   = "/events"
      }
      target = "acmeCo/webhooks/events"
    },
  ]
}

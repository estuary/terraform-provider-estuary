variable "salesforce_consumer_key" {
  type      = string
  sensitive = true
}

variable "salesforce_consumer_secret" {
  type      = string
  sensitive = true
}

variable "salesforce_client_id" {
  type      = string
  sensitive = true
}

variable "salesforce_client_secret" {
  type      = string
  sensitive = true
}

variable "salesforce_refresh_token" {
  type      = string
  sensitive = true
}

# Client credentials of an external client app, with auto discovery adding every new object.
resource "estuary_source_salesforce" "crm" {
  name = "crm"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    my_domain  = "acme.my.salesforce.com"
    start_date = "2024-01-01T00:00:00Z"

    credentials = {
      credentials_title = "Client Credentials"
      client_id         = var.salesforce_consumer_key
      client_secret     = var.salesforce_consumer_secret
    }
  }

  bindings = [
    {
      resource = {
        name     = "Account"
        interval = "PT5M"
        schedule = "55 23 * * *"
      }
    },
    {
      resource = {
        name     = "Opportunity"
        interval = "PT5M"
      }
      target = "acmeCo/crm/opportunities"
    },
  ]
}

# A sandbox with the OAuth credentials of your own connected app and a smaller incremental query window.
resource "estuary_source_salesforce" "sandbox" {
  name = "crm/sandbox"

  config = {
    is_sandbox = true

    credentials = {
      credentials_title = "OAuth Credentials"
      client_id         = var.salesforce_client_id
      client_secret     = var.salesforce_client_secret
      refresh_token     = var.salesforce_refresh_token
      instance_url      = "https://acme--uat.sandbox.my.salesforce.com"
    }

    advanced = {
      window_size = {
        window_type = "days"
        days        = 30
      }
    }
  }

  bindings = [
    {
      resource = {
        name     = "Contact"
        interval = "PT15M"
      }
    },
  ]
}

# A capture created through the dashboard login, imported with its secrets left out.
resource "estuary_source_salesforce" "imported" {
  name = "crm/imported"

  config = {
    credentials = {
      credentials_title = "OAuth Credentials"
      instance_url      = "https://acme.my.salesforce.com"
    }
  }

  bindings = []
}

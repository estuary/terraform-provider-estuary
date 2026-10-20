terraform {
  required_version = ">= 1.11"

  required_providers {
    estuary = {
      source  = "estuary/estuary"
      version = "~> 0.1"
    }
  }
}

# The token is read from the FLOW_AUTH_TOKEN environment variable when unset. Check the project README for more details
provider "estuary" {
  tenant = "acmeCo/"
}

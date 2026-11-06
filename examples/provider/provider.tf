terraform {
  required_version = ">= 1.11"

  required_providers {
    estuary = {
      source  = "estuary/estuary"
      version = "~> 0.1"
    }
  }
}

# Provider for the acmeCo/ tenant, authenticated by FLOW_AUTH_TOKEN.
provider "estuary" {
  tenant = "acmeCo/"
}

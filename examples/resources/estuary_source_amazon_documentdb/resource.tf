variable "documentdb_password" {
  type      = string
  sensitive = true
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

# Batch capture of two collections through an SSH bastion host.
resource "estuary_source_amazon_documentdb" "shop" {
  name = "documentdb"

  config = {
    address       = "mongodb://acme.cluster-abc123.us-east-1.docdb.amazonaws.com:27017"
    user          = "flow_capture"
    password      = var.documentdb_password
    database      = "shop"
    poll_schedule = "6h"

    network_tunnel = {
      ssh_forwarding = {
        ssh_endpoint = "ssh://ec2-user@ec2-198-51-100-7.compute-1.amazonaws.com:22"
        private_key  = var.ssh_private_key
      }
    }
  }

  bindings = [
    {
      resource = {
        database      = "shop"
        collection    = "orders"
        capture_mode  = "Batch Incremental"
        cursor_field  = "updatedAt"
        poll_schedule = "15m"
      }
    },
    {
      resource = {
        database     = "shop"
        collection   = "countries"
        capture_mode = "Batch Snapshot"
        cursor_field = "_id"
      }
    },
  ]
}

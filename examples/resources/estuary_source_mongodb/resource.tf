variable "mongodb_password" {
  type      = string
  sensitive = true
}

variable "ssh_private_key" {
  type      = string
  sensitive = true
}

# Change stream capture of two collections, adding every new collection of the
# database as it's discovered.
resource "estuary_source_mongodb" "shop" {
  name = "mongodb"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    address  = "mongodb+srv://cluster0.abc123.mongodb.net?authSource=admin"
    user     = "flow_capture"
    password = var.mongodb_password
    database = "shop"
  }

  bindings = [
    {
      resource = {
        database   = "shop"
        collection = "users"
      }
    },
    {
      resource = {
        database   = "shop"
        collection = "orders"
      }
      target = "acmeCo/shop/orders"
    },
  ]
}

# Batch capture of a view and a time series collection, alongside a change
# stream collection.
resource "estuary_source_mongodb" "metrics" {
  name = "mongodb/metrics"

  config = {
    address                 = "mongodb://mongo.internal.acme.co:27017?authSource=admin"
    user                    = "flow_capture"
    password                = var.mongodb_password
    database                = "shop,metrics"
    batch_and_change_stream = true
    poll_schedule           = "6h"
  }

  bindings = [
    {
      resource = {
        database      = "shop"
        collection    = "user_view"
        capture_mode  = "Batch Snapshot"
        cursor_field  = "_id"
        poll_schedule = "1h"
      }
    },
    {
      resource = {
        database      = "metrics"
        collection    = "readings"
        capture_mode  = "Batch Incremental"
        cursor_field  = "ts"
        poll_schedule = "daily at 02:00Z"
      }
    },
    {
      resource = {
        database     = "shop"
        collection   = "events"
        capture_mode = "Change Stream Incremental"
      }
    },
  ]
}

# Private deployment reached through an SSH bastion host, with advanced change
# stream options.
resource "estuary_source_mongodb" "private" {
  name = "mongodb/private"

  config = {
    address  = "mongodb://10.0.1.25:27017?authSource=admin"
    user     = "flow_capture"
    password = var.mongodb_password

    network_tunnel = {
      ssh_forwarding = {
        ssh_endpoint = "ssh://ec2-user@ec2-198-21-98-1.compute-1.amazonaws.com:22"
        private_key  = var.ssh_private_key
      }
    }

    advanced = {
      max_await_time      = "10s"
      disable_pre_images  = true
      exclude_collections = "shop:sessions,shop:carts"
      skip_backfills      = "shop:events"
    }
  }

  bindings = [
    {
      resource = {
        database   = "shop"
        collection = "events"
      }
    },
  ]
}

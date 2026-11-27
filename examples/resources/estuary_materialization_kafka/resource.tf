variable "kafka_password" {
  type      = string
  sensitive = true
}

variable "schema_registry_secret" {
  type      = string
  sensitive = true
}

# Avro messages, with their schemas in a Confluent schema registry.
resource "estuary_materialization_kafka" "events" {
  name = "events"

  config = {
    bootstrap_servers = "broker-1.example.com:9092,broker-2.example.com:9092"
    tls               = "system_certificates"
    message_format    = "Avro"
    compression       = "zstd"

    credentials = {
      auth_type = "user_password"
      mechanism = "SCRAM-SHA-512"
      username  = "flow_materialize"
      password  = var.kafka_password
    }

    schema_registry = {
      endpoint = "https://psrc-12345.us-east-2.aws.confluent.cloud"
      username = "ABCDEFGHIJKLMNOP"
      password = var.schema_registry_secret
    }
  }

  bindings = [
    {
      source   = "acmeCo/mongodb/shop/orders"
      resource = { topic = "orders" }
    },
    {
      source   = "acmeCo/mongodb/shop/users"
      resource = { topic = "users" }

      fields = {
        recommended = "1"
        exclude     = ["internal_notes"]
      }
    },
  ]
}

# JSON messages following a capture, with the partitions and replication of the topics it creates.
resource "estuary_materialization_kafka" "stream" {
  name = "events/stream"

  source = {
    capture = "acmeCo/aurora/source-amazon-aurora-postgres"
  }

  config = {
    bootstrap_servers        = "broker-1.example.com:9092"
    message_format           = "JSON"
    topic_partitions         = 12
    topic_replication_factor = 2

    credentials = {
      auth_type = "user_password"
      mechanism = "PLAIN"
      username  = "flow_materialize"
      password  = var.kafka_password
    }
  }

  bindings = [
    {
      source   = "acmeCo/aurora/public/orders"
      resource = { topic = "orders" }
    },
  ]
}

variable "kafka_password" {
  type      = string
  sensitive = true
}

variable "schema_registry_secret" {
  type      = string
  sensitive = true
}

variable "aws_secret_access_key" {
  type      = string
  sensitive = true
}

# A SASL user and a Confluent schema registry, with auto discovery adding every new topic.
resource "estuary_source_kafka" "events" {
  name = "events"

  auto_discover = {
    add_new_bindings                = true
    evolve_incompatible_collections = true
  }

  config = {
    bootstrap_servers = "broker-1.example.com:9092,broker-2.example.com:9092"
    tls               = "system_certificates"

    credentials = {
      auth_type = "user_password"
      mechanism = "SCRAM-SHA-512"
      username  = "flow_capture"
      password  = var.kafka_password
    }

    schema_registry = {
      schema_registry_type = "confluent_schema_registry"
      endpoint             = "https://psrc-12345.us-east-2.aws.confluent.cloud"
      username             = "ABCDEFGHIJKLMNOP"
      password             = var.schema_registry_secret
    }
  }

  bindings = [
    {
      resource = { topic = "orders" }
    },
    {
      resource = { topic = "payments" }
      target   = "acmeCo/events/kafka-payments"
    },
  ]
}

# An Amazon MSK cluster through IAM, with JSON messages and no schema registry.
resource "estuary_source_kafka" "msk" {
  name = "events/msk"

  config = {
    bootstrap_servers = "b-1.acme.abc123.c2.kafka.us-east-1.amazonaws.com:9098"

    credentials = {
      auth_type             = "AWS"
      aws_access_key_id     = "AKIAIOSFODNN7EXAMPLE"
      aws_secret_access_key = var.aws_secret_access_key
      region                = "us-east-1"
    }

    schema_registry = {
      schema_registry_type = "no_schema_registry"
      enable_json_only     = true
    }
  }

  bindings = [
    {
      resource = { topic = "clicks" }
    },
  ]
}

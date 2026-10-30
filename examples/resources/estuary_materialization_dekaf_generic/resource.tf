variable "dekaf_token" {
  type      = string
  sensitive = true
}

# Any Kafka consumer, following a capture so each collection it adds is exposed as a topic, skipping old documents on one.
resource "estuary_materialization_dekaf_generic" "events" {
  name = "events"

  source = {
    capture = "acmeCo/aurora/source-amazon-aurora-postgres"
  }

  config = {
    token              = var.dekaf_token
    strict_topic_names = true
    deletions          = "kafka"
  }

  bindings = [
    {
      source     = "acmeCo/aurora/public/events"
      not_before = "2026-01-01T00:00:00Z"
      resource   = { topic_name = "events" }
    },
  ]
}

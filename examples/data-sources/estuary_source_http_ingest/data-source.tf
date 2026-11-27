# Live bindings of a capture managed outside this configuration.
data "estuary_source_http_ingest" "source" {
  name = "webhooks"
}

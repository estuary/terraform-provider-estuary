# Live bindings of a capture managed outside this configuration.
data "estuary_source_postgres" "source" {
  name = "postgres"
}

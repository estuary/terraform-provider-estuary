# Live bindings of a capture managed outside this configuration.
data "estuary_source_aurora_postgres" "source" {
  name = "aurora"
}

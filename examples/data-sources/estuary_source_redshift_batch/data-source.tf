# Live bindings of a capture managed outside this configuration.
data "estuary_source_redshift_batch" "source" {
  name = "warehouse"
}

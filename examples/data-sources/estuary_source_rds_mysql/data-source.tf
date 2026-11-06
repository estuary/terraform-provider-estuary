# Live bindings of a capture managed outside this configuration.
data "estuary_source_rds_mysql" "source" {
  name = "rds"
}

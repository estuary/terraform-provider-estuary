# Live bindings of a capture managed outside this configuration.
data "estuary_source_dynamodb" "source" {
  name = "dynamodb"
}

package flowctl

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/utils"
)

func TestParseConnectorSchema(test *testing.T) {
	var output []byte = []byte(`[{"endpoint_spec_schema":{"properties":{"region":{"type":"string"}}},"resource_spec_schema":{"properties":{"table":{"type":"string"}}},"connectors":{"image_name":"ghcr.io/estuary/source-dynamodb"}}]`)

	schema, err := parseConnectorSchema(output, "ghcr.io/estuary/source-dynamodb:v1")

	if err != nil {
		test.Fatalf("parseConnectorSchema: %v", err)
	}

	if utils.Lookup(schema.Endpoint, "properties", "region") == nil || utils.Lookup(schema.Resource, "properties", "table") == nil {
		test.Errorf("schema = %+v, want the region endpoint and table resource properties", schema)
	}
}

func TestParseConnectorSchemaRequiresOneTag(test *testing.T) {
	if _, err := parseConnectorSchema([]byte("[]"), "ghcr.io/estuary/source-dynamodb:v9"); err == nil {
		test.Error("error = nil, want an unpublished tag to be rejected")
	}
}

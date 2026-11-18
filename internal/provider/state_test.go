package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestUpgradeResourceStateReadsListBindings(test *testing.T) {
	var ctx context.Context = context.Background()
	var typeName string = "estuary_source_postgres"
	var stateValue tftypes.Value
	var attributes map[string]tftypes.Value
	var bindings []tftypes.Value
	var rawState []byte = []byte(`{
		"name": "acmeCo/postgres/source-postgres",
		"catalog_name": "acmeCo/postgres/source-postgres",
		"keep_unmanaged_bindings": true,
		"bindings": [
			{
				"name": "acmeCo/postgres/users",
				"target": "acmeCo/postgres/users",
				"resource": {"namespace": "public", "stream": "users", "primary_key": ["id"]},
				"resource_path": ["public", "users"]
			},
			{
				"name": "acmeCo/postgres/orders",
				"target": "acmeCo/postgres/orders",
				"resource": {"namespace": "public", "stream": "orders"},
				"resource_path": ["public", "orders"]
			}
		]
	}`)

	server, err := providerserver.NewProtocol6WithError(New("test")())()

	if err != nil {
		test.Fatal(err)
	}

	schemaResp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})

	if err != nil || schemaResp.ResourceSchemas[typeName] == nil {
		test.Fatalf("GetProviderSchema = %v (%v), want the %s schema", schemaResp, err, typeName)
	}

	var stateType tftypes.Type = schemaResp.ResourceSchemas[typeName].ValueType()

	upgradeResp, err := server.UpgradeResourceState(ctx, &tfprotov6.UpgradeResourceStateRequest{
		TypeName: typeName,
		Version:  schemaResp.ResourceSchemas[typeName].Version,
		RawState: &tfprotov6.RawState{JSON: rawState},
	})

	if err != nil || len(upgradeResp.Diagnostics) > 0 || upgradeResp.UpgradedState == nil {
		test.Fatalf("UpgradeResourceState = %v (%v), want the state without diagnostics", upgradeResp, err)
	}

	stateValue, err = upgradeResp.UpgradedState.Unmarshal(stateType)

	if err == nil {
		err = stateValue.As(&attributes)
	}

	if err == nil {
		err = attributes["bindings"].As(&bindings)
	}

	if err != nil {
		test.Fatal(err)
	}

	if _, isSet := attributes["bindings"].Type().(tftypes.Set); !isSet || len(bindings) != 2 {
		test.Fatalf("bindings = %v, want a set of both stored bindings", attributes["bindings"])
	}

	for _, want := range []string{"users", "orders"} {
		var found bool

		for _, binding := range bindings {
			var bindingAttributes, resourceAttributes map[string]tftypes.Value
			var stream string

			if binding.As(&bindingAttributes) != nil || bindingAttributes["resource"].As(&resourceAttributes) != nil || resourceAttributes["stream"].As(&stream) != nil {
				continue
			}

			found = found || (stream == want && bindingAttributes["target"].Equal(tftypes.NewValue(tftypes.String, "acmeCo/postgres/"+want)))
		}

		if !found {
			test.Errorf("bindings = %v, want the stored %s binding with its target", bindings, want)
		}
	}
}

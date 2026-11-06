package provider

import (
	"context"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// configure runs Configure on a provider configuration of the given attribute values, null when left out.
func configure(test *testing.T, values map[string]tftypes.Value) provider.ConfigureResponse {
	var ctx context.Context = context.Background()
	var schemaResp provider.SchemaResponse
	var configureResp provider.ConfigureResponse
	var estuaryProvider *EstuaryProvider = &EstuaryProvider{}

	estuaryProvider.Schema(ctx, provider.SchemaRequest{}, &schemaResp)

	var configType tftypes.Object = schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	var attributes map[string]tftypes.Value = map[string]tftypes.Value{}

	for attribute, attributeType := range configType.AttributeTypes {
		attributes[attribute] = tftypes.NewValue(attributeType, nil)
	}

	for attribute, value := range values {
		attributes[attribute] = value
	}

	estuaryProvider.Configure(ctx, provider.ConfigureRequest{
		Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(configType, attributes)},
	}, &configureResp)

	return configureResp
}

func TestConfigureRejectsUnknownSettings(test *testing.T) {
	test.Setenv("FLOW_AUTH_TOKEN", "ambient-token")
	test.Setenv("ESTUARY_TENANT", "ambientCo/")

	for _, attribute := range []string{"backend", "token", "tenant", "flowctl_path", "data_plane"} {
		configureResp := configure(test, map[string]tftypes.Value{attribute: tftypes.NewValue(tftypes.String, tftypes.UnknownValue)})

		if !configureResp.Diagnostics.HasError() || configureResp.ResourceData != nil {
			test.Errorf("%s: diagnostics = %v, session = %v, want an error and no session falling back to the environment",
				attribute, configureResp.Diagnostics, configureResp.ResourceData)
		}
	}
}

func TestConfigurePrefersSettingsOverTheEnvironment(test *testing.T) {
	test.Setenv("FLOW_AUTH_TOKEN", "ambient-token")
	test.Setenv("ESTUARY_TENANT", "ambientCo/")

	expectations := map[string]struct {
		values map[string]tftypes.Value
		want   string
	}{
		"configured tenant":  {map[string]tftypes.Value{"tenant": tftypes.NewValue(tftypes.String, "acmeCo")}, "acmeCo/"},
		"environment tenant": {map[string]tftypes.Value{}, "ambientCo/"},
		"configured token":   {map[string]tftypes.Value{"token": tftypes.NewValue(tftypes.String, "configured-token")}, "ambientCo/"},
	}

	for name, expected := range expectations {
		configureResp := configure(test, expected.values)

		session, _ := configureResp.ResourceData.(*catalog.Session)

		if configureResp.Diagnostics.HasError() || session == nil || session.Tenant != expected.want {
			test.Errorf("%s: session = %v (%v), want tenant %s", name, session, configureResp.Diagnostics, expected.want)
		}
	}
}

package builder

import (
	"context"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/snowflake"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/webhook"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
	"github.com/estuary/terraform-provider-estuary/internal/resources/builder/translator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var snowflakeConnector Connector = Connector{
	TypeName:      "materialization_snowflake",
	Image:         snowflake.Image,
	ConnectorName: snowflake.ConnectorName,
}

var webhookConnector Connector = Connector{
	TypeName:      "materialization_webhook",
	Image:         webhook.Image,
	ConnectorName: webhook.ConnectorName,
}

var webhookHeadersPath *tftypes.AttributePath = tftypes.NewAttributePath().WithAttributeName("config").WithAttributeName("headers").WithAttributeName("custom_headers")

func TestSnowflakeSamplesRoundTrip(test *testing.T) {
	assertMaterializationSamplesRoundTrip[snowflake.Config, snowflake.Resource](test, snowflakeConnector, "../../catalog/materialization/snowflake/testdata")
}

func TestWebhookSamplesRoundTrip(test *testing.T) {
	assertMaterializationSamplesRoundTrip[webhook.Config, webhook.Resource](test, webhookConnector, "../../catalog/materialization/webhook/testdata")
}

func TestWebhookHeaderValuesAreWriteOnly(test *testing.T) {
	var spec MaterializationSpec[webhook.Config, webhook.Resource] = spectest.DecodeMaterializations[MaterializationSpec[webhook.Config, webhook.Resource]](test, "../../catalog/materialization/webhook/testdata/full.yaml")["acmeCo/analytics/materialize-webhook"]

	codec, err := newCodec[materializationDocument[webhook.Config, webhook.Resource]](webhookConnector, materializationType[webhook.Config, webhook.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	config, err := codec.Value("analytics", spec)

	if err != nil {
		test.Fatal(err)
	}

	plan, err := codec.secrets.nullSecrets(config)

	if err != nil {
		test.Fatal(err)
	}

	for index := range spec.Endpoint.Connector.Config.Headers.CustomHeaders {
		var header *tftypes.AttributePath = webhookHeadersPath.WithElementKeyInt(index)

		if value, _ := attributeValue(plan, header.WithAttributeName("value")); !value.IsNull() {
			test.Errorf("header %d value = %v, want null in the plan", index, value)
		}

		if name, _ := attributeValue(plan, header.WithAttributeName("name")); name.IsNull() {
			test.Errorf("header %d name is null, want it kept in the plan", index)
		}

		if _, isHash := codec.secrets.secretPath(header.WithAttributeName("value_hash")); !isHash {
			test.Errorf("header %d value_hash isn't the hash of a write-only value", index)
		}
	}

	if merged, err := codec.secrets.withConfigSecrets(plan, config); err != nil || !merged.Equal(config) {
		test.Errorf("plan with config secrets = %v, %v, want the config", merged, err)
	}

	if values := codec.secrets.values(config); len(values) != 2 {
		test.Errorf("masked secrets = %d values, want the 2 header values", len(values))
	}

	state, err := codec.secrets.stateHashes(config, config)

	if err != nil {
		test.Fatal(err)
	}

	_, changed, err := codec.secrets.planHashes(config, config, state)

	if err != nil || len(changed) != 0 {
		test.Errorf("unchanged headers: changed = %v, %v, want none", changed, err)
	}

	spec.Endpoint.Connector.Config.Headers.CustomHeaders[1].Value = "rotated"

	rotated, err := codec.Value("analytics", spec)

	if err != nil {
		test.Fatal(err)
	}

	_, changed, err = codec.secrets.planHashes(rotated, rotated, state)

	if err != nil || len(changed) != 1 || !changed[0].Equal(webhookHeadersPath.WithElementKeyInt(1).WithAttributeName("value")) {
		test.Errorf("rotated header: changed = %v, %v, want the second header value", changed, err)
	}
}

func TestWebhookURLsForbidCredentials(test *testing.T) {
	codec, err := newCodec[materializationDocument[webhook.Config, webhook.Resource]](webhookConnector, materializationType[webhook.Config, webhook.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	var paths []*tftypes.AttributePath = []*tftypes.AttributePath{
		tftypes.NewAttributePath().WithAttributeName("config").WithAttributeName("address"),
		tftypes.NewAttributePath().WithAttributeName("bindings").WithElementKeyInt(0).WithAttributeName("resource").WithAttributeName("relative_path"),
	}

	for _, path := range paths {
		attribute, err := codec.secrets.schema.AttributeAtTerraformPath(context.Background(), path)

		if err != nil {
			test.Fatalf("%s: %v", path, err)
		}

		if translator.AttributeCredentials(attribute.(schema.Attribute)) != catalog.CredentialsForbidden {
			test.Errorf("%s doesn't reject credentials", path)
		}
	}

	expectations := map[string]string{
		"https://user:secret@hooks.example.com/":     "https://***@hooks.example.com/",
		"https://hooks.example.com/?access_token=t1": "https://hooks.example.com/?access_token=***",
		"https://hooks.example.com/estuary/":         "https://hooks.example.com/estuary/",
		"orders?api_key=k1&page=2":                   "orders?api_key=***&page=2",
	}

	for value, want := range expectations {
		if got := translator.RedactCredentials(value, catalog.CredentialsForbidden); got != want {
			test.Errorf("RedactCredentials(%q) = %q, want %q", value, got, want)
		}
	}
}

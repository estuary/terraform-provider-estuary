package builder

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/databricks"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var databricksConnector Connector = Connector{
	TypeName:      "materialization_databricks",
	Image:         databricks.Image,
	ConnectorName: databricks.ConnectorName,
}

// databricksLive is a Databricks materialization as the dashboard publishes it on the default image: sealed secrets,
// explicit endpoint defaults, the bindings of a source capture with their resource paths, and a runtime shard flag.
const databricksLive string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/materialize-databricks:v3
			config:
				address: dbc-abcdef12-3456.cloud.databricks.com
				http_path: /sql/1.0/warehouses/abcdef1234567890
				catalog_name: main
				schema_name: default
				hardDelete: false
				credentials:
					auth_type: OAuth2 M2M
					client_id_sops: ENC[AES256_GCM,data:abc,type:str]
					client_secret_sops: ENC[AES256_GCM,data:def,type:str]
				syncSchedule:
					syncFrequency: 30m
				advanced:
					no_flow_document: false
				sops:
					gcp_kms:
						- resource_id: projects/example/locations/us-central1/keyRings/example/cryptoKeys/example
							created_at: "2026-01-01T00:00:00Z"
							enc: <encrypted>
					lastmodified: "2026-01-01T00:00:00Z"
					mac: ENC[AES256_GCM,data:jkl,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	source:
		capture: acmeCo/shop/source-postgres
	bindings:
		- resource:
				table: orders
				schema: default
				_meta:
					path: [main, default, orders]
			source: acmeCo/shop/orders
			fields:
				recommended: true
		- resource:
				table: events
				schema: raw
				delta_updates: true
				additional_table_create_sql: ALTER TABLE raw.events SET TBLPROPERTIES ('delta.appendOnly' = 'true')
				_meta:
					path: [main, raw, events]
			source: acmeCo/shop/events
			fields:
				recommended: true
			backfill: 2
	shards:
		flags:
			enable-runtime-v2: "true"
	expectPubId: "11:22:33:44:55:66:77:88"
`

// databricksLiveV2 is a Databricks materialization on the v2 image, whose schema has no OAuth2 M2M credentials and no
// additional_table_create_sql.
const databricksLiveV2 string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/materialize-databricks:v2
			config:
				address: dbc-abcdef12-3456.cloud.databricks.com
				http_path: /sql/1.0/warehouses/abcdef1234567890
				catalog_name: main
				schema_name: default
				hardDelete: false
				credentials:
					auth_type: PAT
					personal_access_token_sops: ENC[AES256_GCM,data:abc,type:str]
				syncSchedule:
					syncFrequency: 30m
				sops:
					mac: ENC[AES256_GCM,data:jkl,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				table: orders
				schema: default
				_meta:
					path: [main, default, orders]
			source: acmeCo/shop/orders
			fields:
				recommended: true
`

// databricksLiveV2UpdateDelay is a v2 materialization setting advanced.updateDelay, which the v3 schema dropped.
const databricksLiveV2UpdateDelay string = `
	endpoint:
		connector:
			image: ghcr.io/estuary/materialize-databricks:v2
			config:
				address: dbc-abcdef12-3456.cloud.databricks.com
				http_path: /sql/1.0/warehouses/abcdef1234567890
				catalog_name: main
				schema_name: default
				credentials:
					auth_type: PAT
					personal_access_token_sops: ENC[AES256_GCM,data:abc,type:str]
				advanced:
					updateDelay: 1h
				sops:
					mac: ENC[AES256_GCM,data:jkl,type:str]
					encrypted_suffix: _sops
					version: 3.9.1
	bindings:
		- resource:
				table: orders
				_meta:
					path: [main, default, orders]
			source: acmeCo/shop/orders
`

func databricksSpec(image string, config databricks.Config, bindings ...materialization.Binding[databricks.Resource]) MaterializationSpec[databricks.Config, databricks.Resource] {
	return MaterializationSpec[databricks.Config, databricks.Resource]{
		Endpoint: endpoint.ConnectorSpec[databricks.Config]{Connector: endpoint.Connector[databricks.Config]{Image: image, Config: config}},
		Bindings: bindings,
	}
}

func TestDatabricksSamplesRoundTrip(test *testing.T) {
	assertMaterializationSamplesRoundTrip[databricks.Config, databricks.Resource](test, databricksConnector, "../../catalog/materialization/databricks/testdata")
}

func TestDatabricksImportPlansNoChanges(test *testing.T) {
	var sourceCapture string = "acmeCo/shop/source-postgres"
	var config databricks.Config = databricks.Config{
		Address:      "dbc-abcdef12-3456.cloud.databricks.com",
		HTTPPath:     "/sql/1.0/warehouses/abcdef1234567890",
		CatalogName:  "main",
		SchemaName:   "default",
		Credentials:  databricks.Credentials{AuthType: databricks.AuthTypeOAuth2M2M},
		SyncSchedule: &materialization.SyncSchedule{SyncFrequency: "30m"},
		Advanced:     &databricks.Advanced{},
	}

	entry := newConnectorMaterialization[databricks.Config, databricks.Resource](test, databricksConnector, databricksLive)

	var state tftypes.Value = entry.importState(test, "acmeCo/warehouse/materialize-databricks", false)
	var undeclared MaterializationSpec[databricks.Config, databricks.Resource] = databricksSpec(databricks.Image, config)

	undeclared.Source = &materialization.Source{Capture: &sourceCapture}

	entry.assertPlansNoChanges(test, "acmeCo/warehouse", undeclared, state)

	var declared MaterializationSpec[databricks.Config, databricks.Resource] = databricksSpec(databricks.Image, config,
		materialization.Binding[databricks.Resource]{
			Resource: databricks.Resource{Table: "orders"},
			Source:   materialization.BindingSource{Name: "acmeCo/shop/orders"},
		},
		materialization.Binding[databricks.Resource]{
			Resource: databricks.Resource{Table: "events", Schema: "raw", DeltaUpdates: true, AdditionalTableCreateSQL: "ALTER TABLE raw.events SET TBLPROPERTIES ('delta.appendOnly' = 'true')"},
			Source:   materialization.BindingSource{Name: "acmeCo/shop/events"},
		},
	)

	declared.Source = &materialization.Source{Capture: &sourceCapture}

	entry.assertAppliesWithoutPublishing(test, "acmeCo/warehouse", declared, state)
}

func TestDatabricksImportOfTheV2ImagePlansNoChanges(test *testing.T) {
	const imageV2 string = "ghcr.io/estuary/materialize-databricks:v2"

	var config databricks.Config = databricks.Config{
		Address:      "dbc-abcdef12-3456.cloud.databricks.com",
		HTTPPath:     "/sql/1.0/warehouses/abcdef1234567890",
		CatalogName:  "main",
		SchemaName:   "default",
		Credentials:  databricks.Credentials{AuthType: databricks.AuthTypePAT},
		SyncSchedule: &materialization.SyncSchedule{SyncFrequency: "30m"},
	}

	entry := newConnectorMaterialization[databricks.Config, databricks.Resource](test, databricksConnector, databricksLiveV2)

	var state tftypes.Value = entry.importState(test, "acmeCo/warehouse/materialize-databricks", false)

	entry.assertPlansNoChanges(test, "acmeCo/warehouse", databricksSpec(imageV2, config), state)

	entry.assertAppliesWithoutPublishing(test, "acmeCo/warehouse", databricksSpec(imageV2, config, materialization.Binding[databricks.Resource]{
		Resource: databricks.Resource{Table: "orders"},
		Source:   materialization.BindingSource{Name: "acmeCo/shop/orders"},
	}), state)

	// The import warns about updateDelay, which stays live while the endpoint isn't published.
	var delayedConfig databricks.Config = config

	delayedConfig.SyncSchedule = nil
	delayedConfig.Advanced = &databricks.Advanced{}

	delayed := newConnectorMaterialization[databricks.Config, databricks.Resource](test, databricksConnector, databricksLiveV2UpdateDelay)

	state = delayed.importState(test, "acmeCo/warehouse/materialize-databricks", true)

	delayed.assertPlansNoChanges(test, "acmeCo/warehouse", databricksSpec(imageV2, delayedConfig), state)
	delayed.assertAppliesWithoutPublishing(test, "acmeCo/warehouse", databricksSpec(imageV2, delayedConfig), state)
}

func TestDatabricksSecretsAreWriteOnly(test *testing.T) {
	var spec MaterializationSpec[databricks.Config, databricks.Resource] = databricksSpec(databricks.Image, databricks.Config{
		Address:       "dbc-abcdef12-3456.cloud.databricks.com",
		HTTPPath:      "/sql/1.0/warehouses/abcdef1234567890",
		CatalogName:   "main",
		SchemaName:    "default",
		Credentials:   databricks.Credentials{PersonalAccessToken: "<personal-access-token>", ClientID: "<client-id>", ClientSecret: "<client-secret>"},
		DBTJobTrigger: &materialization.DBTJobTrigger{JobID: "12345", AccountID: "67890", APIKey: "<dbt-api-key>"},
	})

	codec, err := newCodec[materializationDocument[databricks.Config, databricks.Resource]](databricksConnector, materializationType[databricks.Config, databricks.Resource]())

	if err != nil {
		test.Fatal(err)
	}

	config, err := codec.Value("warehouse", spec)

	if err != nil {
		test.Fatal(err)
	}

	assertWriteOnly(test, codec.secrets, config,
		configAttributePath("credentials", "personal_access_token"),
		configAttributePath("credentials", "client_id"),
		configAttributePath("credentials", "client_secret"),
		configAttributePath("dbt_job_trigger", "api_key"),
	)
}

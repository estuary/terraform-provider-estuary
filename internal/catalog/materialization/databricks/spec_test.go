package databricks

import (
	"regexp"
	"slices"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
)

func TestSpec(test *testing.T) {
	spectest.RunMaterializationSuite[Spec](test, spectest.Connector{Image: Image, Name: ConnectorName})
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test)
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, AuthTypes, materialization.DBTJobTriggerModes, materialization.OnIncompatibleSchemaChanges, materialization.TargetNamingStrategies)
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		authType AuthType
		bindings []Resource
	}{
		"acmeCo/databricks/personal-access-token": {AuthTypePAT, []Resource{{Table: "orders"}}},
		"acmeCo/databricks/service-principal":     {AuthTypeOAuth2M2M, []Resource{{Table: "orders"}}},
		"acmeCo/databricks/delta-updates":         {AuthTypePAT, []Resource{{Table: "events", Schema: "raw", DeltaUpdates: true}, {Table: "orders", Schema: "raw"}}},
	}

	if len(specs) != len(expectations) {
		test.Fatalf("got %d materializations, want %d", len(specs), len(expectations))
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			if !found {
				test.Fatal("materialization missing from sample")
			}

			if authType := spec.Endpoint.Connector.Config.Credentials.AuthType; authType != expected.authType {
				test.Errorf("auth_type = %q, want %q", authType, expected.authType)
			}

			spectest.AssertMaterializationResources(test, spec.Bindings, expected.bindings)
		})
	}
}

func TestSampleFullSpec(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var config Config = specs["acmeCo/analytics/materialize-databricks"].Endpoint.Connector.Config

	expectedSyncSchedule := materialization.SyncSchedule{
		SyncFrequency:       "1h",
		Timezone:            "America/New_York",
		FastSyncStartTime:   "09:00",
		FastSyncStopTime:    "17:00",
		FastSyncEnabledDays: "M-F",
	}

	expectedDBTJobTrigger := materialization.DBTJobTrigger{
		JobID:     "12345",
		AccountID: "67890",
		APIKey:    "<dbt-api-key>",
		AccessURL: "https://cloud.getdbt.com",
		Cause:     "Estuary",
		Mode:      materialization.DBTJobTriggerModeReplace,
		Interval:  "30m",
	}

	expectedAdvanced := Advanced{
		NoFlowDocument: true,
		FeatureFlags:   "allow_existing_tables_for_new_bindings",
	}

	if config.Address != "dbc-abcdef12-3456.cloud.databricks.com:443" || config.HTTPPath != "/sql/1.0/warehouses/abcdef1234567890" {
		test.Errorf("address = %q, http_path = %q", config.Address, config.HTTPPath)
	}

	if config.CatalogName != "acmeCo_catalog" || config.SchemaName != "acmeCo_flow_schema" || !config.HardDelete {
		test.Errorf("catalog_name = %q, schema_name = %q, hardDelete = %v", config.CatalogName, config.SchemaName, config.HardDelete)
	}

	if config.SyncSchedule == nil || *config.SyncSchedule != expectedSyncSchedule {
		test.Errorf("syncSchedule = %+v, want %+v", config.SyncSchedule, expectedSyncSchedule)
	}

	if config.DBTJobTrigger == nil || *config.DBTJobTrigger != expectedDBTJobTrigger {
		test.Errorf("dbt_job_trigger = %+v, want %+v", config.DBTJobTrigger, expectedDBTJobTrigger)
	}

	if config.Advanced == nil || *config.Advanced != expectedAdvanced {
		test.Errorf("advanced = %+v, want %+v", config.Advanced, expectedAdvanced)
	}
}

func TestSampleBindings(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/full.yaml")

	var bindings []Binding = specs["acmeCo/analytics/materialize-databricks"].Bindings

	spectest.AssertMaterializationResources(test, bindings, []Resource{
		{Table: "orders"},
		{Table: "customers", Schema: "crm", AdditionalTableCreateSQL: "ALTER TABLE crm.customers SET TBLPROPERTIES ('delta.enableChangeDataFeed' = 'true')"},
		{Table: "events", DeltaUpdates: true},
	})

	if bindings[2].Fields == nil || !slices.Equal(bindings[2].Fields.Exclude, []string{"payload"}) {
		test.Errorf("events fields = %+v, want exclude [payload]", bindings[2].Fields)
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var address *regexp.Regexp = regexp.MustCompile(`^[^/:@]+(:[0-9]+)?$`)
	var authTypes *spectest.EnumSamples[AuthType] = spectest.NewEnumSamples(test, "auth_type", AuthTypes)
	var deltaUpdates map[bool]bool = map[bool]bool{}

	for _, file := range spectest.SampleFiles(test) {
		var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, file)

		for name, spec := range specs {
			var config Config = spec.Endpoint.Connector.Config

			var credentials Credentials = config.Credentials

			if !address.MatchString(config.Address) {
				test.Errorf("%s: address %q must be host[:port]", name, config.Address)
			}

			authTypes.Add(name, credentials.AuthType)

			if credentials.AuthType == AuthTypePAT && (credentials.PersonalAccessToken == "" || credentials.ClientID != "" || credentials.ClientSecret != "") {
				test.Errorf("%s: PAT credentials need personal_access_token and no client", name)
			}

			if credentials.AuthType == AuthTypeOAuth2M2M && (credentials.ClientID == "" || credentials.ClientSecret == "" || credentials.PersonalAccessToken != "") {
				test.Errorf("%s: OAuth2 M2M credentials need client_id and client_secret and no personal_access_token", name)
			}

			for _, binding := range spec.Bindings {
				deltaUpdates[binding.Resource.DeltaUpdates] = true
			}
		}
	}

	authTypes.AssertAllSampled()

	if !deltaUpdates[true] || !deltaUpdates[false] {
		test.Errorf("delta_updates values sampled = %v, want a delta updates binding and a standard one", deltaUpdates)
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"updateDelay is a v2 advanced option": {
			Key: "updateDelay",
			Content: `
				materializations:
					acmeCo/databricks/update-delay:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-databricks:v2
								config:
									address: dbc-abcdef12-3456.cloud.databricks.com
									http_path: /sql/1.0/warehouses/abcdef1234567890
									catalog_name: acmeCo_catalog
									schema_name: default
									credentials:
										auth_type: PAT
										personal_access_token: <personal-access-token>
									advanced:
										updateDelay: 30m
						bindings: []
			`,
		},
		"clustering is a Snowflake resource field": {
			Key: "clustering",
			Content: `
				materializations:
					acmeCo/databricks/clustering:
						endpoint:
							connector:
								image: ghcr.io/estuary/materialize-databricks:v3
								config:
									address: dbc-abcdef12-3456.cloud.databricks.com
									http_path: /sql/1.0/warehouses/abcdef1234567890
									catalog_name: acmeCo_catalog
									schema_name: default
									credentials:
										auth_type: PAT
										personal_access_token: <personal-access-token>
						bindings:
							- resource:
									table: orders
									clustering: region
								source: acmeCo/orders
			`,
		},
	}

	spectest.AssertUnknownMaterializationKeys[Spec](test, documents)
}

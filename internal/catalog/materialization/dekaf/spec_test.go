package dekaf

import (
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/spectest"
	"gopkg.in/yaml.v3"
)

func TestSpec(test *testing.T) {
	spectest.RunMaterializationSuite[Spec](test)
}

func TestSchemaConformance(test *testing.T) {
	spectest.AssertConformsToSchema[Config, Resource](test)
}

func TestEnumTagsMatchLists(test *testing.T) {
	spectest.AssertEnumTags[Spec](test, Variants, DeletionModes, materialization.OnIncompatibleSchemaChanges, materialization.TargetNamingStrategies)
}

func TestConnectorNames(test *testing.T) {
	for _, variant := range Variants {
		if name := ConnectorName(variant); name != "dekaf-"+string(variant) {
			test.Errorf("connector name of %q = %q, want dekaf-%s", variant, name, variant)
		}
	}
}

func TestSampleVariants(test *testing.T) {
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	expectations := map[string]struct {
		variant          Variant
		strictTopicNames bool
		deletions        DeletionMode
	}{
		"acmeCo/dekaf/generic":        {VariantGeneric, false, DeletionModeKafka},
		"acmeCo/dekaf/generic-strict": {VariantGeneric, true, DeletionModeKafka},
		"acmeCo/dekaf/materialize":    {VariantMaterialize, false, DeletionModeKafka},
		"acmeCo/dekaf/tinybird":       {VariantTinybird, false, DeletionModeCDC},
		"acmeCo/dekaf/clickhouse":     {VariantClickHouse, false, DeletionModeCDC},
		"acmeCo/dekaf/risingwave":     {VariantRisingWave, false, DeletionModeKafka},
		"acmeCo/dekaf/singlestore":    {VariantSingleStore, false, DeletionModeKafka},
		"acmeCo/dekaf/startree":       {VariantStarTree, false, DeletionModeKafka},
		"acmeCo/dekaf/imply-polaris":  {VariantImplyPolaris, false, DeletionModeKafka},
		"acmeCo/dekaf/bytewax":        {VariantBytewax, false, DeletionModeKafka},
	}

	if len(specs) != len(expectations) {
		test.Fatalf("got %d materializations, want %d", len(specs), len(expectations))
	}

	for name, expected := range expectations {
		test.Run(name, func(test *testing.T) {
			spec, found := specs[name]

			var endpoint Endpoint = spec.Endpoint.Dekaf

			if !found {
				test.Fatal("materialization missing from sample")
			}

			if endpoint.Variant != expected.variant {
				test.Errorf("variant = %q, want %q", endpoint.Variant, expected.variant)
			}

			if endpoint.Config.Token != "<auth-token>" {
				test.Errorf("token = %q, want <auth-token>", endpoint.Config.Token)
			}

			if endpoint.Config.StrictTopicNames != expected.strictTopicNames {
				test.Errorf("strict_topic_names = %v, want %v", endpoint.Config.StrictTopicNames, expected.strictTopicNames)
			}

			if endpoint.Config.Deletions != expected.deletions {
				test.Errorf("deletions = %q, want %q", endpoint.Config.Deletions, expected.deletions)
			}

			if len(spec.Bindings) != 1 || spec.Bindings[0].Resource.TopicName != "orders" || spec.Bindings[0].Source.Name != "acmeCo/orders" {
				test.Errorf("bindings = %+v, want a single orders binding from acmeCo/orders", spec.Bindings)
			}
		})
	}
}

func TestSamplesFollowConnectorRules(test *testing.T) {
	var variants *spectest.EnumSamples[Variant] = spectest.NewEnumSamples(test, "variant", Variants)
	var deletionModes *spectest.EnumSamples[DeletionMode] = spectest.NewEnumSamples(test, "deletions", DeletionModes)
	var specs map[string]Spec = spectest.DecodeMaterializations[Spec](test, "testdata/variants.yaml")

	for name, spec := range specs {
		var endpoint Endpoint = spec.Endpoint.Dekaf

		variants.Add(name, endpoint.Variant)

		deletionModes.Add(name, endpoint.Config.Deletions)
	}

	variants.AssertAllSampled()
}

func TestExplicitDefaultsAreEmitted(test *testing.T) {
	spec := Spec{
		Endpoint: EndpointSpec{
			Dekaf: Endpoint{
				Variant: VariantGeneric,
				Config: Config{
					Token: "token",
				},
			},
		},
		Bindings: []Binding{{Resource: Resource{TopicName: "orders"}, Source: materialization.BindingSource{Name: "acmeCo/orders"}}},
	}

	content, err := yaml.Marshal(spec)

	if err != nil {
		test.Fatalf("marshaling spec: %v", err)
	}

	var marshaled struct {
		Endpoint struct {
			Dekaf struct {
				Config map[string]any `yaml:"config"`
			} `yaml:"dekaf"`
		} `yaml:"endpoint"`
	}

	if err := yaml.Unmarshal(content, &marshaled); err != nil {
		test.Fatalf("decoding marshaled spec: %v", err)
	}

	for _, key := range []string{"strict_topic_names", "deletions"} {
		if _, found := marshaled.Endpoint.Dekaf.Config[key]; !found {
			test.Errorf("config is missing %s:\n%s", key, content)
		}
	}
}

func TestUnknownKeysAreRejected(test *testing.T) {
	documents := map[string]spectest.UnknownKey{
		"misspelled strict_topic_names": {
			Key: "strict_topic_name",
			Content: `
				materializations:
					acmeCo/dekaf/typo:
						endpoint:
							dekaf:
								variant: generic
								config:
									token: <auth-token>
									strict_topic_name: true
						bindings: []
			`,
		},
	}

	spectest.AssertUnknownMaterializationKeys[Spec](test, documents)
}

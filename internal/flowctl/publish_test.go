package flowctl

import (
	"strings"
	"testing"

	"github.com/estuary/terraform-provider-estuary/internal/catalog/capture"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/common/endpoint"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization"
	"github.com/estuary/terraform-provider-estuary/internal/catalog/materialization/dekaf"
)

func TestCaptureFilesWrapSpecUnderItsName(test *testing.T) {
	spec := capture.Spec[endpoint.ConnectorSpec[map[string]any], map[string]any]{
		Endpoint: endpoint.ConnectorSpec[map[string]any]{
			Connector: endpoint.Connector[map[string]any]{
				Image:  "ghcr.io/estuary/source-dynamodb:v1",
				Config: map[string]any{"region": "us-east-1"},
			},
		},
	}

	files, err := captureFiles("acmeCo/dynamodb/source", spec)

	if err != nil {
		test.Fatalf("captureFiles: %v", err)
	}

	var content string = string(files[specFile])

	if len(files) != 1 || !strings.Contains(content, "captures:\n    acmeCo/dynamodb/source:\n") || !strings.Contains(content, "region: us-east-1") {
		test.Errorf("files = %v, want a single spec file with the capture under its name", files)
	}
}

func TestMaterializationFilesSplitDekafConfig(test *testing.T) {
	spec := dekaf.Spec{
		Endpoint: dekaf.EndpointSpec{
			Dekaf: dekaf.Endpoint{
				Variant: dekaf.VariantTinybird,
				Config: dekaf.Config{
					Token:     "<auth-token>",
					Deletions: dekaf.DeletionModeKafka,
				},
			},
		},
		Bindings: []dekaf.Binding{
			{
				Resource: dekaf.Resource{TopicName: "orders"},
				Source:   "acmeCo/orders",
			},
		},
	}

	files, err := materializationFiles("acmeCo/dekaf/tinybird", spec)

	if err != nil {
		test.Fatalf("materializationFiles: %v", err)
	}

	var content string = string(files[specFile])

	var config string = string(files[dekafConfigFile])

	if !strings.Contains(content, "config: "+dekafConfigFile) || strings.Contains(content, "<auth-token>") {
		test.Errorf("spec file = %s, want the Dekaf config replaced by %s", content, dekafConfigFile)
	}

	if !strings.Contains(config, "token: <auth-token>") || !strings.Contains(config, "deletions: kafka") {
		test.Errorf("config file = %s, want the Dekaf config", config)
	}
}

func TestMaterializationFilesKeepConnectorConfigInline(test *testing.T) {
	spec := materialization.Spec[endpoint.ConnectorSpec[map[string]any], map[string]any]{
		Endpoint: endpoint.ConnectorSpec[map[string]any]{
			Connector: endpoint.Connector[map[string]any]{
				Image:  "ghcr.io/estuary/materialize-snowflake:v4",
				Config: map[string]any{"host": "orgname-accountname.snowflakecomputing.com"},
			},
		},
	}

	files, err := materializationFiles("acmeCo/snowflake/materialize", spec)

	if err != nil {
		test.Fatalf("materializationFiles: %v", err)
	}

	if len(files) != 1 || !strings.Contains(string(files[specFile]), "host: orgname-accountname.snowflakecomputing.com") {
		test.Errorf("files = %v, want a single spec file with the config inline", files)
	}
}

package provider

import (
	"context"
	"maps"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestResourceAttributeDescriptions(test *testing.T) {
	var ctx context.Context = context.Background()

	for _, newResource := range resources {
		var metadataResp resource.MetadataResponse
		var schemaResp resource.SchemaResponse
		var entry resource.Resource = newResource()

		entry.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "estuary"}, &metadataResp)
		entry.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

		if schemaResp.Diagnostics.HasError() {
			test.Errorf("%s: schema: %v", metadataResp.TypeName, schemaResp.Diagnostics)
			continue
		}

		var pending map[string]schema.Attribute = maps.Clone(schemaResp.Schema.Attributes)

		for len(pending) > 0 {
			for name, attribute := range pending {
				delete(pending, name)

				if attribute.GetDescription() == "" {
					test.Errorf("%s: %s has no description", metadataResp.TypeName, name)
				}

				if attribute.IsWriteOnly() != strings.HasSuffix(attribute.GetDescription(), " Write-only.") {
					test.Errorf("%s: %s is write-only = %t, but its description ending in \" Write-only.\" says %t",
						metadataResp.TypeName, name, attribute.IsWriteOnly(), !attribute.IsWriteOnly())
				}

				switch nested := attribute.(type) {
				case schema.SingleNestedAttribute:
					for child, childAttribute := range nested.Attributes {
						pending[name+"."+child] = childAttribute
					}
				case schema.ListNestedAttribute:
					for child, childAttribute := range nested.NestedObject.Attributes {
						pending[name+"."+child] = childAttribute
					}
				}
			}
		}
	}
}

func TestDataSourceAttributesHaveDescriptions(test *testing.T) {
	var ctx context.Context = context.Background()

	for _, newDataSource := range dataSources {
		var metadataResp datasource.MetadataResponse
		var schemaResp datasource.SchemaResponse
		var source datasource.DataSource = newDataSource()

		source.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "estuary"}, &metadataResp)
		source.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)

		if schemaResp.Diagnostics.HasError() {
			test.Errorf("%s: schema: %v", metadataResp.TypeName, schemaResp.Diagnostics)
			continue
		}

		for name, attribute := range schemaResp.Schema.Attributes {
			if attribute.GetDescription() == "" {
				test.Errorf("%s: %s has no description", metadataResp.TypeName, name)
			}
		}
	}
}

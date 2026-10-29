package builder

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// entryDataSource reads the live bindings of an entry by name, for entries this configuration doesn't manage.
type entryDataSource[D Document[D, S], S any] struct {
	entry[D, S]
}

// Compile-time check that entryDataSource implements the framework data source interfaces.
var _ datasource.DataSourceWithConfigure = &entryDataSource[captureDocument[struct{}, struct{}], CaptureSpec[struct{}, struct{}]]{}

func newDataSource[D Document[D, S], S any](connector Connector, catalogType catalogType) func() datasource.DataSource {
	return func() datasource.DataSource {
		return &entryDataSource[D, S]{
			entry: newEntry[D, S](connector, catalogType),
		}
	}
}

func (entryDataSource *entryDataSource[D, S]) Metadata(_ context.Context, metadataReq datasource.MetadataRequest, metadataResp *datasource.MetadataResponse) {
	metadataResp.TypeName = entryDataSource.connector.typeName(metadataReq.ProviderTypeName)
}

func (entryDataSource *entryDataSource[D, S]) Schema(_ context.Context, _ datasource.SchemaRequest, schemaResp *datasource.SchemaResponse) {
	if entryDataSource.schemaError(&schemaResp.Diagnostics) {
		return
	}

	bindings, isList := entryDataSource.codec.Schema().Attributes[bindingsAttribute].(schema.ListNestedAttribute)

	if !isList {
		schemaResp.Diagnostics.AddError("Invalid connector schema", "The "+entryDataSource.catalogType.name+" has no bindings list.")
		return
	}

	schemaResp.Schema = datasourceschema.Schema{
		Attributes: map[string]datasourceschema.Attribute{
			nameAttribute: datasourceschema.StringAttribute{
				Description: entryDataSource.catalogType.nameDescription(),
				Required:    true,
			},
			liveBindingsAttribute: datasourceschema.ListAttribute{
				Description: liveBindingsDescription,
				Computed:    true,
				ElementType: bindings.NestedObject.Type(),
			},
		},
	}
}

func (entryDataSource *entryDataSource[D, S]) Configure(_ context.Context, configureReq datasource.ConfigureRequest, configureResp *datasource.ConfigureResponse) {
	entryDataSource.configure(configureReq.ProviderData, &configureResp.Diagnostics)
}

// Read reads the live bindings, failing when the entry doesn't exist rather than listing none, as building bindings
// from an empty list would remove them all.
func (entryDataSource *entryDataSource[D, S]) Read(ctx context.Context, readReq datasource.ReadRequest, readResp *datasource.ReadResponse) {
	var name types.String

	readResp.Diagnostics.Append(readReq.Config.GetAttribute(ctx, path.Root(nameAttribute), &name)...)

	if readResp.Diagnostics.HasError() {
		return
	}

	var catalogName string = entryDataSource.catalogName(name.ValueString())

	live, err := entryDataSource.readLiveValue(ctx, name.ValueString())

	if err != nil {
		readResp.Diagnostics.AddError("Reading "+entryDataSource.catalogType.name+" "+catalogName, err.Error())
		return
	}

	var stateType tftypes.Object = readResp.State.Schema.Type().TerraformType(ctx).(tftypes.Object)

	value, err := withLiveBindings(tftypes.NewValue(stateType, map[string]tftypes.Value{
		nameAttribute:         tftypes.NewValue(tftypes.String, name.ValueString()),
		liveBindingsAttribute: tftypes.NewValue(stateType.AttributeTypes[liveBindingsAttribute], nil),
	}), live)

	if err != nil {
		readResp.Diagnostics.AddError("Reading "+entryDataSource.catalogType.name+" "+catalogName, err.Error())
		return
	}

	readResp.State.Raw = value
}

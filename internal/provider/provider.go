package provider

import (
	"context"
	"os"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &EstuaryProvider{}

// EstuaryProvider is the Terraform provider for Estuary resources.
type EstuaryProvider struct {
	version string
}

// EstuaryProviderModel is the provider configuration block.
type EstuaryProviderModel struct {
	Backend     types.String `tfsdk:"backend"`
	Token       types.String `tfsdk:"token"`
	Tenant      types.String `tfsdk:"tenant"`
	FlowctlPath types.String `tfsdk:"flowctl_path"`
	DataPlane   types.String `tfsdk:"data_plane"`
}

// New returns an EstuaryProvider factory.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &EstuaryProvider{version: version}
	}
}

// Metadata sets the provider type name and version.
func (estuaryProvider *EstuaryProvider) Metadata(_ context.Context, _ provider.MetadataRequest, metadataResp *provider.MetadataResponse) {
	metadataResp.TypeName = "estuary"
	metadataResp.Version = estuaryProvider.version
}

// Schema declares the provider configuration. Explicit values override environment variables.
func (estuaryProvider *EstuaryProvider) Schema(_ context.Context, _ provider.SchemaRequest, schemaResp *provider.SchemaResponse) {
	schemaResp.Schema = schema.Schema{
		Description: "The Estuary provider manages Flow capture and materialization connectors through a configurable catalog backend.",
		Attributes: map[string]schema.Attribute{
			"backend": schema.StringAttribute{
				Description: "Flow catalog backend. Defaults to " + defaultBackend + ", currently the only one available. Can also be set via ESTUARY_BACKEND env var.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive(defaultBackend),
				},
			},
			"token": schema.StringAttribute{
				Description: "Estuary API Token. Can also be set via environment variable by using FLOW_AUTH_TOKEN.",
				Optional:    true,
				Sensitive:   true,
			},
			"tenant": schema.StringAttribute{
				Description: "Tenant prefixed to every catalog name that doesn't start with it, e.g. 'myorg/'. A missing trailing slash is added. " +
					"Can also be set via env variable using ESTUARY_TENANT",
				Optional: true,
			},
			"flowctl_path": schema.StringAttribute{
				Description: "Absolute path to the flowctl binary. Defaults to 'flowctl', which must be available on PATH.",
				Optional:    true,
			},
			"data_plane": schema.StringAttribute{
				Description: "Data plane the provider creates new captures and materializations in and runs their discovery on, by its internal ID. " +
					"Existing entries keep their data plane. Required for BYOC deployments or a non-default public data plane. Can also be set via env variable by using ESTUARY_DATA_PLANE.",
				Optional: true,
			},
		},
	}
}

// Configure builds the catalog.Session, holding the backend and tenant, shared by every resource and data source.
func (estuaryProvider *EstuaryProvider) Configure(ctx context.Context, configureReq provider.ConfigureRequest, configureResp *provider.ConfigureResponse) {
	var providerConfig EstuaryProviderModel

	configureResp.Diagnostics.Append(configureReq.Config.Get(ctx, &providerConfig)...)

	if configureResp.Diagnostics.HasError() {
		return
	}

	var token string = valueOr(providerConfig.Token, os.Getenv("FLOW_AUTH_TOKEN"))
	var tenant string = valueOr(providerConfig.Tenant, os.Getenv("ESTUARY_TENANT"))
	var dataPlane string = valueOr(providerConfig.DataPlane, os.Getenv("ESTUARY_DATA_PLANE"))
	var flowctlPath string = valueOr(providerConfig.FlowctlPath, "flowctl")
	var backend string = strings.ToLower(valueOr(providerConfig.Backend, os.Getenv("ESTUARY_BACKEND")))

	if backend == "" {
		backend = defaultBackend
	}

	if token == "" {
		configureResp.Diagnostics.AddError(
			"Missing Estuary Flow Auth Token", "Set the token provider attribute or the FLOW_AUTH_TOKEN environment variable.",
		)
		return
	}

	catalogBackend, err := newBackend(backend, backendOptions{
		flowctlPath: flowctlPath,
		token:       token,
		dataPlane:   dataPlane,
	})

	if err != nil {
		configureResp.Diagnostics.AddError("Unsupported Estuary backend", err.Error())
		return
	}

	var session *catalog.Session = &catalog.Session{
		Backend: catalogBackend,
		Tenant:  catalog.Tenant(tenant),
	}

	configureResp.ResourceData = session
	configureResp.DataSourceData = session
}

// Resources returns the supported connector resources, see resources.
func (estuaryProvider *EstuaryProvider) Resources(_ context.Context) []func() resource.Resource {
	return resources
}

// DataSources returns the provider data sources.
func (estuaryProvider *EstuaryProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return dataSources
}

// valueOr returns the attribute value when set, fallback otherwise.
func valueOr(attribute types.String, fallback string) string {
	if attribute.IsNull() || attribute.IsUnknown() {
		return fallback
	}

	return attribute.ValueString()
}

package provider

import (
	"context"
	"os"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/estuary/terraform-provider-estuary/internal/resources/capture/mongodb"
	"github.com/estuary/terraform-provider-estuary/internal/resources/materialization/postgres"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &EstuaryProvider{}

// EstuaryProvider is the root Terraform provider for Estuary's resources management
type EstuaryProvider struct {
	version string
}

type EstuaryProviderModel struct {
	Backend     types.String `tfsdk:"backend"`
	Token       types.String `tfsdk:"token"`
	Tenant      types.String `tfsdk:"tenant"`
	FlowctlPath types.String `tfsdk:"flowctl_path"`
	DataPlane   types.String `tfsdk:"data_plane"`
}

// New returns a factory function that creates an EstuaryProvider
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &EstuaryProvider{version: version}
	}
}

// Metadata sets the provider type and version which Terraform uses to identify resources and data sources belonging to this provider.
func (estuaryProvider *EstuaryProvider) Metadata(_ context.Context, _ provider.MetadataRequest, metadataResp *provider.MetadataResponse) {
	metadataResp.TypeName = "estuary"
	metadataResp.Version = estuaryProvider.version
}

// Schema declares provider configuration, including the backend selection. Explicit Values override environment fallbacks,
// and flowctl_path defaults to flowctl.
func (estuaryProvider *EstuaryProvider) Schema(_ context.Context, _ provider.SchemaRequest, schemaResp *provider.SchemaResponse) {
	schemaResp.Schema = schema.Schema{
		Description: "The Estuary provider manages Flow capture and materialization connectors through a configurable catalog backend.",
		Attributes: map[string]schema.Attribute{
			"backend": schema.StringAttribute{
				Description: "Flow catalog backend. Defaults to flowctl. Currently only flowctl is available. Can also be set via ESTUARY_BACKEND env var.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive("flowctl"),
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
				Description: "Target Estuary Data Plane, you must use the internal data plane ID. Required for BYOC deployments or when targeting a non-default public data plane. " +
					"Can also be set via env variable by using ESTUARY_DATA_PLANE.",
				Optional: true,
			},
		},
	}
}

// Configure resolves backend and connection settings, preferring explicit provider attributes over environment variables
// and initializes a shared catalog.Session, holding the backend and tenant, injected into every Terraform resource and data source.
func (estuaryProvider *EstuaryProvider) Configure(ctx context.Context, configureReq provider.ConfigureRequest, configureResp *provider.ConfigureResponse) {
	var providerConfig EstuaryProviderModel
	var token string = os.Getenv("FLOW_AUTH_TOKEN")
	var tenant string = os.Getenv("ESTUARY_TENANT")
	var dataPlane string = os.Getenv("ESTUARY_DATA_PLANE")
	var backend string = os.Getenv("ESTUARY_BACKEND")

	var flowctlPath string = "flowctl"

	configureResp.Diagnostics.Append(configureReq.Config.Get(ctx, &providerConfig)...)

	if configureResp.Diagnostics.HasError() {
		return
	}

	if !providerConfig.Token.IsNull() && !providerConfig.Token.IsUnknown() {
		token = providerConfig.Token.ValueString()
	}

	if !providerConfig.Tenant.IsNull() && !providerConfig.Tenant.IsUnknown() {
		tenant = providerConfig.Tenant.ValueString()
	}

	if !providerConfig.FlowctlPath.IsNull() && !providerConfig.FlowctlPath.IsUnknown() {
		flowctlPath = providerConfig.FlowctlPath.ValueString()
	}

	if !providerConfig.DataPlane.IsNull() && !providerConfig.DataPlane.IsUnknown() {
		dataPlane = providerConfig.DataPlane.ValueString()
	}

	if !providerConfig.Backend.IsNull() && !providerConfig.Backend.IsUnknown() {
		backend = strings.ToLower(providerConfig.Backend.ValueString())
	}

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
		configureResp.Diagnostics.AddError("Unsupported Estuary backend.", err.Error())
		return
	}

	var session *catalog.Session = &catalog.Session{
		Backend: catalogBackend,
		Tenant:  catalog.Tenant(tenant),
	}

	configureResp.ResourceData = session
	configureResp.DataSourceData = session
}

// Resources returns the full list of Terraform resource factories registered with this provider
func (estuaryProvider *EstuaryProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		postgres.New,
		mongodb.New,
	}
}

// DataSources returns the list of data source factories registered with this provider.
func (estuaryProvider *EstuaryProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

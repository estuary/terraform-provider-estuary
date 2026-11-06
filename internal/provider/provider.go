package provider

import (
	"context"
	"os"
	"strings"

	"github.com/estuary/terraform-provider-estuary/internal/catalog"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
		Description: "Manages Estuary captures (sources) and materializations (destinations).",
		Attributes: map[string]schema.Attribute{
			"backend": schema.StringAttribute{
				Description: "How the provider reaches Estuary. Defaults to " + defaultBackend + ", currently the only backend. " +
					"Can also be set through ESTUARY_BACKEND.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive(defaultBackend),
				},
			},
			"token": schema.StringAttribute{
				Description: "Estuary refresh token, from the dashboard under Admin > CLI-API. Access tokens also work but expire, " +
					"so use a refresh token in CI/CD. Can also be set through FLOW_AUTH_TOKEN, and one of them is required.",
				Optional:  true,
				Sensitive: true,
			},
			"tenant": schema.StringAttribute{
				Description: "Prefix added to every capture and materialization name that doesn't already start with it, such as " +
					"acmeCo/ or acmeCo/dev/. A missing trailing slash is added. Collection names you write yourself aren't prefixed. " +
					"Can also be set through ESTUARY_TENANT.",
				Optional: true,
			},
			"flowctl_path": schema.StringAttribute{
				Description: "Path to the flowctl binary. Defaults to flowctl on PATH.",
				Optional:    true,
			},
			"data_plane": schema.StringAttribute{
				Description: "Data plane that new captures and materializations are created in, and that capture discovery runs on " +
					"(e.g., ops/dp/public/example-data-plane). Required for BYOC or a non-default public data plane. Defaults to " +
					"the first data plane of your storage mapping. Existing entries keep their data plane. Can also be set through " +
					"ESTUARY_DATA_PLANE.",
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

	// An unknown attribute fails rather than falling back to its environment variable, which would run the plan under
	// another token or tenant than the apply.
	for _, setting := range []struct {
		attribute string
		value     types.String
	}{
		{"backend", providerConfig.Backend},
		{"token", providerConfig.Token},
		{"tenant", providerConfig.Tenant},
		{"flowctl_path", providerConfig.FlowctlPath},
		{"data_plane", providerConfig.DataPlane},
	} {
		if setting.value.IsUnknown() {
			configureResp.Diagnostics.AddAttributeError(path.Root(setting.attribute), "Provider "+setting.attribute+" unknown at plan time",
				"The provider's "+setting.attribute+" is only known after apply, but plans need it to read the catalog. Set it "+
					"from a value known at plan time, such as a variable, or first apply the resource it depends on with -target.")
		}
	}

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
			"Missing Estuary auth token", "Set the token attribute of the provider, or the FLOW_AUTH_TOKEN environment variable.",
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

// valueOr returns the attribute value when set, fallback otherwise. Configure rejects unknown attributes first.
func valueOr(attribute types.String, fallback string) string {
	if attribute.IsNull() {
		return fallback
	}

	return attribute.ValueString()
}

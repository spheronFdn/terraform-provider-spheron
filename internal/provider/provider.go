package provider

import (
	"context"
	"os"

	"terraform-provider-spheron/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ provider.Provider = &SpheronProvider{}

type SpheronProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

type SpheronProviderModel struct {
	Token types.String `tfsdk:"token"`
}

func (p *SpheronProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "spheron"
	resp.Version = p.version
}

func (p *SpheronProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				MarkdownDescription: "Spheron access token. If left empty provide SPHERON_TOKEN env variable.",
				Optional:            true,
			},
		},
		Blocks:              map[string]schema.Block{},
		MarkdownDescription: "Interface with the Spheron API.",
	}
}

func (p *SpheronProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Inventory client")

	var config SpheronProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Unknown Spheron access token",
			"The provider cannot create the Spheron API client as there is an unknown token value for the Spheron API token. "+
				"Either set the value directly in the provider, or use the SPHERON_TOKEN environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	token := os.Getenv("SPHERON_TOKEN")

	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}

	tflog.Debug(ctx, "Creating Spheron client")

	spheronApi, err := client.NewSpheronApi(token)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Spheron API Client",
			"An unexpected error occurred when creating the Spheron API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Spheron Client Error: "+err.Error(),
		)
		return
	}

	_, err = spheronApi.GetOrganization()

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Spheron API Client",
			"An unexpected error occurred when creating the Spheron API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Spheron Client Error: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = spheronApi
	resp.ResourceData = spheronApi
}

func (p *SpheronProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewInstanceResource,
		NewDomainResource,
		NewMarketplaceInstanceResource,
	}
}

func (p *SpheronProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOrganizationDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &SpheronProvider{
			version: version,
		}
	}
}

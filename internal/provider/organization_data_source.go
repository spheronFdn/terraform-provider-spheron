package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/spheron/terraform-provider-spheron/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &OrganizationDataSource{}

func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{}
}

type OrganizationDataSource struct {
	client *client.SpheronApi
}

type SpheronDataSourceModel struct {
	Name types.String `tfsdk:"name"`
	ID   types.String `tfsdk:"id"`
}

func (d *OrganizationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *OrganizationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Organization data source..",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Organization name.",
				Computed:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Organization identifier.",
				Computed:            true,
			},
		},
	}
}

func (d *OrganizationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.SpheronApi)
	if !ok {
		tflog.Error(ctx, "Unable to prepare Spheron API client.")
		return
	}
	d.client = client
}

func (d *OrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	tflog.Debug(ctx, "Preparing to read item data source.")
	var state SpheronDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	organization, err := d.client.GetOrganization()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get organization for provided access token.",
			err.Error(),
		)
		return
	}

	// Map response body to model
	state = SpheronDataSourceModel{
		ID:   types.StringValue(organization.ID),
		Name: types.StringValue(organization.Profile.Name),
	}

	// Set state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	tflog.Debug(ctx, "Finished reading item data source", map[string]any{"success": true})
}

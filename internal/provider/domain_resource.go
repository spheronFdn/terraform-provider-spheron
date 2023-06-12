package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/spheron/terraform-provider-spheron/internal/client"
)

// Ensure DomainResource satisfies the required interfaces
var _ resource.Resource = &DomainResource{}
var _ resource.ResourceWithImportState = &DomainResource{}

type DomainResource struct {
	client *client.SpheronApi
}

type DomainResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Verified     types.Bool   `tfsdk:"verified"`
	InstancePort types.Int64  `tfsdk:"instance_port"`
	Type         types.String `tfsdk:"type"`
	InstanceID   types.String `tfsdk:"instance_id"`
}

func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

func (r *DomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Instnce resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"verified": schema.BoolAttribute{
				Computed: true,
			},
			"instance_port": schema.Int64Attribute{
				Required: true,
			},
			"type": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"instance_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *DomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.SpheronApi)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DomainResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !isValidDomainType(plan.Type.ValueString()) {
		resp.Diagnostics.AddError("DomainType not supported.", "DomainType not supported. Supported domain types are: doain and subdomain.")
		return
	}

	instance, err := r.client.GetClusterInstance(plan.InstanceID.ValueString())

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create domain for instance",
			err.Error(),
		)
		return
	}

	order, err := r.client.GetClusterInstanceOrder(instance.ActiveOrder)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create domain for instance",
			err.Error(),
		)
		return
	}

	url := getInstanceDeploymentURL(order, int(plan.InstancePort.ValueInt64()))

	if url == "" {
		resp.Diagnostics.AddError(
			"Unable to create domain for instance",
			"Unable to create domain for instance",
		)
		return
	}

	domainRequest := client.DomainRequest{
		Name: plan.Name.ValueString(),
		Type: client.DomainTypeEnum(plan.Type.ValueString()),
		Link: url,
	}

	domain, err := r.client.AddClusterInstanceDomain(plan.InstanceID.ValueString(), domainRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create domain",
			err.Error(),
		)
		return
	}

	// populate the state from the domain
	plan.ID = types.StringValue(domain.ID)
	plan.Verified = types.BoolValue(domain.Verified)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DomainResourceModel
	tflog.Debug(ctx, "Preparing to read item resource")
	// Get current state
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// instance, err := r.client.GetClusterInstance(state.Id.ValueString())
	// if err != nil {
	// 	fmt.Println("Error:", err)
	// 	return
	// }

	// state.Image = types.StringValue(instance.)

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read example, got error: %s", err))
	//     return
	// }

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DomainResourceModel

	// Retrieve values from plan
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !isValidDomainType(plan.Type.ValueString()) {
		resp.Diagnostics.AddError("DomainType not supported.", "DomainType not supported. Supported domain types are: doain and subdomain.")
		return
	}

	instance, err := r.client.GetClusterInstance(plan.InstanceID.ValueString())

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create domain for instance",
			err.Error(),
		)
		return
	}

	order, err := r.client.GetClusterInstanceOrder(instance.ActiveOrder)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create domain for instance",
			err.Error(),
		)
		return
	}

	url := getInstanceDeploymentURL(order, int(plan.InstancePort.ValueInt64()))

	if url == "" {
		resp.Diagnostics.AddError(
			"Unable to create domain for instance",
			"Unable to create domain for instance",
		)
		return
	}

	domainRequest := client.DomainRequest{
		Name: plan.Name.ValueString(),
		Type: client.DomainTypeEnum(plan.Type.ValueString()),
		Link: url,
	}

	domain, err := r.client.UpdateClusterInstanceDomain(plan.InstanceID.ValueString(), plan.ID.ValueString(), domainRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create domain",
			err.Error(),
		)
		return
	}

	// populate the state from the domain
	plan.Verified = types.BoolValue(domain.Verified)

	// Save updated data into Terraform state
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Updated item resource", map[string]any{"success": true})
}

func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Debug(ctx, "Preparing to delete item resource")
	// Retrieve values from state
	var state DomainResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteClusterInstanceDomain(state.InstanceID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to destroy Instance",
			err.Error(),
		)
		return
	}

	tflog.Debug(ctx, "Domain deleted", map[string]any{"success": true})
}

func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func isValidDomainType(value string) bool {
	switch client.DomainTypeEnum(value) {
	case client.DomainTypeDomain, client.DomainTypeSubdomain:
		return true
	}
	return false
}

func getInstanceDeploymentURL(input client.InstanceOrder, desiredPort int) string {
	if desiredPort == 80 && input.URLPreview != "" {
		return input.URLPreview
	}

	if input.ProtocolData != nil && input.ProtocolData.ProviderHost != "" {
		for _, port := range input.ClusterInstanceConfiguration.Ports {
			if port.ContainerPort == desiredPort {
				return fmt.Sprintf("%s:%d", input.ProtocolData.ProviderHost, port.ExposedPort)
			}
		}
	}

	return ""
}

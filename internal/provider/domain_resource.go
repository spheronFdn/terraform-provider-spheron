package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"

	"terraform-provider-spheron/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

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
		MarkdownDescription: "Instance domain resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Id of the domain.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The domain name",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"verified": schema.BoolAttribute{
				MarkdownDescription: "Is veriffied. True means that the domain is verified and that it will start serving the content",
				Computed:            true,
			},
			"instance_port": schema.Int64Attribute{
				MarkdownDescription: "Container port of the instnace to whict to attach the domain.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of the domain. Available options are domain and subdomain.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"instance_id": schema.StringAttribute{
				MarkdownDescription: "The id of an instance to which to attach the domain.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *DomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
			"Unable to create domain for instance111",
			err.Error(),
		)
		return
	}

	order, err := r.client.GetClusterInstanceOrder(instance.ActiveOrder)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create domain for instance1112222",
			err.Error(),
		)
		return
	}

	url := getInstanceDeploymentURL(order, int(plan.InstancePort.ValueInt64()))

	if url == "" {
		resp.Diagnostics.AddError(
			"Unable to create domain for instance333, no urls",
			"Unable to create domain for instance333",
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

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.ID.ValueString() == "" || state.InstanceID.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Id or instanceId not provided. Unable to get domain details.",
			"Id or instanceId not provided. Unable to get domain details.",
		)
		return
	}

	domains, err := r.client.GetClusterInstanceDomains(state.InstanceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Coudn't fetch instance domains for provided instance id.",
			err.Error(),
		)
		return
	}

	instance, err := r.client.GetClusterInstance(state.InstanceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Coudn't fetch instance for specified domain.",
			err.Error(),
		)
		return
	}

	if instance.State == "Closed" || instance.ActiveOrder == "" {
		resp.State.RemoveResource(ctx)
		resp.Diagnostics.AddWarning("Instance domain was attached to is closed", fmt.Sprintf("Domain %s is attached to closed instance. Applying will attach domain to redeployed instance.", state.Name.ValueString()))
		return
	}

	order, err := r.client.GetClusterInstanceOrder(instance.ActiveOrder)
	if err != nil {
		resp.State.RemoveResource(ctx)
		resp.Diagnostics.AddWarning("Instance domain is attached to doesn't have provisioned deployments.",
			err.Error(),
		)
		return
	}

	domain, err := findDomainByID(domains, state.ID.ValueString())

	containerPort, err := getPortFromDeploymentURL(order, domain.Link)
	if err != nil {
		resp.State.RemoveResource(ctx)
		resp.Diagnostics.AddWarning("Instance doesn't have provisioned deployments.",
			err.Error(),
		)
		return
	}

	state.InstancePort = types.Int64Value(int64(containerPort))
	state.Name = types.StringValue(domain.Name)
	state.Verified = types.BoolValue(domain.Verified)
	state.Type = types.StringValue(string(domain.Type))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
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

	domain, err := r.client.UpdateClusterInstanceDomain(plan.InstanceID.ValueString(), plan.ID.ValueString(), domainRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create domain",
			err.Error(),
		)
		return
	}

	plan.Verified = types.BoolValue(domain.Verified)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Updated item resource", map[string]any{"success": true})
}

func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Debug(ctx, "Preparing to delete item resource")
	var state DomainResourceModel

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

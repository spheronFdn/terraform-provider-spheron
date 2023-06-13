package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/spheron/terraform-provider-spheron/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MarketplaceInstanceResource{}
var _ resource.ResourceWithImportState = &MarketplaceInstanceResource{}

func NewMarketplaceInstanceResource() resource.Resource {
	return &MarketplaceInstanceResource{}
}

// ExampleResource defines the resource implementation.
type MarketplaceInstanceResource struct {
	client *client.SpheronApi
}

// ExampleResourceModel describes the resource data model.
type MarketplaceInstanceResourceModel struct {
	Env          []Env        `tfsdk:"env"`
	Region       types.String `tfsdk:"region"`
	Name         types.String `tfsdk:"name"`
	MachineImage types.String `tfsdk:"machine_image"`
	Id           types.String `tfsdk:"id"`
}

func (r *MarketplaceInstanceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_marketplace_instance"
}

func (r *MarketplaceInstanceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Instnce resource",

		Attributes: map[string]schema.Attribute{
			"region": schema.StringAttribute{
				MarkdownDescription: "Region to which to deploy instance.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"machine_image": schema.StringAttribute{
				MarkdownDescription: "Machine image name which should be used for deploying instance.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the marketplace app.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"env": schema.ListNestedAttribute{
				MarkdownDescription: "The list of environmetnt variables. NOTE: Some marketplace apps have required env variables that must be provided.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "Environment variable key.",
							Required:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Environment variable value.",
							Required:            true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
						},
					},
				},
				Optional: true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Id or the instance.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *MarketplaceInstanceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MarketplaceInstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MarketplaceInstanceResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	organization, err := r.client.GetOrganization()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get organization",
			err.Error(),
		)
		return
	}

	region := plan.Region.ValueString()
	if region == "" {
		region = "any"
	}

	computeMachines, err := r.client.GetComputeMachines()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get fetch available compute machines.",
			err.Error(),
		)
		return
	}

	chosenMachineID, err := findComputeMachineID(computeMachines, plan.MachineImage.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get machine image by provided name.",
			err.Error(),
		)
		return
	}

	marketplaceApps, err := r.client.GetClusterTemplates()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get available markeplace apps.",
			err.Error(),
		)
		return
	}

	chosenMarketplaceApp, err := findMarketplaceAppByName(marketplaceApps, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get marketplace app by provided name.",
			err.Error(),
		)
		return
	}

	topicId := uuid.New()
	deploymentEnv, err := checkRequiredDeploymentVariables(chosenMarketplaceApp.ServiceData.Variables, mapMarketplaceEnvs(plan.Env))

	if err != nil {
		resp.Diagnostics.AddError(
			"Required env variable not set!",
			err.Error(),
		)
		return
	}

	instanceConfig := client.CreateInstanceFromMarketplaceRequest{
		TemplateID:           chosenMarketplaceApp.ID,
		EnvironmentVariables: deploymentEnv,
		OrganizationID:       organization.ID,
		AkashImageID:         chosenMachineID,
		UniqueTopicID:        topicId.String(),
		Region:               region,
	}

	response, err := r.client.CreateClusterInstanceFromTemplate(instanceConfig)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy instance from marketplace app.",
			err.Error(),
		)
		return
	}

	deployed, err := r.client.WaitForDeployedEvent(topicId.String())

	if err != nil || !deployed {
		resp.Diagnostics.AddError(
			"Marketplace instance deployment failed.",
			"Marketplace instance deployment failed.",
		)
		return
	}

	// Map response body to model
	plan.Id = types.StringValue(response.ClusterInstanceID)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Save data into Terraform state
	tflog.Debug(ctx, "Created item resource", map[string]any{"success": true})
}

func (r *MarketplaceInstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state MarketplaceInstanceResourceModel
	tflog.Debug(ctx, "Preparing to read item resource.")
	// Get current state
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *MarketplaceInstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MarketplaceInstanceResourceModel

	// Retrieve values from plan
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Updated item resource", map[string]any{"success": true})
}

func (r *MarketplaceInstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Debug(ctx, "Preparing to delete item resource")
	// Retrieve values from state
	var state MarketplaceInstanceResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CloseClusterInstance(state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to destroy marketplace instance",
			err.Error(),
		)
		return
	}
	tflog.Debug(ctx, "Instance closed", map[string]any{"success": true})
}

func (r *MarketplaceInstanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func findComputeMachineID(machines []client.ComputeMachine, name string) (string, error) {
	for _, machine := range machines {
		if machine.Name == name {
			return machine.ID, nil
		}
	}

	return "", errors.New("ComputeMachine not found with the provided ID")
}

func findMarketplaceAppByName(apps []client.MarketplaceApp, name string) (client.MarketplaceApp, error) {
	for _, app := range apps {
		if app.Name == name {
			return app, nil
		}
	}

	return client.MarketplaceApp{}, fmt.Errorf("MarketplaceApp not found with name: %s", name)
}

func mapMarketplaceEnvs(envList []Env) []client.MarketplaceDeploymentVariable {
	marketplaceEnvs := make([]client.MarketplaceDeploymentVariable, 0, len(envList))
	for i, env := range envList {
		marketplaceEnvs[i] = client.MarketplaceDeploymentVariable{
			Value: env.Value.ValueString(),
			Label: env.Key.ValueString(),
		}
	}
	return marketplaceEnvs
}

func checkRequiredDeploymentVariables(appVariables []client.MarketplaceAppVariable, deploymentVariables []client.MarketplaceDeploymentVariable) ([]client.MarketplaceDeploymentVariable, error) {
	allVariables := make(map[string]client.MarketplaceDeploymentVariable)
	missingVariables := make(map[string]bool)

	for _, appVar := range appVariables {
		deploymentVar := client.MarketplaceDeploymentVariable{
			Label: appVar.Label,
			Value: appVar.DefaultValue,
		}
		allVariables[appVar.Label] = deploymentVar

		if appVar.Required {
			missingVariables[appVar.Label] = true
		}
	}

	for _, depVar := range deploymentVariables {
		if depVar, ok := allVariables[depVar.Label]; ok {
			allVariables[depVar.Label] = depVar
			missingVariables[depVar.Label] = false
		}
	}

	for varName, isMissing := range missingVariables {
		if isMissing {
			return nil, errors.New(fmt.Sprintf("Missing required deployment variable: %s", varName))
		}
	}

	updatedDeploymentVariables := make([]client.MarketplaceDeploymentVariable, 0, len(allVariables))
	for _, variable := range allVariables {
		updatedDeploymentVariables = append(updatedDeploymentVariables, variable)
	}

	return updatedDeploymentVariables, nil
}

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
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
	Region       types.String `tfsdk:"region"`
	Name         types.String `tfsdk:"name"`
	MachineImage types.String `tfsdk:"machine_image"`
	Ports        types.List   `tfsdk:"ports"`
	Env          types.Set    `tfsdk:"env"`
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
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
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
			"env": schema.SetNestedAttribute{
				MarkdownDescription: "The list of environmetnt variables. NOTE: Some marketplace apps have required env variables that must be provided.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "Environment variable key.",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Environment variable value.",
							Required:            true,
						},
					},
				},
				Optional: true,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
			"ports": schema.ListNestedAttribute{
				MarkdownDescription: "The list of port mappings",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"container_port": schema.Int64Attribute{
							MarkdownDescription: "Container port that will be exposed.",
							Computed:            true,
						},
						"exposed_port": schema.Int64Attribute{
							MarkdownDescription: "The port container port will be exposed to. Exposed port will be know and available for use after the deployment.",
							Computed:            true,
						},
					},
				},
				Computed: true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
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

	envList := make([]Env, 0, len(plan.Env.Elements()))
	plan.Env.ElementsAs(ctx, &envList, false)

	deploymentEnv, err := checkRequiredDeploymentVariables(chosenMarketplaceApp.ServiceData.Variables, envList)
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

	time.Sleep(5 * time.Second)

	order, err := r.client.GetClusterInstanceOrder(response.ClusterInstanceOrderID)

	if err != nil || !deployed {
		resp.Diagnostics.AddError(
			"Instance deployment failed",
			err.Error(),
		)
		return
	}

	plan.Id = types.StringValue(response.ClusterInstanceID)
	plan.Ports = types.ListValueMust(types.ObjectType{AttrTypes: getPortAtrTypes()}, mapModelPortToPortValue(order.ClusterInstanceConfiguration.Ports))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

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

	if state.Id.IsNull() {
		resp.Diagnostics.AddError(
			"Id not provided. Unable to get marketplace instance details.",
			"Id not provided. Unable to get marketplace instance details.",
		)
		return
	}

	instance, err := r.client.GetClusterInstance(state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Coudnt fetch instance by provided id.",
			err.Error(),
		)
		return
	}
	cluster, err := r.client.GetCluster(instance.Cluster)
	if err != nil {
		resp.Diagnostics.AddError(
			"Instance cluster not found.",
			err.Error(),
		)
		return
	}

	order, err := r.client.GetClusterInstanceOrder(instance.ActiveOrder)
	if err != nil {
		state.MachineImage = types.StringValue("")
		state.Region = types.StringValue("")
		state.Name = types.StringValue(cluster.Name)

		// Save updated data into Terraform state
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

		return
	}

	jsona, err := json.Marshal(order)
	tflog.Info(ctx, fmt.Sprintf("got ports %s", jsona))

	ports, diag := types.ListValue(types.ObjectType{AttrTypes: getPortAtrTypes()}, mapModelPortToPortValue(order.ClusterInstanceConfiguration.Ports))
	if diag.HasError() {
		resp.Diagnostics.Append(diag.Errors()...)
		return
	}

	if len(order.ClusterInstanceConfiguration.Env) != 0 {
		envs, diag := types.SetValue(types.ObjectType{AttrTypes: getEnvAtrTypes()}, mapClientEnvsToEnvsValue(order.ClusterInstanceConfiguration.Env, false))
		if diag.HasError() {
			resp.Diagnostics.Append(diag.Errors()...)
			return
		}

		state.Env = envs
	}

	state.Ports = ports
	state.MachineImage = types.StringValue(order.ClusterInstanceConfiguration.AgreedMachineImage.MachineType)
	state.Region = types.StringValue(order.ClusterInstanceConfiguration.Region)
	state.Name = types.StringValue(cluster.Name)

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
	for _, env := range envList {
		marketplaceEnv := client.MarketplaceDeploymentVariable{
			Value: env.Value.ValueString(),
			Label: env.Key.ValueString(),
		}
		marketplaceEnvs = append(marketplaceEnvs, marketplaceEnv)
	}
	return marketplaceEnvs
}

func checkRequiredDeploymentVariables(appVariables []client.MarketplaceAppVariable, envList []Env) ([]client.MarketplaceDeploymentVariable, error) {
	allVariables := make(map[string]client.MarketplaceAppVariable)
	missingVariables := make(map[string]bool)
	deploymentVariables := make([]client.MarketplaceDeploymentVariable, 0, len(envList))

	for _, appVar := range appVariables {
		allVariables[appVar.Name] = appVar
		missingVariables[appVar.Name] = true
	}

	for _, env := range envList {
		if appVar, ok := allVariables[env.Key.ValueString()]; ok {
			marketplaceEnv := client.MarketplaceDeploymentVariable{
				Value: env.Value.ValueString(),
				Label: appVar.Label,
			}
			deploymentVariables = append(deploymentVariables, marketplaceEnv)

			missingVariables[appVar.Name] = false
		}
	}

	for varName, isMissing := range missingVariables {
		if isMissing {
			return nil, errors.New(fmt.Sprintf("Missing required deployment variable: %s", varName))
		}
	}

	return deploymentVariables, nil
}

func mapModelPortToPortValue(portList []client.Port) []attr.Value {
	ports := make([]attr.Value, len(portList))
	for i, pm := range portList {
		portTypes := make(map[string]attr.Type)
		portValues := make(map[string]attr.Value)

		portTypes["container_port"] = types.Int64Type
		portTypes["exposed_port"] = types.Int64Type

		portValues["container_port"] = types.Int64Value(int64(pm.ContainerPort))
		portValues["exposed_port"] = types.Int64Value(int64(pm.ExposedPort))
		port := types.ObjectValueMust(portTypes, portValues)

		ports[i] = port

		fmt.Printf("added %s", ports)
	}
	return ports
}

func getPortAtrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"container_port": types.Int64Type,
		"exposed_port":   types.Int64Type,
	}
}

func mapClientEnvsToEnvsValue(clientEnvs []client.Env, isSecret bool) []attr.Value {
	if len(clientEnvs) == 0 {
		return nil
	}

	envList := make([]attr.Value, 0, len(clientEnvs))

	for _, clientEnv := range clientEnvs {
		if clientEnv.IsSecret != isSecret {
			continue
		}

		split := strings.SplitN(clientEnv.Value, "=", 2)
		keyString, valueString := split[0], split[1]

		portTypes := make(map[string]attr.Type)
		portValues := make(map[string]attr.Value)

		portTypes["key"] = types.StringType
		portTypes["value"] = types.StringType

		portValues["key"] = types.StringValue(keyString)
		portValues["value"] = types.StringValue(valueString)

		envList = append(envList, types.ObjectValueMust(portTypes, portValues))
	}
	return envList
}

func getEnvAtrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key":   types.StringType,
		"value": types.StringType,
	}
}

package provider

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"terraform-provider-spheron/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &InstanceResource{}
var _ resource.ResourceWithImportState = &InstanceResource{}

func NewInstanceResource() resource.Resource {
	return &InstanceResource{}
}

type InstanceResource struct {
	client *client.SpheronApi
}

type InstanceResourceModel struct {
	Image        types.String `tfsdk:"image"`
	Tag          types.String `tfsdk:"tag"`
	ClusterName  types.String `tfsdk:"cluster_name"`
	Ports        []Port       `tfsdk:"ports"`
	Env          []Env        `tfsdk:"env"`
	EnvSecret    []Env        `tfsdk:"env_secret"`
	Commands     []string     `tfsdk:"commands"`
	Args         []string     `tfsdk:"args"`
	Region       types.String `tfsdk:"region"`
	MachineImage types.String `tfsdk:"machine_image"`
	Id           types.String `tfsdk:"id"`
	HealthCheck  types.Object `tfsdk:"health_check"`
}

type Port struct {
	ContainerPort types.Int64 `tfsdk:"container_port"`
	ExposedPort   types.Int64 `tfsdk:"exposed_port"`
}

type Env struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type HealthCheck struct {
	Port types.Int64  `tfsdk:"port"`
	Path types.String `tfsdk:"path"`
}

func (r *InstanceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_instance"
}

func (r *InstanceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Instnce resource",

		Attributes: map[string]schema.Attribute{
			"image": schema.StringAttribute{
				MarkdownDescription: "The docker image to deploy. Currently only public dockerhub images are supported.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tag": schema.StringAttribute{
				MarkdownDescription: "The tag of docker image.",
				Required:            true,
			},
			"cluster_name": schema.StringAttribute{
				MarkdownDescription: "The name of the cluster.",
				Required:            true,
			},
			"ports": schema.ListNestedAttribute{
				MarkdownDescription: "The list of port mappings",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"container_port": schema.Int64Attribute{
							MarkdownDescription: "Container port that will be exposed.",
							Required:            true,
						},
						"exposed_port": schema.Int64Attribute{
							MarkdownDescription: "The port container port will be exposed to. Currently only posible to expose to port 80. Leave empty to map to random value. Exposed port will be know and available for use after the deployment.",
							Optional:            true,
							Computed:            true,
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
					},
				},
				Optional: true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"env": schema.SetNestedAttribute{
				MarkdownDescription: "The list of environmetnt variables.",
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
			},
			"env_secret": schema.SetNestedAttribute{
				MarkdownDescription: "The list of secret environmetnt variables.",
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
			},
			"commands": schema.ListAttribute{
				MarkdownDescription: "List of executables for docker CMD command.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"args": schema.ListAttribute{
				MarkdownDescription: "List of params for docker CMD command.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Region to which to deploy instance.",
				Required:            true,
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
			"health_check": schema.ObjectAttribute{
				MarkdownDescription: "Path and container port on which health check should be done.",
				AttributeTypes: map[string]attr.Type{
					"path": types.StringType,
					"port": types.Int64Type,
				},
				Optional: true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Id of the instance.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *InstanceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *InstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan InstanceResourceModel

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

	var healthCheck HealthCheck
	opts := basetypes.ObjectAsOptions{}
	plan.HealthCheck.As(ctx, &healthCheck, opts)

	topicId := uuid.New()

	instanceConfig := client.InstanceConfiguration{
		FolderName:            "",
		Protocol:              client.ClusterProtocolAkash,
		Image:                 plan.Image.ValueString(),
		Tag:                   plan.Tag.ValueString(),
		InstanceCount:         1,
		BuildImage:            false,
		Ports:                 mapPortToPortModel(plan.Ports),
		Env:                   append(mapEnvsToClientEnvs(plan.Env, false), mapEnvsToClientEnvs(plan.EnvSecret, true)...),
		Command:               plan.Commands,
		Args:                  plan.Args,
		Region:                plan.Region.ValueString(),
		AkashMachineImageName: plan.MachineImage.ValueString(),
	}

	createRequest := client.CreateInstanceRequest{
		OrganizationID:  organization.ID,
		UniqueTopicID:   topicId.String(),
		Configuration:   instanceConfig,
		ClusterURL:      plan.Image.ValueString(),
		ClusterProvider: "DOCKERHUB",
		ClusterName:     plan.ClusterName.ValueString(),
		HealthCheckURL:  healthCheck.Path.ValueString(),
		HealthCheckPort: healthCheck.Port.String(),
	}

	response, err := r.client.CreateClusterInstance(createRequest)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy instance",
			err.Error(),
		)
		return
	}

	eventDataString, err := r.client.WaitForDeployedEvent(topicId.String())

	if err != nil {
		resp.Diagnostics.AddError(
			"Instance deployment failed.",
			fmt.Sprintf("Instance deployment on cluster %s failed.", plan.ClusterName.ValueString()),
		)
		return
	}

	ports, err := ParseClientPorts(eventDataString)
	if err != nil {
		resp.Diagnostics.AddError(
			"Instance deployment failed.",
			fmt.Sprintf("Instance deployment on cluster %s failed.", plan.ClusterName.ValueString()),
		)
		return
	}

	plan.Id = types.StringValue(response.ClusterInstanceID)
	plan.Ports = mapModelPortToPort(ports)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Created item resource", map[string]any{"success": true})
}

func (r *InstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state InstanceResourceModel
	tflog.Debug(ctx, "Preparing to read item resource")
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.Id.IsNull() {
		resp.Diagnostics.AddError(
			"Id not provided. Unable to get instance details.",
			"Id not provided. Unable to get instance details.",
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

	if instance.State == "Closed" {
		resp.State.RemoveResource(ctx)
		resp.Diagnostics.AddWarning("Instance is closed", fmt.Sprintf("Instance %s, in cluster %s is closed. Applying will redeploy new instance in its place.", instance.ID, state.ClusterName.ValueString()))
		return
	}

	order, err := r.client.GetClusterInstanceOrder(instance.ActiveOrder)
	if err != nil {
		resp.Diagnostics.AddError(
			"Instance doesn't have provisioned deployments.",
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

	state.Args = order.ClusterInstanceConfiguration.Args
	state.ClusterName = types.StringValue(cluster.Name)
	state.Commands = order.ClusterInstanceConfiguration.Command
	state.Env = mapClientEnvsToEnvs(order.ClusterInstanceConfiguration.Env, false)
	state.EnvSecret = mapClientEnvsToEnvs(order.ClusterInstanceConfiguration.Env, true)

	if instance.HealthCheck.Port != (client.Port{}) {
		hcTypes := make(map[string]attr.Type)
		hcValues := make(map[string]attr.Value)

		hcTypes["port"] = types.Int64Type
		hcTypes["path"] = types.StringType

		hcValues["port"] = types.Int64Value(int64(instance.HealthCheck.Port.ContainerPort))
		hcValues["path"] = types.StringValue(instance.HealthCheck.URL)

		state.HealthCheck = types.ObjectValueMust(hcTypes, hcValues)
	}

	state.Image = types.StringValue(order.ClusterInstanceConfiguration.Image)
	state.MachineImage = types.StringValue(order.ClusterInstanceConfiguration.AgreedMachineImage.MachineType)
	state.Ports = mapModelPortToPort(order.ClusterInstanceConfiguration.Ports)
	state.Region = types.StringValue(order.ClusterInstanceConfiguration.Region)
	state.Tag = types.StringValue(order.ClusterInstanceConfiguration.Tag)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *InstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan InstanceResourceModel

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

	var healthCheck HealthCheck
	opts := basetypes.ObjectAsOptions{}
	plan.HealthCheck.As(ctx, &healthCheck, opts)

	if !healthCheck.Path.IsNull() && !healthCheck.Port.IsNull() {

		hcUpdate := client.HealthCheckUpdateReq{
			HealthCheckURL:  healthCheck.Path.ValueString(),
			HealthCheckPort: int(healthCheck.Port.ValueInt64()),
		}

		_, err := r.client.UpdateClusterInstanceHealthCheckInfo(plan.Id.ValueString(), hcUpdate)

		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to update instance healthchek endpoint.",
				err.Error(),
			)
			return
		}
	}

	instance, err := r.client.GetClusterInstance(plan.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Coudnt fetch instance by provided id.",
			err.Error(),
		)
		return
	}

	order, err := r.client.GetClusterInstanceOrder(instance.ActiveOrder)
	if err != nil {
		resp.Diagnostics.AddError(
			"Instance doesn't have provisioned deployments.",
			err.Error(),
		)
		return
	}

	envs := append(mapEnvsToClientEnvs(plan.Env, false), mapEnvsToClientEnvs(plan.EnvSecret, true)...)

	argsEqual := reflect.DeepEqual(order.ClusterInstanceConfiguration.Args, plan.Args)
	commandEqual := reflect.DeepEqual(order.ClusterInstanceConfiguration.Command, plan.Commands)
	envEqual := reflect.DeepEqual(envs, order.ClusterInstanceConfiguration.Env)
	tagEqual := plan.Tag.ValueString() == order.ClusterInstanceConfiguration.Tag

	if !argsEqual || !commandEqual || !envEqual || !tagEqual {
		topicId := uuid.New()

		updateRequest := client.UpdateInstanceRequest{
			Env:            envs,
			Command:        plan.Commands,
			Args:           plan.Args,
			UniqueTopicID:  topicId.String(),
			Tag:            plan.Tag.ValueString(),
			OrganizationID: organization.ID,
		}

		_, err = r.client.UpdateClusterInstance(plan.Id.ValueString(), updateRequest)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to update instance.",
				err.Error(),
			)
			return
		}

		_, err = r.client.WaitForDeployedEvent(topicId.String())

		if err != nil {
			resp.Diagnostics.AddError(
				"Instance deployment failed",
				err.Error(),
			)
			return
		}
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Updated item resource", map[string]any{"success": true})
}

func (r *InstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Debug(ctx, "Preparing to delete item resource")
	var state InstanceResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CloseClusterInstance(state.Id.ValueString())
	if err != nil && err.Error() != "Instance already closed" {
		resp.Diagnostics.AddError(
			"Unable to destroy Instance",
			err.Error(),
		)
		return
	}
	tflog.Debug(ctx, "Instance closed", map[string]any{"success": true})
}

func (r *InstanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

package provider

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
	Image             types.String `tfsdk:"image"`
	Tag               types.String `tfsdk:"tag"`
	ClusterName       types.String `tfsdk:"cluster_name"`
	Ports             []Port       `tfsdk:"ports"`
	Env               []Env        `tfsdk:"env"`
	EnvSecret         []Env        `tfsdk:"env_secret"`
	Commands          []string     `tfsdk:"commands"`
	Args              []string     `tfsdk:"args"`
	Region            types.String `tfsdk:"region"`
	MachineImage      types.String `tfsdk:"machine_image"`
	Id                types.String `tfsdk:"id"`
	HealthCheck       types.Object `tfsdk:"health_check"`
	Storage           types.Int64  `tfsdk:"storage"`
	Cpu               types.String `tfsdk:"cpu"`
	Memory            types.String `tfsdk:"memory"`
	Replicas          types.Int64  `tfsdk:"replicas"`
	PersistentStorage types.Object `tfsdk:"persistent_storage"`
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

type PersistentStorage struct {
	Class      types.String `tfsdk:"class"`
	MountPoint types.String `tfsdk:"mount_point"`
	Size       types.Int64  `tfsdk:"size"`
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
			"storage": schema.Int64Attribute{
				MarkdownDescription: "Instance storage in GB. Value cannot exceed 1024GB",
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
					int64validator.AtMost(1024),
				},
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"cpu": schema.StringAttribute{
				MarkdownDescription: "Instance CPU. Value cannot exceed 1024GB",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(
						"0.5",
						"1",
						"2",
						"4",
						"8",
						"16",
						"32",
					),
					stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("memory")),
					stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("machine_image")),
				},
			},
			"memory": schema.StringAttribute{
				MarkdownDescription: "Instance Memory in GB.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(
						"0.5",
						"1",
						"2",
						"4",
						"8",
						"16",
						"32",
					),
					stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("cpu")),
					stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("machine_image")),
				},
			},
			"replicas": schema.Int64Attribute{
				MarkdownDescription: "Number of instance replicas.",
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
					int64validator.AtMost(20),
				},
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
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
				Required: true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
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
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("memory")),
					stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("cpu")),
				},
			},
			"health_check": schema.SingleNestedAttribute{
				MarkdownDescription: "Path and container port on which health check should be done.",
				Attributes: map[string]schema.Attribute{
					"path": schema.StringAttribute{
						MarkdownDescription: "Path on which health check should be done.",
						Required:            true,
					},
					"port": schema.Int64Attribute{
						MarkdownDescription: "Instance container path on which health check should be done.",
						Required:            true,
						Validators: []validator.Int64{
							int64validator.AtLeast(1),
							int64validator.AtMost(65353),
						},
					},
				},
				Optional: true,
			},
			"persistent_storage": schema.SingleNestedAttribute{
				MarkdownDescription: "Persistent storage that will be attached to the instance.",
				Attributes: map[string]schema.Attribute{
					"class": schema.StringAttribute{
						MarkdownDescription: "Storage class. Available classes are HDD, SSD and NVMe",
						Required:            true,
						Validators: []validator.String{stringvalidator.OneOf(
							"HDD",
							"SSD",
							"NVMe",
						)},
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"mount_point": schema.StringAttribute{
						MarkdownDescription: "Attachement point used fot attaching persistent storage.",
						Required:            true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"size": schema.Int64Attribute{
						MarkdownDescription: "Persistent storage in GB. Value cannot exceed 1024GB",
						Required:            true,
						Validators: []validator.Int64{
							int64validator.AtLeast(1),
							int64validator.AtMost(1024),
						},
						PlanModifiers: []planmodifier.Int64{
							int64planmodifier.RequiresReplace(),
						},
					},
				},
				Optional: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
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

	opts := basetypes.ObjectAsOptions{}

	var customSpecs = client.CustomInstanceSpecs{
		Storage: fmt.Sprintf("%dGi", int(plan.Storage.ValueInt64())),
	}

	if !plan.PersistentStorage.IsNull() {
		var persistentStorage PersistentStorage
		plan.PersistentStorage.As(ctx, &persistentStorage, opts)

		value, _ := GetPersistentStorageClassEnum(persistentStorage.Class.ValueString())

		customSpecs.PersistentStorage = client.PersistentStorage{
			Class:      value,
			MountPoint: persistentStorage.MountPoint.ValueString(),
			Size:       fmt.Sprintf("%dGi", int(persistentStorage.Size.ValueInt64())),
		}
	}

	topicId := uuid.New()

	instanceConfig := client.InstanceConfiguration{
		FolderName:    "",
		Protocol:      client.ClusterProtocolAkash,
		Image:         plan.Image.ValueString(),
		Tag:           plan.Tag.ValueString(),
		InstanceCount: int(plan.Replicas.ValueInt64()),
		BuildImage:    false,
		Ports:         mapPortToPortModel(plan.Ports),
		Env:           append(mapEnvsToClientEnvs(plan.Env, false), mapEnvsToClientEnvs(plan.EnvSecret, true)...),
		Command:       plan.Commands,
		Args:          plan.Args,
		Region:        plan.Region.ValueString(),
	}

	if plan.MachineImage.ValueString() == "" {
		customSpecs.CPU = plan.Cpu.ValueString()
		customSpecs.Memory = fmt.Sprintf("%sGi", plan.Memory.ValueString())

		plan.MachineImage = types.StringValue("Custom Plan")
	} else {
		instanceConfig.AkashMachineImageName = plan.MachineImage.ValueString()
	}

	instanceConfig.CustomInstanceSpecs = customSpecs

	createRequest := client.CreateInstanceRequest{
		OrganizationID:  organization.ID,
		UniqueTopicID:   topicId.String(),
		Configuration:   instanceConfig,
		ClusterURL:      plan.Image.ValueString(),
		ClusterProvider: "DOCKERHUB",
		ClusterName:     plan.ClusterName.ValueString(),
	}

	if !plan.HealthCheck.IsNull() {
		var healthCheck HealthCheck
		plan.HealthCheck.As(ctx, &healthCheck, opts)

		createRequest.HealthCheckURL = healthCheck.Path.ValueString()
		createRequest.HealthCheckPort = healthCheck.Port.String()
	}

	response, err := r.client.CreateClusterInstance(createRequest)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy instance",
			err.Error(),
		)
		return
	}

	eventDataString, err := r.client.WaitForDeployedEvent(ctx, topicId.String())

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

	if plan.Cpu.ValueString() == "" || plan.Memory.ValueString() == "" {
		order, err := r.client.GetClusterInstanceOrder(response.ClusterInstanceOrderID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Instance doesn't have provisioned deployments.",
				err.Error(),
			)
			return
		}

		plan.Memory = types.StringValue(RemoveGiSuffix(order.ClusterInstanceConfiguration.AgreedMachineImage.Memory))
		plan.Cpu = types.StringValue(fmt.Sprint(order.ClusterInstanceConfiguration.AgreedMachineImage.Cpu))
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

	if state.Id.ValueString() == "" {
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
	state.Image = types.StringValue(order.ClusterInstanceConfiguration.Image)
	state.MachineImage = types.StringValue(order.ClusterInstanceConfiguration.AgreedMachineImage.MachineType)
	state.Ports = mapModelPortToPort(order.ClusterInstanceConfiguration.Ports)
	state.Region = types.StringValue(order.ClusterInstanceConfiguration.Region)
	state.Tag = types.StringValue(order.ClusterInstanceConfiguration.Tag)
	state.Replicas = types.Int64Value(int64(order.ClusterInstanceConfiguration.InstanceCount))

	numberStr := RemoveGiSuffix(order.ClusterInstanceConfiguration.AgreedMachineImage.Storage) // Remove the last two characters ("Gi")
	number, _ := strconv.Atoi(numberStr)
	state.Storage = types.Int64Value(int64(number))

	state.Memory = types.StringValue(RemoveGiSuffix(order.ClusterInstanceConfiguration.AgreedMachineImage.Memory))
	state.Cpu = types.StringValue(fmt.Sprint(order.ClusterInstanceConfiguration.AgreedMachineImage.Cpu))

	if instance.HealthCheck.Port != (client.Port{}) {
		hcTypes := make(map[string]attr.Type)
		hcValues := make(map[string]attr.Value)

		hcTypes["port"] = types.Int64Type
		hcTypes["path"] = types.StringType

		hcValues["port"] = types.Int64Value(int64(instance.HealthCheck.Port.ContainerPort))
		hcValues["path"] = types.StringValue(instance.HealthCheck.URL)

		state.HealthCheck = types.ObjectValueMust(hcTypes, hcValues)
	}

	if order.ClusterInstanceConfiguration.AgreedMachineImage.PersistentStorage != nil &&
		order.ClusterInstanceConfiguration.AgreedMachineImage.PersistentStorage.Class != "" {
		var pStorage = order.ClusterInstanceConfiguration.AgreedMachineImage.PersistentStorage

		psTypes := make(map[string]attr.Type)
		psValues := make(map[string]attr.Value)

		psTypes["class"] = types.StringType
		psTypes["mount_point"] = types.StringType
		psTypes["size"] = types.Int64Type

		value, _ := GetStorageClassFromValue(pStorage.Class)

		psValues["class"] = types.StringValue(value)
		psValues["mount_point"] = types.StringValue(pStorage.MountPoint)

		numberStr := pStorage.Size[:len(pStorage.Size)-2] // Remove the last two characters ("Gi")
		number, _ := strconv.Atoi(numberStr)
		psValues["size"] = types.Int64Value(int64(number))

		state.PersistentStorage = types.ObjectValueMust(psTypes, psValues)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *InstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan InstanceResourceModel

	// Retrieve values from plan
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

	hcUpdate := client.HealthCheckUpdateReq{
		HealthCheckURL:  healthCheck.Path.ValueString(),
		HealthCheckPort: int(healthCheck.Port.ValueInt64()),
	}

	_, err = r.client.UpdateClusterInstanceHealthCheckInfo(plan.Id.ValueString(), hcUpdate)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update instance healthchek endpoint.",
			err.Error(),
		)
		return
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

		_, err = r.client.WaitForDeployedEvent(ctx, topicId.String())

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

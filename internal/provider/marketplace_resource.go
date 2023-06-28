package provider

import (
	"context"
	"fmt"
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"terraform-provider-spheron/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
	Region            types.String `tfsdk:"region"`
	Name              types.String `tfsdk:"name"`
	MachineImage      types.String `tfsdk:"machine_image"`
	Ports             types.List   `tfsdk:"ports"`
	Env               types.Set    `tfsdk:"env"`
	Id                types.String `tfsdk:"id"`
	Cpu               types.String `tfsdk:"cpu"`
	Memory            types.String `tfsdk:"memory"`
	Storage           types.Int64  `tfsdk:"storage"`
	Replicas          types.Int64  `tfsdk:"replicas"`
	PersistentStorage types.Object `tfsdk:"persistent_storage"`
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
					stringplanmodifier.RequiresReplace(),
				},
			},
			"machine_image": schema.StringAttribute{
				MarkdownDescription: "Machine image name which should be used for deploying instance.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("memory")),
					stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("cpu")),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the marketplace app.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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

	var customSpecs = client.CustomInstanceSpecs{
		Storage: fmt.Sprintf("%dGi", int(plan.Storage.ValueInt64())),
	}

	if !plan.PersistentStorage.IsNull() {
		var persistentStorage PersistentStorage
		opts := basetypes.ObjectAsOptions{}

		plan.PersistentStorage.As(ctx, &persistentStorage, opts)

		value, _ := GetPersistentStorageClassEnum(persistentStorage.Class.ValueString())

		customSpecs.PersistentStorage = client.PersistentStorage{
			Class:      value,
			MountPoint: persistentStorage.MountPoint.ValueString(),
			Size:       fmt.Sprintf("%dGi", int(persistentStorage.Size.ValueInt64())),
		}
	}

	instanceConfig := client.CreateInstanceFromMarketplaceRequest{
		TemplateID:           chosenMarketplaceApp.ID,
		EnvironmentVariables: deploymentEnv,
		OrganizationID:       organization.ID,
		UniqueTopicID:        topicId.String(),
		Region:               plan.Region.ValueString(),
		InstanceCount:        int(plan.Replicas.ValueInt64()),
	}

	if !plan.Cpu.IsNull() && !plan.Memory.IsNull() {
		customSpecs.CPU = plan.Cpu.ValueString()
		customSpecs.Memory = fmt.Sprintf("%sGi", plan.Memory.ValueString())

		plan.MachineImage = types.StringValue("Custom Plan")
	} else {

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

		instanceConfig.AkashImageID = chosenMachineID
	}

	instanceConfig.CustomInstanceSpecs = customSpecs

	response, err := r.client.CreateClusterInstanceFromTemplate(instanceConfig)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy instance from marketplace app.",
			err.Error(),
		)
		return
	}

	eventDataString, err := r.client.WaitForDeployedEvent(topicId.String())

	if err != nil {
		resp.Diagnostics.AddError(
			"Marketplace instance deployment failed.",
			fmt.Sprintf("Marketplace instance deployment on cluster %s failed.", plan.Name.ValueString()),
		)
		return
	}

	ports, err := ParseClientPorts(eventDataString)
	if err != nil {
		resp.Diagnostics.AddError(
			"Marketplace instance deployment failed.",
			fmt.Sprintf("Marketplace instance deployment on cluster %s failed.", plan.Name.ValueString()),
		)
		return
	}

	plan.Id = types.StringValue(response.ClusterInstanceID)
	plan.Ports = types.ListValueMust(types.ObjectType{AttrTypes: getPortAtrTypes()}, mapModelPortToPortValue(ports))

	if plan.Cpu.IsNull() && plan.Memory.IsNull() {
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

	if instance.State == "Closed" {
		resp.State.RemoveResource(ctx)
		resp.Diagnostics.AddWarning("Markerplace app instance is closed", fmt.Sprintf("Markerplace app instance %s is closed. Applying will redeploy new markerplace app instance in its place.", state.Name.ValueString()))
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

	numberStr := order.ClusterInstanceConfiguration.AgreedMachineImage.Storage[:len(order.ClusterInstanceConfiguration.AgreedMachineImage.Storage)-2] // Remove the last two characters ("Gi")
	number, _ := strconv.Atoi(numberStr)
	state.Storage = types.Int64Value(int64(number))

	state.Memory = types.StringValue(RemoveGiSuffix(order.ClusterInstanceConfiguration.AgreedMachineImage.Memory))
	state.Cpu = types.StringValue(fmt.Sprint(order.ClusterInstanceConfiguration.AgreedMachineImage.Cpu))
	state.Replicas = types.Int64Value(int64(order.ClusterInstanceConfiguration.InstanceCount))
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
	var state MarketplaceInstanceResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CloseClusterInstance(state.Id.ValueString())
	if err != nil && err.Error() != "Instance already closed" {
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

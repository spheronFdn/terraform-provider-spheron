package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/spheron/terraform-provider-spheron/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &InstanceResource{}
var _ resource.ResourceWithImportState = &InstanceResource{}

func NewInstanceResource() resource.Resource {
	return &InstanceResource{}
}

// ExampleResource defines the resource implementation.
type InstanceResource struct {
	client *client.SpheronApi
}

// ExampleResourceModel describes the resource data model.
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
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Instnce resource",

		Attributes: map[string]schema.Attribute{
			"image": schema.StringAttribute{
				MarkdownDescription: "Docker image",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tag": schema.StringAttribute{
				MarkdownDescription: "Docer image tag",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cluster_name": schema.StringAttribute{
				MarkdownDescription: "Cluster name",
				Required:            true,
			},
			"ports": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"container_port": schema.Int64Attribute{
							MarkdownDescription: "Example configurable attribute",
							Required:            true,
						},
						"exposed_port": schema.Int64Attribute{
							MarkdownDescription: "Example configurable attribute",
							Optional:            true,
							Computed:            true,
						},
					},
				},
				Required: true,
			},
			"env": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "Env var key",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Env var value",
							Required:            true,
						},
					},
				},
				Optional: true,
			},
			"env_secret": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "Env var key",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Env var value",
							Required:            true,
						},
					},
				},
				Optional: true,
			},
			"commands": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"args": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Docer image tag",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"machine_image": schema.StringAttribute{
				MarkdownDescription: "Docer image tag",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"health_check": schema.ObjectAttribute{
				AttributeTypes: map[string]attr.Type{
					"path": types.StringType,
					"port": types.Int64Type,
				},
				Optional: true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Docer image tag",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *InstanceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	region := plan.Region.ValueString()
	if region == "" {
		region = "any"
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
		Region:                region,
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
		HealthCheckPort: "",
	}

	response, err := r.client.CreateClusterInstance(createRequest)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy instance",
			err.Error(),
		)
		return
	}

	deployed, err := r.client.WaitForDeployedEvent(topicId.String())

	if err != nil || !deployed {
		resp.Diagnostics.AddError(
			"Instance deployment failed",
			"Instance deployment failed",
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

	// Map response body to model
	plan.Id = types.StringValue(response.ClusterInstanceID)
	plan.Ports = mapModelPortToPort(order.ClusterInstanceConfiguration.Ports)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Save data into Terraform state
	tflog.Debug(ctx, "Created item resource", map[string]any{"success": true})
}

func (r *InstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state InstanceResourceModel
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

	if !healthCheck.Path.IsNull() && !healthCheck.Port.IsNull() {

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
	}

	topicId := uuid.New()

	updateRequest := client.UpdateInstanceRequest{
		Env:            append(mapEnvsToClientEnvs(plan.Env, false), mapEnvsToClientEnvs(plan.EnvSecret, true)...),
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

	deployed, err := r.client.WaitForDeployedEvent(topicId.String())

	if err != nil || !deployed {
		resp.Diagnostics.AddError(
			"Instance deployment failed",
			err.Error(),
		)
		return
	}

	// Save updated data into Terraform state
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Debug(ctx, "Updated item resource", map[string]any{"success": true})
}

func (r *InstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	tflog.Debug(ctx, "Preparing to delete item resource")
	// Retrieve values from state
	var state InstanceResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CloseClusterInstance(state.Id.ValueString())
	if err != nil {
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

func mapPortToPortModel(portList []Port) []client.Port {
	ports := make([]client.Port, len(portList))
	for i, pm := range portList {
		exposedPort := int(pm.ContainerPort.ValueInt64())
		if pm.ExposedPort.ValueInt64() != 0 {
			exposedPort = int(pm.ExposedPort.ValueInt64())
		}

		ports[i] = client.Port{
			ContainerPort: int(pm.ContainerPort.ValueInt64()),
			ExposedPort:   exposedPort,
		}
	}
	return ports
}

func mapModelPortToPort(portList []client.Port) []Port {
	ports := make([]Port, len(portList))
	for i, pm := range portList {
		ports[i] = Port{
			ContainerPort: types.Int64Value(int64(pm.ContainerPort)),
			ExposedPort:   types.Int64Value(int64(pm.ExposedPort)),
		}
	}
	return ports
}

func mapEnvsToClientEnvs(envList []Env, isSecret bool) []client.Env {
	clientEnvs := make([]client.Env, 0, len(envList))
	for i, env := range envList {
		clientEnvs[i] = client.Env{
			Value:    env.Key.ValueString() + "=" + env.Value.ValueString(),
			IsSecret: isSecret,
		}
	}
	return clientEnvs
}

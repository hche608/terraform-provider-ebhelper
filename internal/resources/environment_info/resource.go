package environment_info

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hche608/terraform-provider-ebhelper/internal/awsclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource              = &Resource{}
	_ resource.ResourceWithConfigure = &Resource{}
)

// Resource defines the environment_info resource implementation.
type Resource struct {
	clients *awsclient.Clients
}

// NewResource returns a new resource instance.
func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_info"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Discovers AWS Elastic Beanstalk environment infrastructure (ASG, load balancer, target groups) during apply. Acts as a deferred data source that resolves after the EB environment is created.",
		Attributes: map[string]schema.Attribute{
			// Inputs
			"application_name": schema.StringAttribute{
				Description: "Name of the Elastic Beanstalk application.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment_name": schema.StringAttribute{
				Description: "Name of the Elastic Beanstalk environment.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"polling_interval": schema.Int64Attribute{
				Description: "Seconds between discovery retry attempts. Default: 10.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(10),
			},
			"polling_timeout": schema.Int64Attribute{
				Description: "Maximum seconds to wait for resource discovery. Default: 300.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(300),
			},

			// Computed - Identity
			"id": schema.StringAttribute{
				Description: "Resource identifier (environment ID).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			// Computed - EB Metadata
			"environment_id": schema.StringAttribute{
				Description: "Elastic Beanstalk environment ID (e-xxxxxxxxxx).",
				Computed:    true,
			},
			"endpoint_url": schema.StringAttribute{
				Description: "Environment endpoint URL (load balancer DNS).",
				Computed:    true,
			},
			"platform_arn": schema.StringAttribute{
				Description: "Platform ARN the environment is running on.",
				Computed:    true,
			},
			"health_status": schema.StringAttribute{
				Description: "Current health status of the environment.",
				Computed:    true,
			},
			"cname": schema.StringAttribute{
				Description: "CNAME of the environment.",
				Computed:    true,
			},

			// Computed - ASG
			"asg_name": schema.StringAttribute{
				Description: "Name of the active Auto Scaling Group.",
				Computed:    true,
			},
			"asg_arn": schema.StringAttribute{
				Description: "ARN of the active Auto Scaling Group.",
				Computed:    true,
			},
			"all_asg_names": schema.ListAttribute{
				Description: "Names of all ASGs associated with the environment (for debugging during immutable deployments).",
				Computed:    true,
				ElementType: types.StringType,
			},

			// Computed - Load Balancer
			"load_balancer_arns": schema.ListAttribute{
				Description: "ARNs of load balancers associated with the environment (de-duplicated).",
				Computed:    true,
				ElementType: types.StringType,
			},
			"load_balancer_dns_names": schema.ListAttribute{
				Description: "DNS names of load balancers associated with the environment.",
				Computed:    true,
				ElementType: types.StringType,
			},

			// Computed - Target Groups
			"target_group_arns": schema.ListAttribute{
				Description: "ARNs of target groups attached to the ASG.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"target_group_names": schema.ListAttribute{
				Description: "Names of target groups attached to the ASG.",
				Computed:    true,
				ElementType: types.StringType,
			},

			// Computed - Instances
			"instance_ids": schema.ListAttribute{
				Description: "IDs of EC2 instances in the environment.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"launch_template_id": schema.StringAttribute{
				Description: "ID of the launch template used by the ASG.",
				Computed:    true,
			},
		},
	}
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clients, ok := req.ProviderData.(*awsclient.Clients)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *awsclient.Clients, got unexpected type.",
		)
		return
	}

	r.clients = clients
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentInfoModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	interval := time.Duration(plan.PollingInterval.ValueInt64()) * time.Second
	timeout := time.Duration(plan.PollingTimeout.ValueInt64()) * time.Second

	result, err := Discover(ctx, r.clients,
		plan.ApplicationName.ValueString(),
		plan.EnvironmentName.ValueString(),
		interval, timeout,
	)
	if err != nil {
		resp.Diagnostics.AddError("Environment discovery failed", err.Error())
		return
	}

	// Map discovery result to state
	state := mapResultToState(ctx, plan, result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentInfoModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Re-run discovery without polling (single attempt with short timeout)
	result, err := Discover(ctx, r.clients,
		state.ApplicationName.ValueString(),
		state.EnvironmentName.ValueString(),
		10*time.Second, 30*time.Second,
	)
	if err != nil {
		// If environment or ASG no longer exists, remove from state
		resp.State.RemoveResource(ctx)
		return
	}

	// Update state with current values
	newState := mapResultToState(ctx, state, result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EnvironmentInfoModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Re-run discovery with new polling settings
	interval := time.Duration(plan.PollingInterval.ValueInt64()) * time.Second
	timeout := time.Duration(plan.PollingTimeout.ValueInt64()) * time.Second

	result, err := Discover(ctx, r.clients,
		plan.ApplicationName.ValueString(),
		plan.EnvironmentName.ValueString(),
		interval, timeout,
	)
	if err != nil {
		resp.Diagnostics.AddError("Environment discovery failed", err.Error())
		return
	}

	state := mapResultToState(ctx, plan, result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// No-op: this resource doesn't create infrastructure, just remove from state.
	// Terraform automatically removes the resource from state when Delete returns without error.
	_ = resp // ensure coverage
}

func mapResultToState(ctx context.Context, plan EnvironmentInfoModel, result *DiscoveryResult) EnvironmentInfoModel {
	state := EnvironmentInfoModel{
		ApplicationName: plan.ApplicationName,
		EnvironmentName: plan.EnvironmentName,
		PollingInterval: plan.PollingInterval,
		PollingTimeout:  plan.PollingTimeout,

		ID:            types.StringValue(result.EnvironmentID),
		EnvironmentID: types.StringValue(result.EnvironmentID),
		EndpointURL:   types.StringValue(result.EndpointURL),
		PlatformARN:   types.StringValue(result.PlatformARN),
		HealthStatus:  types.StringValue(result.HealthStatus),
		CNAME:         types.StringValue(result.CNAME),

		ASGName:          types.StringValue(result.ASGName),
		ASGARN:           types.StringValue(result.ASGARN),
		LaunchTemplateID: types.StringValue(result.LaunchTemplateID),
	}

	// Convert string slices to types.List
	state.AllASGNames, _ = types.ListValueFrom(ctx, types.StringType, result.AllASGNames)
	state.LoadBalancerARNs, _ = types.ListValueFrom(ctx, types.StringType, result.LoadBalancerARNs)
	state.LoadBalancerDNS, _ = types.ListValueFrom(ctx, types.StringType, result.LoadBalancerDNS)
	state.TargetGroupARNs, _ = types.ListValueFrom(ctx, types.StringType, result.TargetGroupARNs)
	state.TargetGroupNames, _ = types.ListValueFrom(ctx, types.StringType, result.TargetGroupNames)
	state.InstanceIDs, _ = types.ListValueFrom(ctx, types.StringType, result.InstanceIDs)

	return state
}

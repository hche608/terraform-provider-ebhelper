// Package asg_health_check implements the ebhelper_asg_health_check resource
// which configures health check settings on EB-managed Auto Scaling Groups.
package asg_health_check

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hche608/terraform-provider-ebhelper/internal/awsclient"
)

var (
	_ resource.Resource              = &Resource{}
	_ resource.ResourceWithConfigure = &Resource{}
)

type Resource struct {
	clients *awsclient.Clients
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asg_health_check"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configures the health check type and grace period on an EB-managed Auto Scaling Group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource identifier (ASG name).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"asg_name": schema.StringAttribute{
				Description: "Name of the Auto Scaling Group to configure.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"health_check_type": schema.StringAttribute{
				Description: "Health check type: EC2 or ELB.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("EC2", "ELB"),
				},
			},
			"health_check_grace_period": schema.Int64Attribute{
				Description: "Health check grace period in seconds. Default: 300.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(300),
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
	var plan ASGHealthCheckModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := plan.ASGName.ValueString()
	healthCheckType := plan.HealthCheckType.ValueString()
	gracePeriod := int32(plan.HealthCheckGracePeriod.ValueInt64())

	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName:   aws.String(asgName),
		HealthCheckType:        aws.String(healthCheckType),
		HealthCheckGracePeriod: aws.Int32(gracePeriod),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to update ASG health check",
			fmt.Sprintf("[ebhelper_asg_health_check] create failed: ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(asgName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ASGHealthCheckModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := state.ASGName.ValueString()

	output, err := r.clients.AutoScaling.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{asgName},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read ASG",
			fmt.Sprintf("[ebhelper_asg_health_check] read failed for ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	if len(output.AutoScalingGroups) == 0 {
		// ASG no longer exists, remove from state
		resp.State.RemoveResource(ctx)
		return
	}

	asg := output.AutoScalingGroups[0]

	// Update state with current values (drift detection)
	state.HealthCheckType = types.StringValue(aws.ToString(asg.HealthCheckType))
	state.HealthCheckGracePeriod = types.Int64Value(int64(aws.ToInt32(asg.HealthCheckGracePeriod)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ASGHealthCheckModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := plan.ASGName.ValueString()
	healthCheckType := plan.HealthCheckType.ValueString()
	gracePeriod := int32(plan.HealthCheckGracePeriod.ValueInt64())

	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName:   aws.String(asgName),
		HealthCheckType:        aws.String(healthCheckType),
		HealthCheckGracePeriod: aws.Int32(gracePeriod),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to update ASG health check",
			fmt.Sprintf("[ebhelper_asg_health_check] update failed for ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(asgName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ASGHealthCheckModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := state.ASGName.ValueString()

	// Reset to EC2 health check with default grace period
	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName:   aws.String(asgName),
		HealthCheckType:        aws.String("EC2"),
		HealthCheckGracePeriod: aws.Int32(300),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to reset ASG health check",
			fmt.Sprintf("[ebhelper_asg_health_check] delete failed for ASG %q: %s", asgName, err.Error()),
		)
	}
}

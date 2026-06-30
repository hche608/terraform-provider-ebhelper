// Package asg_max_instance_lifetime implements the ebhelper_asg_max_instance_lifetime resource
// which configures the maximum instance lifetime on EB-managed Auto Scaling Groups.
package asg_max_instance_lifetime

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	resp.TypeName = req.ProviderTypeName + "_asg_max_instance_lifetime"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configures the maximum instance lifetime on an EB-managed Auto Scaling Group.",
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
			"max_instance_lifetime_seconds": schema.Int64Attribute{
				Description: "Maximum instance lifetime in seconds. Must be 0 (disabled) or between 86400 and 31536000.",
				Required:    true,
				Validators: []validator.Int64{
					int64validator.Any(
						int64validator.OneOf(0),
						int64validator.Between(86400, 31536000),
					),
				},
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
	var plan ASGMaxInstanceLifetimeModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := plan.ASGName.ValueString()
	maxLifetime := int32(plan.MaxInstanceLifetimeSeconds.ValueInt64())

	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgName),
		MaxInstanceLifetime:  aws.Int32(maxLifetime),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to update ASG max instance lifetime",
			fmt.Sprintf("[ebhelper_asg_max_instance_lifetime] create failed: ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(asgName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ASGMaxInstanceLifetimeModel

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
			fmt.Sprintf("[ebhelper_asg_max_instance_lifetime] read failed for ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	if len(output.AutoScalingGroups) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	asg := output.AutoScalingGroups[0]

	state.MaxInstanceLifetimeSeconds = types.Int64Value(int64(aws.ToInt32(asg.MaxInstanceLifetime)))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ASGMaxInstanceLifetimeModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := plan.ASGName.ValueString()
	maxLifetime := int32(plan.MaxInstanceLifetimeSeconds.ValueInt64())

	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgName),
		MaxInstanceLifetime:  aws.Int32(maxLifetime),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to update ASG max instance lifetime",
			fmt.Sprintf("[ebhelper_asg_max_instance_lifetime] update failed for ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(asgName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ASGMaxInstanceLifetimeModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := state.ASGName.ValueString()

	// Reset to 0 (disabled)
	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgName),
		MaxInstanceLifetime:  aws.Int32(0),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to reset ASG max instance lifetime",
			fmt.Sprintf("[ebhelper_asg_max_instance_lifetime] delete failed for ASG %q: %s", asgName, err.Error()),
		)
	}
}

// Package asg_instance_maintenance_policy implements the
// ebhelper_asg_instance_maintenance_policy resource which configures instance
// maintenance policies on EB-managed Auto Scaling Groups.
package asg_instance_maintenance_policy

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
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
	resp.TypeName = req.ProviderTypeName + "_asg_instance_maintenance_policy"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configures the instance maintenance policy on an EB-managed Auto Scaling Group, controlling instance replacement behavior during updates.",
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
			"min_healthy_percentage": schema.Int64Attribute{
				Description: "Minimum percentage of healthy instances during replacements (0-100).",
				Required:    true,
				Validators: []validator.Int64{
					int64validator.Between(0, 100),
				},
			},
			"max_healthy_percentage": schema.Int64Attribute{
				Description: "Maximum percentage of healthy instances during replacements (100-200).",
				Required:    true,
				Validators: []validator.Int64{
					int64validator.Between(100, 200),
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
	var plan ASGMaintenancePolicyModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := plan.ASGName.ValueString()
	minHealthy := int32(plan.MinHealthyPercentage.ValueInt64())
	maxHealthy := int32(plan.MaxHealthyPercentage.ValueInt64())

	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgName),
		InstanceMaintenancePolicy: &asgtypes.InstanceMaintenancePolicy{
			MinHealthyPercentage: aws.Int32(minHealthy),
			MaxHealthyPercentage: aws.Int32(maxHealthy),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to set instance maintenance policy",
			fmt.Sprintf("[ebhelper_asg_instance_maintenance_policy] create failed for ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(asgName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ASGMaintenancePolicyModel

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
			fmt.Sprintf("[ebhelper_asg_instance_maintenance_policy] read failed for ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	if len(output.AutoScalingGroups) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	asg := output.AutoScalingGroups[0]

	if asg.InstanceMaintenancePolicy != nil {
		state.MinHealthyPercentage = types.Int64Value(int64(aws.ToInt32(asg.InstanceMaintenancePolicy.MinHealthyPercentage)))
		state.MaxHealthyPercentage = types.Int64Value(int64(aws.ToInt32(asg.InstanceMaintenancePolicy.MaxHealthyPercentage)))
	} else {
		// Policy was removed externally
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ASGMaintenancePolicyModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := plan.ASGName.ValueString()
	minHealthy := int32(plan.MinHealthyPercentage.ValueInt64())
	maxHealthy := int32(plan.MaxHealthyPercentage.ValueInt64())

	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgName),
		InstanceMaintenancePolicy: &asgtypes.InstanceMaintenancePolicy{
			MinHealthyPercentage: aws.Int32(minHealthy),
			MaxHealthyPercentage: aws.Int32(maxHealthy),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to update instance maintenance policy",
			fmt.Sprintf("[ebhelper_asg_instance_maintenance_policy] update failed for ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(asgName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ASGMaintenancePolicyModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := state.ASGName.ValueString()

	// Remove policy by setting -1 values
	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgName),
		InstanceMaintenancePolicy: &asgtypes.InstanceMaintenancePolicy{
			MinHealthyPercentage: aws.Int32(-1),
			MaxHealthyPercentage: aws.Int32(-1),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to remove instance maintenance policy",
			fmt.Sprintf("[ebhelper_asg_instance_maintenance_policy] delete failed for ASG %q: %s", asgName, err.Error()),
		)
	}
}

// Package asg_termination_policy implements the ebhelper_asg_termination_policy resource
// which configures termination policies on EB-managed Auto Scaling Groups.
package asg_termination_policy

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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
	resp.TypeName = req.ProviderTypeName + "_asg_termination_policy"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configures the termination policies on an EB-managed Auto Scaling Group.",
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
			"termination_policies": schema.ListAttribute{
				Description: "Ordered list of termination policies for the ASG.",
				Required:    true,
				ElementType: types.StringType,
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
	var plan ASGTerminationPolicyModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := plan.ASGName.ValueString()

	var policies []string
	resp.Diagnostics.Append(plan.TerminationPolicies.ElementsAs(ctx, &policies, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgName),
		TerminationPolicies:  policies,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to update ASG termination policies",
			fmt.Sprintf("[ebhelper_asg_termination_policy] create failed: ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(asgName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ASGTerminationPolicyModel

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
			fmt.Sprintf("[ebhelper_asg_termination_policy] read failed for ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	if len(output.AutoScalingGroups) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	asg := output.AutoScalingGroups[0]

	policies, diags := types.ListValueFrom(ctx, types.StringType, asg.TerminationPolicies)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.TerminationPolicies = policies

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ASGTerminationPolicyModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := plan.ASGName.ValueString()

	var policies []string
	resp.Diagnostics.Append(plan.TerminationPolicies.ElementsAs(ctx, &policies, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgName),
		TerminationPolicies:  policies,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to update ASG termination policies",
			fmt.Sprintf("[ebhelper_asg_termination_policy] update failed for ASG %q: %s", asgName, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(asgName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ASGTerminationPolicyModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	asgName := state.ASGName.ValueString()

	// Reset to default termination policy
	_, err := r.clients.AutoScaling.UpdateAutoScalingGroup(ctx, &autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgName),
		TerminationPolicies:  []string{"Default"},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to reset ASG termination policies",
			fmt.Sprintf("[ebhelper_asg_termination_policy] delete failed for ASG %q: %s", asgName, err.Error()),
		)
	}
}

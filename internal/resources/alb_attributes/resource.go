// Package alb_attributes implements the ebhelper_alb_attributes resource
// which configures attributes on an ALB managed by Elastic Beanstalk.
package alb_attributes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
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
	resp.TypeName = req.ProviderTypeName + "_alb_attributes"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configures attributes on an ALB managed by Elastic Beanstalk.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Resource identifier (load balancer ARN).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"load_balancer_arn": schema.StringAttribute{
				Description: "ARN of the Application Load Balancer to configure.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"attributes": schema.MapAttribute{
				Description: "Map of ALB attribute key-value pairs to set.",
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
	var plan ALBAttributesModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	lbARN := plan.LoadBalancerARN.ValueString()

	var attrs map[string]string
	resp.Diagnostics.Append(plan.Attributes.ElementsAs(ctx, &attrs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	elbAttrs := make([]elbv2types.LoadBalancerAttribute, 0, len(attrs))
	for k, v := range attrs {
		elbAttrs = append(elbAttrs, elbv2types.LoadBalancerAttribute{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	_, err := r.clients.ELBv2.ModifyLoadBalancerAttributes(ctx, &elasticloadbalancingv2.ModifyLoadBalancerAttributesInput{
		LoadBalancerArn: aws.String(lbARN),
		Attributes:      elbAttrs,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to modify ALB attributes",
			fmt.Sprintf("[ebhelper_alb_attributes] create failed: ALB %q: %s", lbARN, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(lbARN)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ALBAttributesModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	lbARN := state.LoadBalancerARN.ValueString()

	// Get the configured attribute keys from state so we only track those
	var configuredAttrs map[string]string
	resp.Diagnostics.Append(state.Attributes.ElementsAs(ctx, &configuredAttrs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	output, err := r.clients.ELBv2.DescribeLoadBalancerAttributes(ctx, &elasticloadbalancingv2.DescribeLoadBalancerAttributesInput{
		LoadBalancerArn: aws.String(lbARN),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to read ALB attributes",
			fmt.Sprintf("[ebhelper_alb_attributes] read failed for ALB %q: %s", lbARN, err.Error()),
		)
		return
	}

	// Only track attributes that are in the config (ignore AWS defaults)
	tracked := make(map[string]string)
	for _, attr := range output.Attributes {
		key := aws.ToString(attr.Key)
		if _, exists := configuredAttrs[key]; exists {
			tracked[key] = aws.ToString(attr.Value)
		}
	}

	attrsValue, diags := types.MapValueFrom(ctx, types.StringType, tracked)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Attributes = attrsValue

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ALBAttributesModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	lbARN := plan.LoadBalancerARN.ValueString()

	var attrs map[string]string
	resp.Diagnostics.Append(plan.Attributes.ElementsAs(ctx, &attrs, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	elbAttrs := make([]elbv2types.LoadBalancerAttribute, 0, len(attrs))
	for k, v := range attrs {
		elbAttrs = append(elbAttrs, elbv2types.LoadBalancerAttribute{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}

	_, err := r.clients.ELBv2.ModifyLoadBalancerAttributes(ctx, &elasticloadbalancingv2.ModifyLoadBalancerAttributesInput{
		LoadBalancerArn: aws.String(lbARN),
		Attributes:      elbAttrs,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to modify ALB attributes",
			fmt.Sprintf("[ebhelper_alb_attributes] update failed for ALB %q: %s", lbARN, err.Error()),
		)
		return
	}

	plan.ID = types.StringValue(lbARN)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// No-op: ALB attributes can't be "unset" to a known default generically.
	// Simply remove from state.
}

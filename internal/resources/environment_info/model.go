package environment_info

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// EnvironmentInfoModel describes the resource data model.
type EnvironmentInfoModel struct {
	// Inputs (Required)
	ApplicationName types.String `tfsdk:"application_name"`
	EnvironmentName types.String `tfsdk:"environment_name"`

	// Inputs (Optional with defaults)
	PollingInterval types.Int64 `tfsdk:"polling_interval"`
	PollingTimeout  types.Int64 `tfsdk:"polling_timeout"`

	// Computed - Identity
	ID types.String `tfsdk:"id"`

	// Computed - EB Metadata
	EnvironmentID types.String `tfsdk:"environment_id"`
	EndpointURL   types.String `tfsdk:"endpoint_url"`
	PlatformARN   types.String `tfsdk:"platform_arn"`
	HealthStatus  types.String `tfsdk:"health_status"`
	CNAME         types.String `tfsdk:"cname"`

	// Computed - ASG
	ASGName     types.String `tfsdk:"asg_name"`
	ASGARN      types.String `tfsdk:"asg_arn"`
	AllASGNames types.List   `tfsdk:"all_asg_names"`

	// Computed - Load Balancer
	LoadBalancerARNs types.List `tfsdk:"load_balancer_arns"`
	LoadBalancerDNS  types.List `tfsdk:"load_balancer_dns_names"`

	// Computed - Target Groups
	TargetGroupARNs  types.List `tfsdk:"target_group_arns"`
	TargetGroupNames types.List `tfsdk:"target_group_names"`

	// Computed - Instances
	InstanceIDs      types.List   `tfsdk:"instance_ids"`
	LaunchTemplateID types.String `tfsdk:"launch_template_id"`
}

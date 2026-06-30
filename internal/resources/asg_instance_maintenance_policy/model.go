package asg_instance_maintenance_policy

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ASGMaintenancePolicyModel describes the resource data model.
type ASGMaintenancePolicyModel struct {
	ID                   types.String `tfsdk:"id"`
	ASGName              types.String `tfsdk:"asg_name"`
	MinHealthyPercentage types.Int64  `tfsdk:"min_healthy_percentage"`
	MaxHealthyPercentage types.Int64  `tfsdk:"max_healthy_percentage"`
}

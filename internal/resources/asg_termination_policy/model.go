package asg_termination_policy

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ASGTerminationPolicyModel describes the resource data model.
type ASGTerminationPolicyModel struct {
	ID                  types.String `tfsdk:"id"`
	ASGName             types.String `tfsdk:"asg_name"`
	TerminationPolicies types.List   `tfsdk:"termination_policies"`
}

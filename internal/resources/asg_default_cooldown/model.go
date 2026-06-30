package asg_default_cooldown

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ASGDefaultCooldownModel describes the resource data model.
type ASGDefaultCooldownModel struct {
	ID              types.String `tfsdk:"id"`
	ASGName         types.String `tfsdk:"asg_name"`
	CooldownSeconds types.Int64  `tfsdk:"cooldown_seconds"`
}

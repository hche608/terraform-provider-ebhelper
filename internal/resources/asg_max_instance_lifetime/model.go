package asg_max_instance_lifetime

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ASGMaxInstanceLifetimeModel describes the resource data model.
type ASGMaxInstanceLifetimeModel struct {
	ID                         types.String `tfsdk:"id"`
	ASGName                    types.String `tfsdk:"asg_name"`
	MaxInstanceLifetimeSeconds types.Int64  `tfsdk:"max_instance_lifetime_seconds"`
}

package asg_default_instance_warmup

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ASGDefaultInstanceWarmupModel describes the resource data model.
type ASGDefaultInstanceWarmupModel struct {
	ID            types.String `tfsdk:"id"`
	ASGName       types.String `tfsdk:"asg_name"`
	WarmupSeconds types.Int64  `tfsdk:"warmup_seconds"`
}

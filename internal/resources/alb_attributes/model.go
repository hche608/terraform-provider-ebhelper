package alb_attributes

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ALBAttributesModel describes the resource data model.
type ALBAttributesModel struct {
	ID              types.String `tfsdk:"id"`
	LoadBalancerARN types.String `tfsdk:"load_balancer_arn"`
	Attributes      types.Map    `tfsdk:"attributes"`
}

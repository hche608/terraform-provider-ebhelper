package asg_health_check

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ASGHealthCheckModel describes the resource data model.
type ASGHealthCheckModel struct {
	ID                     types.String `tfsdk:"id"`
	ASGName                types.String `tfsdk:"asg_name"`
	HealthCheckType        types.String `tfsdk:"health_check_type"`
	HealthCheckGracePeriod types.Int64  `tfsdk:"health_check_grace_period"`
}

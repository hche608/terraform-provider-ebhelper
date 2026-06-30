// Package provider implements the Terraform provider for ebhelper.
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hche608/terraform-provider-ebhelper/internal/awsclient"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/alb_attributes"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/asg_default_cooldown"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/asg_default_instance_warmup"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/asg_health_check"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/asg_instance_maintenance_policy"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/asg_max_instance_lifetime"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/asg_termination_policy"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/environment_info"
)

// Ensure EbhelperProvider satisfies various provider interfaces.
var _ provider.Provider = &EbhelperProvider{}

// EbhelperProvider defines the provider implementation.
type EbhelperProvider struct {
	version string
}

// EbhelperProviderModel describes the provider data model.
type EbhelperProviderModel struct {
	Region     types.String     `tfsdk:"region"`
	AssumeRole *AssumeRoleModel `tfsdk:"assume_role"`
}

// AssumeRoleModel describes the assume_role block.
type AssumeRoleModel struct {
	RoleARN     types.String `tfsdk:"role_arn"`
	SessionName types.String `tfsdk:"session_name"`
	ExternalID  types.String `tfsdk:"external_id"`
}

func (p *EbhelperProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ebhelper"
	resp.Version = p.version
}

func (p *EbhelperProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The ebhelper provider discovers AWS Elastic Beanstalk environment infrastructure and manages ASG settings that EB does not expose through Terraform.",
		Attributes: map[string]schema.Attribute{
			"region": schema.StringAttribute{
				Description: "AWS region for API calls. Uses the default AWS credential chain if not specified.",
				Optional:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"assume_role": schema.SingleNestedBlock{
				Description: "Configuration for assuming an IAM role for cross-account access.",
				Attributes: map[string]schema.Attribute{
					"role_arn": schema.StringAttribute{
						Description: "ARN of the IAM role to assume.",
						Optional:    true,
					},
					"session_name": schema.StringAttribute{
						Description: "Session name for the assumed role. Defaults to 'terraform-ebhelper'.",
						Optional:    true,
					},
					"external_id": schema.StringAttribute{
						Description: "External ID for the assumed role (used for third-party access).",
						Optional:    true,
					},
				},
			},
		},
	}
}

func (p *EbhelperProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config EbhelperProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build AWS client config
	clientCfg := awsclient.Config{}

	if !config.Region.IsNull() && !config.Region.IsUnknown() {
		clientCfg.Region = config.Region.ValueString()
	}

	if config.AssumeRole != nil {
		if !config.AssumeRole.RoleARN.IsNull() && !config.AssumeRole.RoleARN.IsUnknown() && config.AssumeRole.RoleARN.ValueString() != "" {
			clientCfg.RoleARN = config.AssumeRole.RoleARN.ValueString()

			if !config.AssumeRole.SessionName.IsNull() && !config.AssumeRole.SessionName.IsUnknown() {
				clientCfg.SessionName = config.AssumeRole.SessionName.ValueString()
			}
			if !config.AssumeRole.ExternalID.IsNull() && !config.AssumeRole.ExternalID.IsUnknown() {
				clientCfg.ExternalID = config.AssumeRole.ExternalID.ValueString()
			}
		}
	}

	clients, err := awsclient.NewClients(ctx, clientCfg)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to configure AWS clients",
			err.Error(),
		)
		return
	}

	resp.DataSourceData = clients
	resp.ResourceData = clients
}

func (p *EbhelperProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		environment_info.NewResource,
		asg_health_check.NewResource,
		asg_instance_maintenance_policy.NewResource,
		asg_termination_policy.NewResource,
		asg_default_cooldown.NewResource,
		asg_max_instance_lifetime.NewResource,
		asg_default_instance_warmup.NewResource,
		alb_attributes.NewResource,
	}
}

func (p *EbhelperProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// New returns a provider.Provider implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &EbhelperProvider{
			version: version,
		}
	}
}

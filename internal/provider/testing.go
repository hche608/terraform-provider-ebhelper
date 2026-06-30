package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hche608/terraform-provider-ebhelper/internal/awsclient"

	"github.com/hche608/terraform-provider-ebhelper/internal/resources/asg_health_check"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/asg_instance_maintenance_policy"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/environment_info"
)

// TestProvider is a provider implementation for unit testing that accepts
// pre-configured AWS clients (mocks) instead of building real ones.
type TestProvider struct {
	Clients *awsclient.Clients
}

var _ provider.Provider = &TestProvider{}

func (p *TestProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ebhelper"
	resp.Version = "test"
}

func (p *TestProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{},
	}
}

func (p *TestProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	resp.DataSourceData = p.Clients
	resp.ResourceData = p.Clients
}

func (p *TestProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		environment_info.NewResource,
		asg_health_check.NewResource,
		asg_instance_maintenance_policy.NewResource,
	}
}

func (p *TestProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// NewTestProvider creates a provider for unit tests with injected mock clients.
func NewTestProvider(clients *awsclient.Clients) *TestProvider {
	return &TestProvider{Clients: clients}
}

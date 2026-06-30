package environment_info

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchema_HasAllExpectedAttributes(t *testing.T) {
	r := NewResource()
	schemaResp := &resource.SchemaResponse{}
	r.(*Resource).Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	s := schemaResp.Schema

	expectedAttrs := []string{
		"application_name",
		"environment_name",
		"polling_interval",
		"polling_timeout",
		"id",
		"environment_id",
		"endpoint_url",
		"platform_arn",
		"health_status",
		"cname",
		"asg_name",
		"asg_arn",
		"all_asg_names",
		"load_balancer_arns",
		"load_balancer_dns_names",
		"target_group_arns",
		"target_group_names",
		"instance_ids",
		"launch_template_id",
	}

	for _, attr := range expectedAttrs {
		_, ok := s.Attributes[attr]
		assert.True(t, ok, "expected attribute %q not found in schema", attr)
	}
}

func TestSchema_PlanModifiers_RequiresReplace(t *testing.T) {
	r := NewResource()
	schemaResp := &resource.SchemaResponse{}
	r.(*Resource).Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	s := schemaResp.Schema

	// application_name should require replace
	appNameAttr, ok := s.Attributes["application_name"]
	require.True(t, ok)
	strAttr, ok := appNameAttr.(schema.StringAttribute)
	require.True(t, ok)
	assert.NotEmpty(t, strAttr.PlanModifiers, "application_name should have plan modifiers")

	// environment_name should require replace
	envNameAttr, ok := s.Attributes["environment_name"]
	require.True(t, ok)
	strAttr, ok = envNameAttr.(schema.StringAttribute)
	require.True(t, ok)
	assert.NotEmpty(t, strAttr.PlanModifiers, "environment_name should have plan modifiers")
}

func TestMapResultToState_MapsAllFields(t *testing.T) {
	ctx := context.Background()
	plan := EnvironmentInfoModel{
		ApplicationName: types.StringValue("my-app"),
		EnvironmentName: types.StringValue("my-env"),
		PollingInterval: types.Int64Value(10),
		PollingTimeout:  types.Int64Value(300),
	}

	result := &DiscoveryResult{
		EnvironmentID:    "e-test123",
		EndpointURL:      "internal-lb.ap-southeast-2.elb.amazonaws.com",
		PlatformARN:      "arn:aws:elasticbeanstalk:ap-southeast-2::platform/Corretto 21/4.8.1",
		HealthStatus:     "Ok",
		CNAME:            "my-env.ap-southeast-2.elasticbeanstalk.com",
		ASGName:          "awseb-e-test123-stack-AWSEBAutoScalingGroup-ABC",
		ASGARN:           "arn:aws:autoscaling:ap-southeast-2:123456789012:autoScalingGroup:id:autoScalingGroupName/asg",
		AllASGNames:      []string{"awseb-e-test123-stack-AWSEBAutoScalingGroup-ABC"},
		TargetGroupARNs:  []string{"arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/tg1/123"},
		TargetGroupNames: []string{"tg1"},
		LoadBalancerARNs: []string{"arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:loadbalancer/app/lb/123"},
		LoadBalancerDNS:  []string{"internal-lb.ap-southeast-2.elb.amazonaws.com"},
		InstanceIDs:      []string{"i-0123456789abcdef0"},
		LaunchTemplateID: "lt-0123456789abcdef0",
	}

	state := mapResultToState(ctx, plan, result)

	assert.Equal(t, "my-app", state.ApplicationName.ValueString())
	assert.Equal(t, "my-env", state.EnvironmentName.ValueString())
	assert.Equal(t, int64(10), state.PollingInterval.ValueInt64())
	assert.Equal(t, int64(300), state.PollingTimeout.ValueInt64())
	assert.Equal(t, "e-test123", state.ID.ValueString())
	assert.Equal(t, "e-test123", state.EnvironmentID.ValueString())
	assert.Equal(t, "internal-lb.ap-southeast-2.elb.amazonaws.com", state.EndpointURL.ValueString())
	assert.Equal(t, "arn:aws:elasticbeanstalk:ap-southeast-2::platform/Corretto 21/4.8.1", state.PlatformARN.ValueString())
	assert.Equal(t, "Ok", state.HealthStatus.ValueString())
	assert.Equal(t, "my-env.ap-southeast-2.elasticbeanstalk.com", state.CNAME.ValueString())
	assert.Equal(t, "awseb-e-test123-stack-AWSEBAutoScalingGroup-ABC", state.ASGName.ValueString())
	assert.Equal(t, "lt-0123456789abcdef0", state.LaunchTemplateID.ValueString())
	assert.False(t, state.AllASGNames.IsNull())
	assert.False(t, state.TargetGroupARNs.IsNull())
	assert.False(t, state.TargetGroupNames.IsNull())
	assert.False(t, state.LoadBalancerARNs.IsNull())
	assert.False(t, state.LoadBalancerDNS.IsNull())
	assert.False(t, state.InstanceIDs.IsNull())
}

func TestMapResultToState_WithEmptySlices(t *testing.T) {
	ctx := context.Background()
	plan := EnvironmentInfoModel{
		ApplicationName: types.StringValue("my-app"),
		EnvironmentName: types.StringValue("my-env"),
		PollingInterval: types.Int64Value(10),
		PollingTimeout:  types.Int64Value(300),
	}

	result := &DiscoveryResult{
		EnvironmentID:    "e-test123",
		EndpointURL:      "",
		PlatformARN:      "",
		HealthStatus:     "",
		CNAME:            "",
		ASGName:          "asg-test",
		ASGARN:           "arn:test",
		AllASGNames:      []string{},
		TargetGroupARNs:  []string{},
		TargetGroupNames: []string{},
		LoadBalancerARNs: []string{},
		LoadBalancerDNS:  []string{},
		InstanceIDs:      []string{},
		LaunchTemplateID: "",
	}

	state := mapResultToState(ctx, plan, result)

	// Empty slices should produce empty (not null) lists
	assert.False(t, state.AllASGNames.IsNull())
	assert.False(t, state.TargetGroupARNs.IsNull())
	assert.False(t, state.TargetGroupNames.IsNull())
	assert.False(t, state.LoadBalancerARNs.IsNull())
	assert.False(t, state.LoadBalancerDNS.IsNull())
	assert.False(t, state.InstanceIDs.IsNull())

	// Verify elements count is 0
	assert.Equal(t, 0, len(state.AllASGNames.Elements()))
	assert.Equal(t, 0, len(state.TargetGroupARNs.Elements()))
}

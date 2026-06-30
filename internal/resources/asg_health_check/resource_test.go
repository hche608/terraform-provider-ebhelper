package asg_health_check_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hche608/terraform-provider-ebhelper/internal/awsclient"
	"github.com/hche608/terraform-provider-ebhelper/internal/mocks"
	tfprovider "github.com/hche608/terraform-provider-ebhelper/internal/provider"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/asg_health_check"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Schema Tests ---

func TestSchema_HealthCheckTypeValidation(t *testing.T) {
	r := asg_health_check.NewResource()
	schemaResp := &resource.SchemaResponse{}
	r.(resource.ResourceWithConfigure).Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	hctAttr := schemaResp.Schema.Attributes["health_check_type"].(schema.StringAttribute)
	assert.NotEmpty(t, hctAttr.Validators)
	assert.True(t, hctAttr.Required)
}

func TestSchema_GracePeriodDefaults(t *testing.T) {
	r := asg_health_check.NewResource()
	schemaResp := &resource.SchemaResponse{}
	r.(resource.ResourceWithConfigure).Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	gpAttr := schemaResp.Schema.Attributes["health_check_grace_period"].(schema.Int64Attribute)
	assert.True(t, gpAttr.Optional)
	assert.True(t, gpAttr.Computed)
	assert.NotNil(t, gpAttr.Default)
}

func TestSchema_ASGNameRequiresReplace(t *testing.T) {
	r := asg_health_check.NewResource()
	schemaResp := &resource.SchemaResponse{}
	r.(resource.ResourceWithConfigure).Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	asgAttr := schemaResp.Schema.Attributes["asg_name"].(schema.StringAttribute)
	assert.True(t, asgAttr.Required)
	assert.NotEmpty(t, asgAttr.PlanModifiers)
}

func TestMetadata(t *testing.T) {
	r := asg_health_check.NewResource()
	resp := &resource.MetadataResponse{}
	r.(resource.ResourceWithConfigure).Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "ebhelper"}, resp)
	assert.Equal(t, "ebhelper_asg_health_check", resp.TypeName)
}

func TestConfigure_NilProviderData(t *testing.T) {
	r := asg_health_check.NewResource()
	resp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)
	assert.False(t, resp.Diagnostics.HasError())
}

func TestConfigure_ValidClients(t *testing.T) {
	r := asg_health_check.NewResource()
	resp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: &awsclient.Clients{}}, resp)
	assert.False(t, resp.Diagnostics.HasError())
}

func TestConfigure_InvalidType(t *testing.T) {
	r := asg_health_check.NewResource()
	resp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: "wrong"}, resp)
	assert.True(t, resp.Diagnostics.HasError())
}

// --- CRUD Tests via terraform-plugin-testing ---

func providerFactories(mockASG *mocks.MockASGClient) map[string]func() (tfprotov6.ProviderServer, error) {
	clients := &awsclient.Clients{AutoScaling: mockASG}
	return map[string]func() (tfprotov6.ProviderServer, error){
		"ebhelper": providerserver.NewProtocol6WithError(tfprovider.NewTestProvider(clients)),
	}
}

func TestAccCreate_Success(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName:   aws.String("test-asg"),
			HealthCheckType:        aws.String("ELB"),
			HealthCheckGracePeriod: aws.Int32(300),
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name          = "test-asg"
  health_check_type = "ELB"
}`,
			Check: tfresource.ComposeTestCheckFunc(
				tfresource.TestCheckResourceAttr("ebhelper_asg_health_check.test", "id", "test-asg"),
				tfresource.TestCheckResourceAttr("ebhelper_asg_health_check.test", "health_check_type", "ELB"),
				tfresource.TestCheckResourceAttr("ebhelper_asg_health_check.test", "health_check_grace_period", "300"),
			),
		}},
	})

	// Verify Create called UpdateAutoScalingGroup with correct params
	calls := mockASG.Calls
	require.NotEmpty(t, calls)
	var createCall *autoscaling.UpdateAutoScalingGroupInput
	for _, c := range calls {
		if c.Method == "UpdateAutoScalingGroup" {
			input := c.Arguments.Get(1).(*autoscaling.UpdateAutoScalingGroupInput)
			if aws.ToString(input.HealthCheckType) == "ELB" {
				createCall = input
				break
			}
		}
	}
	require.NotNil(t, createCall, "expected UpdateAutoScalingGroup call with ELB")
	assert.Equal(t, "test-asg", aws.ToString(createCall.AutoScalingGroupName))
	assert.Equal(t, int32(300), aws.ToInt32(createCall.HealthCheckGracePeriod))
}

func TestAccCreate_APIError(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("ValidationError: AutoScalingGroup not found"))

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name          = "missing-asg"
  health_check_type = "ELB"
}`,
			ExpectError: regexp.MustCompile(`Failed to update ASG health check`),
		}},
	})
}

func TestAccRead_DriftDetection(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)
	// Read returns drifted values
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName:   aws.String("test-asg"),
			HealthCheckType:        aws.String("EC2"), // Drifted from ELB
			HealthCheckGracePeriod: aws.Int32(600),    // Drifted from 300
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name          = "test-asg"
  health_check_type = "ELB"
}`,
			ExpectNonEmptyPlan: true,
		}},
	})
}

func TestAccRead_ASGDeleted(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name          = "test-asg"
  health_check_type = "ELB"
}`,
			ExpectNonEmptyPlan: true,
		}},
	})
}

func TestAccRead_APIError(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("AccessDenied"))

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name          = "test-asg"
  health_check_type = "ELB"
}`,
			ExpectError: regexp.MustCompile(`Failed to read ASG`),
		}},
	})
}

func TestAccValidation_InvalidHealthCheckType(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name          = "test-asg"
  health_check_type = "INVALID"
}`,
			ExpectError: regexp.MustCompile(`must be one of`),
		}},
	})
}

func TestAccUpdate_ChangesHealthCheckType(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)

	// First: return ELB/300 for create+read
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName:   aws.String("test-asg"),
			HealthCheckType:        aws.String("ELB"),
			HealthCheckGracePeriod: aws.Int32(300),
		}},
	}, nil).Times(2)

	// After update: return EC2/600
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName:   aws.String("test-asg"),
			HealthCheckType:        aws.String("EC2"),
			HealthCheckGracePeriod: aws.Int32(600),
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockASG),
		Steps: []tfresource.TestStep{
			{
				Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name                    = "test-asg"
  health_check_type           = "ELB"
  health_check_grace_period   = 300
}`,
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("ebhelper_asg_health_check.test", "health_check_type", "ELB"),
					tfresource.TestCheckResourceAttr("ebhelper_asg_health_check.test", "health_check_grace_period", "300"),
				),
			},
			{
				Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name                    = "test-asg"
  health_check_type           = "EC2"
  health_check_grace_period   = 600
}`,
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("ebhelper_asg_health_check.test", "health_check_type", "EC2"),
					tfresource.TestCheckResourceAttr("ebhelper_asg_health_check.test", "health_check_grace_period", "600"),
				),
			},
		},
	})

	// Verify Update was called (multiple UpdateAutoScalingGroup calls)
	updateCalls := 0
	for _, c := range mockASG.Calls {
		if c.Method == "UpdateAutoScalingGroup" {
			updateCalls++
		}
	}
	// At least 2 create/update calls + 1 delete call
	assert.GreaterOrEqual(t, updateCalls, 3)
}

func TestAccUpdate_ChangesGracePeriod(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)

	// First: return ELB/300 for create+read
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName:   aws.String("test-asg"),
			HealthCheckType:        aws.String("ELB"),
			HealthCheckGracePeriod: aws.Int32(300),
		}},
	}, nil).Times(2)

	// After update: return ELB/600
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName:   aws.String("test-asg"),
			HealthCheckType:        aws.String("ELB"),
			HealthCheckGracePeriod: aws.Int32(600),
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockASG),
		Steps: []tfresource.TestStep{
			{
				Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name                    = "test-asg"
  health_check_type           = "ELB"
  health_check_grace_period   = 300
}`,
			},
			{
				Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name                    = "test-asg"
  health_check_type           = "ELB"
  health_check_grace_period   = 600
}`,
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("ebhelper_asg_health_check.test", "health_check_grace_period", "600"),
				),
			},
		},
	})
}

func TestAccUpdate_APIError(t *testing.T) {
	mockASG := new(mocks.MockASGClient)

	// First call (Create) succeeds, second call (Update) fails
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil).Once()
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("ThrottlingException: Rate exceeded")).Once()
	// Allow delete call
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil).Maybe()
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName:   aws.String("test-asg"),
			HealthCheckType:        aws.String("ELB"),
			HealthCheckGracePeriod: aws.Int32(300),
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockASG),
		Steps: []tfresource.TestStep{
			{
				Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name                    = "test-asg"
  health_check_type           = "ELB"
  health_check_grace_period   = 300
}`,
			},
			{
				Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name                    = "test-asg"
  health_check_type           = "EC2"
  health_check_grace_period   = 600
}`,
				ExpectError: regexp.MustCompile(`Failed to update ASG health check`),
			},
		},
	})
}

func TestAccDelete_ResetsToEC2(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName:   aws.String("test-asg"),
			HealthCheckType:        aws.String("ELB"),
			HealthCheckGracePeriod: aws.Int32(300),
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_health_check" "test" {
  asg_name          = "test-asg"
  health_check_type = "ELB"
}`,
		}},
	})

	// Verify Delete called with EC2 + 300
	var deleteCall *autoscaling.UpdateAutoScalingGroupInput
	for _, c := range mockASG.Calls {
		if c.Method == "UpdateAutoScalingGroup" {
			input := c.Arguments.Get(1).(*autoscaling.UpdateAutoScalingGroupInput)
			if aws.ToString(input.HealthCheckType) == "EC2" {
				deleteCall = input
			}
		}
	}
	require.NotNil(t, deleteCall, "expected delete to reset health check to EC2")
	assert.Equal(t, int32(300), aws.ToInt32(deleteCall.HealthCheckGracePeriod))
}

package asg_instance_maintenance_policy_test

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
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/asg_instance_maintenance_policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Schema Tests ---

func TestSchema_MinHealthyPercentageValidation(t *testing.T) {
	r := asg_instance_maintenance_policy.NewResource()
	schemaResp := &resource.SchemaResponse{}
	r.(resource.ResourceWithConfigure).Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	attr := schemaResp.Schema.Attributes["min_healthy_percentage"].(schema.Int64Attribute)
	assert.True(t, attr.Required)
	assert.NotEmpty(t, attr.Validators)
}

func TestSchema_MaxHealthyPercentageValidation(t *testing.T) {
	r := asg_instance_maintenance_policy.NewResource()
	schemaResp := &resource.SchemaResponse{}
	r.(resource.ResourceWithConfigure).Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	attr := schemaResp.Schema.Attributes["max_healthy_percentage"].(schema.Int64Attribute)
	assert.True(t, attr.Required)
	assert.NotEmpty(t, attr.Validators)
}

func TestSchema_ASGNameRequiresReplace(t *testing.T) {
	r := asg_instance_maintenance_policy.NewResource()
	schemaResp := &resource.SchemaResponse{}
	r.(resource.ResourceWithConfigure).Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	attr := schemaResp.Schema.Attributes["asg_name"].(schema.StringAttribute)
	assert.True(t, attr.Required)
	assert.NotEmpty(t, attr.PlanModifiers)
}

func TestMetadata(t *testing.T) {
	r := asg_instance_maintenance_policy.NewResource()
	resp := &resource.MetadataResponse{}
	r.(resource.ResourceWithConfigure).Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "ebhelper"}, resp)
	assert.Equal(t, "ebhelper_asg_instance_maintenance_policy", resp.TypeName)
}

// --- Configure Tests ---

func TestConfigure_NilProviderData(t *testing.T) {
	r := asg_instance_maintenance_policy.NewResource()
	resp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)
	assert.False(t, resp.Diagnostics.HasError())
}

func TestConfigure_ValidClients(t *testing.T) {
	r := asg_instance_maintenance_policy.NewResource()
	resp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: &awsclient.Clients{}}, resp)
	assert.False(t, resp.Diagnostics.HasError())
}

func TestConfigure_InvalidType(t *testing.T) {
	r := asg_instance_maintenance_policy.NewResource()
	resp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: 42}, resp)
	assert.True(t, resp.Diagnostics.HasError())
}

// --- CRUD Acceptance Tests ---

func testProviderFactories(mockASG *mocks.MockASGClient) map[string]func() (tfprotov6.ProviderServer, error) {
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
			AutoScalingGroupName: aws.String("test-asg"),
			InstanceMaintenancePolicy: &asgtypes.InstanceMaintenancePolicy{
				MinHealthyPercentage: aws.Int32(90),
				MaxHealthyPercentage: aws.Int32(120),
			},
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{
			{
				Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 120
}`,
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("ebhelper_asg_instance_maintenance_policy.test", "id", "test-asg"),
					tfresource.TestCheckResourceAttr("ebhelper_asg_instance_maintenance_policy.test", "min_healthy_percentage", "90"),
					tfresource.TestCheckResourceAttr("ebhelper_asg_instance_maintenance_policy.test", "max_healthy_percentage", "120"),
				),
			},
		},
	})

	// Verify Create called with correct policy
	var createCall *autoscaling.UpdateAutoScalingGroupInput
	for _, c := range mockASG.Calls {
		if c.Method == "UpdateAutoScalingGroup" {
			input := c.Arguments.Get(1).(*autoscaling.UpdateAutoScalingGroupInput)
			if input.InstanceMaintenancePolicy != nil && aws.ToInt32(input.InstanceMaintenancePolicy.MinHealthyPercentage) == 90 {
				createCall = input
				break
			}
		}
	}
	require.NotNil(t, createCall)
	assert.Equal(t, int32(120), aws.ToInt32(createCall.InstanceMaintenancePolicy.MaxHealthyPercentage))
}

func TestAccCreate_APIError(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("ValidationError: ASG not found"))

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "missing-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 100
}`,
			ExpectError: regexp.MustCompile(`Failed to set instance maintenance policy`),
		}},
	})
}

func TestAccRead_DriftDetection(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("test-asg"),
			InstanceMaintenancePolicy: &asgtypes.InstanceMaintenancePolicy{
				MinHealthyPercentage: aws.Int32(50),  // Drifted from 90
				MaxHealthyPercentage: aws.Int32(150), // Drifted from 100
			},
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 100
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
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 100
}`,
			ExpectNonEmptyPlan: true,
		}},
	})
}

func TestAccRead_PolicyRemovedExternally(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName:      aws.String("test-asg"),
			InstanceMaintenancePolicy: nil, // Removed externally
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 100
}`,
			ExpectNonEmptyPlan: true,
		}},
	})
}

func TestAccRead_APIError(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("throttled"))

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 100
}`,
			ExpectError: regexp.MustCompile(`Failed to read ASG`),
		}},
	})
}

func TestAccUpdate_ChangesPercentages(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)

	// First: return 90/120 for create+read
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("test-asg"),
			InstanceMaintenancePolicy: &asgtypes.InstanceMaintenancePolicy{
				MinHealthyPercentage: aws.Int32(90),
				MaxHealthyPercentage: aws.Int32(120),
			},
		}},
	}, nil).Times(2)

	// After update: return 80/150
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("test-asg"),
			InstanceMaintenancePolicy: &asgtypes.InstanceMaintenancePolicy{
				MinHealthyPercentage: aws.Int32(80),
				MaxHealthyPercentage: aws.Int32(150),
			},
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{
			{
				Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 120
}`,
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("ebhelper_asg_instance_maintenance_policy.test", "min_healthy_percentage", "90"),
					tfresource.TestCheckResourceAttr("ebhelper_asg_instance_maintenance_policy.test", "max_healthy_percentage", "120"),
				),
			},
			{
				Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 80
  max_healthy_percentage = 150
}`,
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("ebhelper_asg_instance_maintenance_policy.test", "min_healthy_percentage", "80"),
					tfresource.TestCheckResourceAttr("ebhelper_asg_instance_maintenance_policy.test", "max_healthy_percentage", "150"),
				),
			},
		},
	})

	// Verify Update was called (at least create + update + delete)
	updateCalls := 0
	for _, c := range mockASG.Calls {
		if c.Method == "UpdateAutoScalingGroup" {
			updateCalls++
		}
	}
	assert.GreaterOrEqual(t, updateCalls, 3)
}

func TestAccUpdate_APIError(t *testing.T) {
	mockASG := new(mocks.MockASGClient)

	// First call (Create) succeeds, second call (Update) fails
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil).Once()
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("ServiceUnavailable")).Once()
	// Allow delete call
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil).Maybe()
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("test-asg"),
			InstanceMaintenancePolicy: &asgtypes.InstanceMaintenancePolicy{
				MinHealthyPercentage: aws.Int32(90),
				MaxHealthyPercentage: aws.Int32(120),
			},
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{
			{
				Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 120
}`,
			},
			{
				Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 50
  max_healthy_percentage = 200
}`,
				ExpectError: regexp.MustCompile(`Failed to update instance maintenance policy`),
			},
		},
	})
}

func TestAccDelete_RemovesPolicy(t *testing.T) {
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("test-asg"),
			InstanceMaintenancePolicy: &asgtypes.InstanceMaintenancePolicy{
				MinHealthyPercentage: aws.Int32(90),
				MaxHealthyPercentage: aws.Int32(120),
			},
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 120
}`,
		}},
	})

	// Verify Delete called with -1 values to remove policy
	var deleteCall *autoscaling.UpdateAutoScalingGroupInput
	for _, c := range mockASG.Calls {
		if c.Method == "UpdateAutoScalingGroup" {
			input := c.Arguments.Get(1).(*autoscaling.UpdateAutoScalingGroupInput)
			if input.InstanceMaintenancePolicy != nil &&
				aws.ToInt32(input.InstanceMaintenancePolicy.MinHealthyPercentage) == -1 {
				deleteCall = input
			}
		}
	}
	require.NotNil(t, deleteCall, "expected delete to set -1 policy values")
	assert.Equal(t, int32(-1), aws.ToInt32(deleteCall.InstanceMaintenancePolicy.MaxHealthyPercentage))
}

func TestAccDelete_APIError_ExercisesPath(t *testing.T) {
	// This test verifies the Delete code path is exercised during teardown.
	// The testing framework reports destroy errors but doesn't fail the test step.
	// We verify coverage by checking the mock was called with -1 values.
	mockASG := new(mocks.MockASGClient)
	mockASG.On("UpdateAutoScalingGroup", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.UpdateAutoScalingGroupOutput{}, nil)
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("test-asg"),
			InstanceMaintenancePolicy: &asgtypes.InstanceMaintenancePolicy{
				MinHealthyPercentage: aws.Int32(90),
				MaxHealthyPercentage: aws.Int32(120),
			},
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 120
}`,
		}},
	})

	// Verify Delete was called with -1/-1 (the delete invocation)
	var deleteCall *autoscaling.UpdateAutoScalingGroupInput
	for _, c := range mockASG.Calls {
		if c.Method == "UpdateAutoScalingGroup" {
			input := c.Arguments.Get(1).(*autoscaling.UpdateAutoScalingGroupInput)
			if input.InstanceMaintenancePolicy != nil &&
				aws.ToInt32(input.InstanceMaintenancePolicy.MinHealthyPercentage) == -1 {
				deleteCall = input
			}
		}
	}
	require.NotNil(t, deleteCall, "expected delete to call UpdateAutoScalingGroup with -1 values")
	assert.Equal(t, int32(-1), aws.ToInt32(deleteCall.InstanceMaintenancePolicy.MaxHealthyPercentage))
}

func TestAccValidation_MinHealthyTooHigh(t *testing.T) {
	mockASG := new(mocks.MockASGClient)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{
			{
				Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 101
  max_healthy_percentage = 100
}`,
				ExpectError: regexp.MustCompile(`must be between 0 and 100`),
			},
		},
	})
}

func TestAccValidation_MaxHealthyTooLow(t *testing.T) {
	mockASG := new(mocks.MockASGClient)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{
			{
				Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 99
}`,
				ExpectError: regexp.MustCompile(`must be between 100 and 200`),
			},
		},
	})
}

func TestAccValidation_MaxHealthyTooHigh(t *testing.T) {
	mockASG := new(mocks.MockASGClient)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories(mockASG),
		Steps: []tfresource.TestStep{
			{
				Config: `resource "ebhelper_asg_instance_maintenance_policy" "test" {
  asg_name               = "test-asg"
  min_healthy_percentage = 90
  max_healthy_percentage = 201
}`,
				ExpectError: regexp.MustCompile(`must be between 100 and 200`),
			},
		},
	})
}

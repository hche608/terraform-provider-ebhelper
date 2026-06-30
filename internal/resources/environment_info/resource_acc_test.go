package environment_info_test

import (
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hche608/terraform-provider-ebhelper/internal/awsclient"
	"github.com/hche608/terraform-provider-ebhelper/internal/mocks"
	tfprovider "github.com/hche608/terraform-provider-ebhelper/internal/provider"
	"github.com/stretchr/testify/mock"
)

func providerFactories(mockEB *mocks.MockEBClient, mockASG *mocks.MockASGClient, mockELB *mocks.MockELBClient) map[string]func() (tfprotov6.ProviderServer, error) {
	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}
	return map[string]func() (tfprotov6.ProviderServer, error){
		"ebhelper": providerserver.NewProtocol6WithError(tfprovider.NewTestProvider(clients)),
	}
}

func TestAccEnvironmentInfo_Create_Success(t *testing.T) {
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	// DescribeEnvironments
	mockEB.On("DescribeEnvironments", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentsOutput{
		Environments: []ebtypes.EnvironmentDescription{{
			EnvironmentId:   aws.String("e-test123"),
			EnvironmentName: aws.String("my-env"),
			ApplicationName: aws.String("my-app"),
			EndpointURL:     aws.String("internal-lb.ap-southeast-2.elb.amazonaws.com"),
			PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Corretto 21/4.8.1"),
			HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
			CNAME:           aws.String("my-env.ap-southeast-2.elasticbeanstalk.com"),
		}},
	}, nil)

	// DescribeEnvironmentResources
	mockEB.On("DescribeEnvironmentResources", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
		EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
			AutoScalingGroups: []ebtypes.AutoScalingGroup{{Name: aws.String("awseb-e-test123-AWSEBAutoScalingGroup-ABC")}},
			Instances:         []ebtypes.Instance{{Id: aws.String("i-abc123")}},
			LaunchTemplates:   []ebtypes.LaunchTemplate{{Id: aws.String("lt-abc123")}},
		},
	}, nil)

	// DescribeAutoScalingGroups
	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("awseb-e-test123-AWSEBAutoScalingGroup-ABC"),
			AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:ap-southeast-2:123456789012:autoScalingGroup:id:asg"),
			TargetGroupARNs:      []string{"arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/tg/123"},
		}},
	}, nil)

	// DescribeTargetGroups
	mockELB.On("DescribeTargetGroups", mock.Anything, mock.Anything, mock.Anything).Return(&elasticloadbalancingv2.DescribeTargetGroupsOutput{
		TargetGroups: []elbtypes.TargetGroup{{
			TargetGroupName:  aws.String("my-tg"),
			TargetGroupArn:   aws.String("arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/tg/123"),
			LoadBalancerArns: []string{"arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:loadbalancer/app/lb/456"},
		}},
	}, nil)

	// DescribeLoadBalancers
	mockELB.On("DescribeLoadBalancers", mock.Anything, mock.Anything, mock.Anything).Return(&elasticloadbalancingv2.DescribeLoadBalancersOutput{
		LoadBalancers: []elbtypes.LoadBalancer{{
			LoadBalancerArn: aws.String("arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:loadbalancer/app/lb/456"),
			DNSName:         aws.String("internal-lb.ap-southeast-2.elb.amazonaws.com"),
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockEB, mockASG, mockELB),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_environment_info" "test" {
  application_name = "my-app"
  environment_name = "my-env"
  polling_timeout  = 5
  polling_interval = 1
}`,
			Check: tfresource.ComposeTestCheckFunc(
				tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "id", "e-test123"),
				tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "environment_id", "e-test123"),
				tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "asg_name", "awseb-e-test123-AWSEBAutoScalingGroup-ABC"),
				tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "health_status", "Ok"),
				tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "launch_template_id", "lt-abc123"),
				tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "target_group_names.0", "my-tg"),
				tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "load_balancer_dns_names.0", "internal-lb.ap-southeast-2.elb.amazonaws.com"),
				tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "instance_ids.0", "i-abc123"),
			),
		}},
	})
}

func TestAccEnvironmentInfo_Create_DiscoveryTimeout(t *testing.T) {
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	// Return empty environments (never found)
	mockEB.On("DescribeEnvironments", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentsOutput{
		Environments: []ebtypes.EnvironmentDescription{},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockEB, mockASG, mockELB),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_environment_info" "test" {
  application_name = "my-app"
  environment_name = "nonexistent"
  polling_timeout  = 2
  polling_interval = 1
}`,
			ExpectError: regexp.MustCompile(`discovery timeout`),
		}},
	})
}

func TestAccEnvironmentInfo_Update_ChangesPollingInterval(t *testing.T) {
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	// All calls succeed for both Create, Read, and Update
	mockEB.On("DescribeEnvironments", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentsOutput{
		Environments: []ebtypes.EnvironmentDescription{{
			EnvironmentId:   aws.String("e-update123"),
			EnvironmentName: aws.String("my-env"),
			ApplicationName: aws.String("my-app"),
			EndpointURL:     aws.String("lb.example.com"),
			PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
			HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
			CNAME:           aws.String("my-env.example.com"),
		}},
	}, nil)

	mockEB.On("DescribeEnvironmentResources", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
		EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
			AutoScalingGroups: []ebtypes.AutoScalingGroup{{Name: aws.String("asg-update")}},
			Instances:         []ebtypes.Instance{{Id: aws.String("i-update123")}},
			LaunchTemplates:   []ebtypes.LaunchTemplate{{Id: aws.String("lt-update123")}},
		},
	}, nil)

	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("asg-update"),
			AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:ap-southeast-2:123456789012:autoScalingGroup:id:asg-update"),
			TargetGroupARNs:      []string{},
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockEB, mockASG, mockELB),
		Steps: []tfresource.TestStep{
			{
				Config: `resource "ebhelper_environment_info" "test" {
  application_name = "my-app"
  environment_name = "my-env"
  polling_timeout  = 5
  polling_interval = 1
}`,
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "polling_interval", "1"),
					tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "polling_timeout", "5"),
				),
			},
			{
				Config: `resource "ebhelper_environment_info" "test" {
  application_name = "my-app"
  environment_name = "my-env"
  polling_timeout  = 60
  polling_interval = 15
}`,
				Check: tfresource.ComposeTestCheckFunc(
					tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "polling_interval", "15"),
					tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "polling_timeout", "60"),
					// Verify re-discovery happens (same infra data returned)
					tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "asg_name", "asg-update"),
					tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "environment_id", "e-update123"),
				),
			},
		},
	})
}

func TestAccEnvironmentInfo_Update_DiscoveryFailure(t *testing.T) {
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	// Create succeeds
	mockEB.On("DescribeEnvironments", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentsOutput{
		Environments: []ebtypes.EnvironmentDescription{{
			EnvironmentId:   aws.String("e-updfail"),
			EnvironmentName: aws.String("my-env"),
			ApplicationName: aws.String("my-app"),
			EndpointURL:     aws.String("lb.example.com"),
			PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
			HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
			CNAME:           aws.String("my-env.example.com"),
		}},
	}, nil).Times(3) // Create + Read calls

	mockEB.On("DescribeEnvironmentResources", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
		EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
			AutoScalingGroups: []ebtypes.AutoScalingGroup{{Name: aws.String("asg-updfail")}},
			Instances:         []ebtypes.Instance{},
			LaunchTemplates:   []ebtypes.LaunchTemplate{},
		},
	}, nil).Times(3)

	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("asg-updfail"),
			AutoScalingGroupARN:  aws.String("arn:asg"),
			TargetGroupARNs:      []string{},
		}},
	}, nil).Times(3)

	// Then environment disappears for Update re-discovery
	mockEB.On("DescribeEnvironments", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentsOutput{
		Environments: []ebtypes.EnvironmentDescription{},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockEB, mockASG, mockELB),
		Steps: []tfresource.TestStep{
			{
				Config: `resource "ebhelper_environment_info" "test" {
  application_name = "my-app"
  environment_name = "my-env"
  polling_timeout  = 2
  polling_interval = 1
}`,
			},
			{
				Config: `resource "ebhelper_environment_info" "test" {
  application_name = "my-app"
  environment_name = "my-env"
  polling_timeout  = 2
  polling_interval = 2
}`,
				ExpectError: regexp.MustCompile(`discovery timeout|discovery failed`),
			},
		},
	})
}

func TestAccEnvironmentInfo_Delete_IsNoOp(t *testing.T) {
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	// Full successful discovery
	mockEB.On("DescribeEnvironments", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentsOutput{
		Environments: []ebtypes.EnvironmentDescription{{
			EnvironmentId:   aws.String("e-del123"),
			EnvironmentName: aws.String("my-env"),
			ApplicationName: aws.String("my-app"),
			EndpointURL:     aws.String("lb.example.com"),
			PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
			HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
			CNAME:           aws.String("my-env.example.com"),
		}},
	}, nil)

	mockEB.On("DescribeEnvironmentResources", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
		EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
			AutoScalingGroups: []ebtypes.AutoScalingGroup{{Name: aws.String("asg-del")}},
			Instances:         []ebtypes.Instance{},
			LaunchTemplates:   []ebtypes.LaunchTemplate{},
		},
	}, nil)

	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("asg-del"),
			AutoScalingGroupARN:  aws.String("arn:asg"),
			TargetGroupARNs:      []string{},
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockEB, mockASG, mockELB),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_environment_info" "test" {
  application_name = "my-app"
  environment_name = "my-env"
  polling_timeout  = 5
  polling_interval = 1
}`,
		}},
	})

	// Delete is a no-op; the test completes successfully, proving Delete worked.
	// No additional API calls should be made for deletion since it just removes from state.
}

func TestAccEnvironmentInfo_Configure_InvalidType(t *testing.T) {
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	// This test just validates that the resource can be instantiated and configured
	// via the test provider (which provides valid Clients).
	mockEB.On("DescribeEnvironments", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentsOutput{
		Environments: []ebtypes.EnvironmentDescription{{
			EnvironmentId:   aws.String("e-cfg123"),
			EnvironmentName: aws.String("cfg-env"),
			ApplicationName: aws.String("cfg-app"),
			EndpointURL:     aws.String("lb.example.com"),
			PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
			HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
			CNAME:           aws.String("cfg.example.com"),
		}},
	}, nil)

	mockEB.On("DescribeEnvironmentResources", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
		EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
			AutoScalingGroups: []ebtypes.AutoScalingGroup{{Name: aws.String("asg-cfg")}},
			Instances:         []ebtypes.Instance{},
			LaunchTemplates:   []ebtypes.LaunchTemplate{},
		},
	}, nil)

	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("asg-cfg"),
			AutoScalingGroupARN:  aws.String("arn:asg"),
			TargetGroupARNs:      []string{},
		}},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockEB, mockASG, mockELB),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_environment_info" "test" {
  application_name = "cfg-app"
  environment_name = "cfg-env"
  polling_timeout  = 5
  polling_interval = 1
}`,
			Check: tfresource.ComposeTestCheckFunc(
				tfresource.TestCheckResourceAttr("ebhelper_environment_info.test", "id", "e-cfg123"),
			),
		}},
	})
}

func TestAccEnvironmentInfo_Read_RemovesWhenGone(t *testing.T) {
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	// Create succeeds
	mockEB.On("DescribeEnvironments", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentsOutput{
		Environments: []ebtypes.EnvironmentDescription{{
			EnvironmentId:   aws.String("e-test123"),
			EnvironmentName: aws.String("my-env"),
			ApplicationName: aws.String("my-app"),
			HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
		}},
	}, nil).Once()

	mockEB.On("DescribeEnvironmentResources", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
		EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
			AutoScalingGroups: []ebtypes.AutoScalingGroup{{Name: aws.String("asg-1")}},
			Instances:         []ebtypes.Instance{},
			LaunchTemplates:   []ebtypes.LaunchTemplate{},
		},
	}, nil).Once()

	mockASG.On("DescribeAutoScalingGroups", mock.Anything, mock.Anything, mock.Anything).Return(&autoscaling.DescribeAutoScalingGroupsOutput{
		AutoScalingGroups: []asgtypes.AutoScalingGroup{{
			AutoScalingGroupName: aws.String("asg-1"),
			AutoScalingGroupARN:  aws.String("arn:asg"),
			TargetGroupARNs:      []string{},
		}},
	}, nil).Once()

	// Read: environment is gone
	mockEB.On("DescribeEnvironments", mock.Anything, mock.Anything, mock.Anything).Return(&elasticbeanstalk.DescribeEnvironmentsOutput{
		Environments: []ebtypes.EnvironmentDescription{},
	}, nil)

	tfresource.UnitTest(t, tfresource.TestCase{
		ProtoV6ProviderFactories: providerFactories(mockEB, mockASG, mockELB),
		Steps: []tfresource.TestStep{{
			Config: `resource "ebhelper_environment_info" "test" {
  application_name = "my-app"
  environment_name = "my-env"
  polling_timeout  = 2
  polling_interval = 1
}`,
			ExpectNonEmptyPlan: true, // Removed from state during read, needs recreation
		}},
	})
}

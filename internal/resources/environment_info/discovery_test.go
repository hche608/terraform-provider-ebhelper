package environment_info

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/hche608/terraform-provider-ebhelper/internal/awsclient"
	"github.com/hche608/terraform-provider-ebhelper/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDiscover_Success_SingleASG_SingleTG_SingleLB(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	// DescribeEnvironments
	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentId:   aws.String("e-abc123def4"),
					EnvironmentName: aws.String("my-app-env"),
					EndpointURL:     aws.String("internal-lb-123.ap-southeast-2.elb.amazonaws.com"),
					PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Corretto 21/4.8.1"),
					HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
					CNAME:           aws.String("my-app-env.ap-southeast-2.elasticbeanstalk.com"),
				},
			},
		}, nil)

	// DescribeEnvironmentResources
	mockEB.On("DescribeEnvironmentResources", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentResourcesInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
			EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
				AutoScalingGroups: []ebtypes.AutoScalingGroup{
					{Name: aws.String("awseb-e-abc123def4-stack-AWSEBAutoScalingGroup-EXAMPLE123")},
				},
				Instances: []ebtypes.Instance{
					{Id: aws.String("i-0123456789abcdef0")},
				},
				LaunchTemplates: []ebtypes.LaunchTemplate{
					{Id: aws.String("lt-0123456789abcdef0")},
				},
			},
		}, nil)

	// DescribeAutoScalingGroups
	mockASG.On("DescribeAutoScalingGroups", ctx, mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.Anything).Return(
		&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("awseb-e-abc123def4-stack-AWSEBAutoScalingGroup-EXAMPLE123"),
					AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:ap-southeast-2:123456789012:autoScalingGroup:12345678-1234-1234-1234-123456789012:autoScalingGroupName/awseb-e-abc123def4-stack-AWSEBAutoScalingGroup-EXAMPLE123"),
					TargetGroupARNs:      []string{"arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/awseb-AWSEB-EXAMPLE1234/1234567890abcdef"},
				},
			},
		}, nil)

	// DescribeTargetGroups
	mockELB.On("DescribeTargetGroups", ctx, mock.AnythingOfType("*elasticloadbalancingv2.DescribeTargetGroupsInput"), mock.Anything).Return(
		&elasticloadbalancingv2.DescribeTargetGroupsOutput{
			TargetGroups: []elbtypes.TargetGroup{
				{
					TargetGroupName: aws.String("awseb-AWSEB-EXAMPLE1234"),
					TargetGroupArn:  aws.String("arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/awseb-AWSEB-EXAMPLE1234/1234567890abcdef"),
					LoadBalancerArns: []string{
						"arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:loadbalancer/app/awseb--AWSEB-abcdef123456/1234567890abcdef",
					},
				},
			},
		}, nil)

	// DescribeLoadBalancers
	mockELB.On("DescribeLoadBalancers", ctx, mock.AnythingOfType("*elasticloadbalancingv2.DescribeLoadBalancersInput"), mock.Anything).Return(
		&elasticloadbalancingv2.DescribeLoadBalancersOutput{
			LoadBalancers: []elbtypes.LoadBalancer{
				{
					DNSName: aws.String("internal-lb-123.ap-southeast-2.elb.amazonaws.com"),
				},
			},
		}, nil)

	result, err := Discover(ctx, clients, "my-app", "my-app-env", 1*time.Second, 5*time.Second)

	require.NoError(t, err)
	assert.Equal(t, "e-abc123def4", result.EnvironmentID)
	assert.Equal(t, "awseb-e-abc123def4-stack-AWSEBAutoScalingGroup-EXAMPLE123", result.ASGName)
	assert.Equal(t, "arn:aws:autoscaling:ap-southeast-2:123456789012:autoScalingGroup:12345678-1234-1234-1234-123456789012:autoScalingGroupName/awseb-e-abc123def4-stack-AWSEBAutoScalingGroup-EXAMPLE123", result.ASGARN)
	assert.Len(t, result.TargetGroupARNs, 1)
	assert.Len(t, result.LoadBalancerARNs, 1)
	assert.Len(t, result.LoadBalancerDNS, 1)
	assert.Equal(t, "internal-lb-123.ap-southeast-2.elb.amazonaws.com", result.LoadBalancerDNS[0])
	assert.Equal(t, []string{"i-0123456789abcdef0"}, result.InstanceIDs)
	assert.Equal(t, "lt-0123456789abcdef0", result.LaunchTemplateID)

	mockEB.AssertExpectations(t)
	mockASG.AssertExpectations(t)
	mockELB.AssertExpectations(t)
}

func TestDiscover_Success_MultipleTGs_SharedLB_Deduplication(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentId:   aws.String("e-xyz789ghi0"),
					EnvironmentName: aws.String("my-webapp-env"),
					EndpointURL:     aws.String("internal-shared-alb-123.ap-southeast-2.elb.amazonaws.com"),
					PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Corretto 21/4.8.1"),
					HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
					CNAME:           aws.String("my-webapp-env.ap-southeast-2.elasticbeanstalk.com"),
				},
			},
		}, nil)

	mockEB.On("DescribeEnvironmentResources", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentResourcesInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
			EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
				AutoScalingGroups: []ebtypes.AutoScalingGroup{
					{Name: aws.String("awseb-e-xyz789ghi0-stack-AWSEBAutoScalingGroup-EXAMPLE456")},
				},
				Instances:       []ebtypes.Instance{{Id: aws.String("i-0abcdef1234567890")}},
				LaunchTemplates: []ebtypes.LaunchTemplate{{Id: aws.String("lt-0abcdef1234567890")}},
			},
		}, nil)

	sharedLBArn := "arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:loadbalancer/app/my-shared-alb/abcdef1234567890"

	mockASG.On("DescribeAutoScalingGroups", ctx, mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.Anything).Return(
		&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("awseb-e-xyz789ghi0-stack-AWSEBAutoScalingGroup-EXAMPLE456"),
					AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:ap-southeast-2:123456789012:autoScalingGroup:87654321-4321-4321-4321-210987654321:autoScalingGroupName/awseb-e-xyz789ghi0-stack-AWSEBAutoScalingGroup-EXAMPLE456"),
					TargetGroupARNs: []string{
						"arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/tg-default/fedcba0987654321",
						"arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/tg-process8/0123456789abcdef",
					},
				},
			},
		}, nil)

	// Both TGs point to the same shared LB
	mockELB.On("DescribeTargetGroups", ctx, mock.AnythingOfType("*elasticloadbalancingv2.DescribeTargetGroupsInput"), mock.Anything).Return(
		&elasticloadbalancingv2.DescribeTargetGroupsOutput{
			TargetGroups: []elbtypes.TargetGroup{
				{
					TargetGroupName:  aws.String("tg-default"),
					TargetGroupArn:   aws.String("arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/tg-default/fedcba0987654321"),
					LoadBalancerArns: []string{sharedLBArn},
				},
				{
					TargetGroupName:  aws.String("tg-process8"),
					TargetGroupArn:   aws.String("arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/tg-process8/0123456789abcdef"),
					LoadBalancerArns: []string{sharedLBArn},
				},
			},
		}, nil)

	mockELB.On("DescribeLoadBalancers", ctx, mock.AnythingOfType("*elasticloadbalancingv2.DescribeLoadBalancersInput"), mock.Anything).Return(
		&elasticloadbalancingv2.DescribeLoadBalancersOutput{
			LoadBalancers: []elbtypes.LoadBalancer{
				{DNSName: aws.String("internal-shared-alb-123.ap-southeast-2.elb.amazonaws.com")},
			},
		}, nil)

	result, err := Discover(ctx, clients, "my-webapp", "my-webapp-env", 1*time.Second, 5*time.Second)

	require.NoError(t, err)
	assert.Equal(t, "e-xyz789ghi0", result.EnvironmentID)
	assert.Len(t, result.TargetGroupARNs, 2)
	assert.Len(t, result.TargetGroupNames, 2)
	// De-duplicated: both TGs share the same LB
	assert.Len(t, result.LoadBalancerARNs, 1)
	assert.Equal(t, sharedLBArn, result.LoadBalancerARNs[0])
	assert.Len(t, result.LoadBalancerDNS, 1)

	mockEB.AssertExpectations(t)
	mockASG.AssertExpectations(t)
	mockELB.AssertExpectations(t)
}

func TestDiscover_MultipleASGs_SelectsFirst(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentId:   aws.String("e-multi123"),
					EnvironmentName: aws.String("multi-asg-env"),
					EndpointURL:     aws.String("lb.example.com"),
					PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
					HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
					CNAME:           aws.String("multi.example.com"),
				},
			},
		}, nil)

	mockEB.On("DescribeEnvironmentResources", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentResourcesInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
			EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
				AutoScalingGroups: []ebtypes.AutoScalingGroup{
					{Name: aws.String("asg-primary")},
					{Name: aws.String("asg-immutable-temp")},
				},
				Instances:       []ebtypes.Instance{},
				LaunchTemplates: []ebtypes.LaunchTemplate{},
			},
		}, nil)

	mockASG.On("DescribeAutoScalingGroups", ctx, mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.Anything).Return(
		&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("asg-primary"),
					AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:ap-southeast-2:123456789012:autoScalingGroup:id:autoScalingGroupName/asg-primary"),
					TargetGroupARNs:      []string{},
				},
			},
		}, nil)

	result, err := Discover(ctx, clients, "my-app", "multi-asg-env", 1*time.Second, 5*time.Second)

	require.NoError(t, err)
	assert.Equal(t, "asg-primary", result.ASGName)
	assert.Equal(t, []string{"asg-primary", "asg-immutable-temp"}, result.AllASGNames)

	mockEB.AssertExpectations(t)
	mockASG.AssertExpectations(t)
}

func TestDiscover_EnvironmentNotFound_ReturnsError(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{},
		}, nil)

	_, err := Discover(ctx, clients, "my-app", "nonexistent-env", 1*time.Second, 2*time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "discovery timeout")
	assert.Contains(t, err.Error(), "not found")
}

func TestDiscover_ASGNotFoundInResources_ReturnsError(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentId:   aws.String("e-noasg123"),
					EnvironmentName: aws.String("no-asg-env"),
					EndpointURL:     aws.String("lb.example.com"),
					PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
					HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
					CNAME:           aws.String("no-asg.example.com"),
				},
			},
		}, nil)

	mockEB.On("DescribeEnvironmentResources", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentResourcesInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
			EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
				AutoScalingGroups: []ebtypes.AutoScalingGroup{}, // Empty
				Instances:         []ebtypes.Instance{},
				LaunchTemplates:   []ebtypes.LaunchTemplate{},
			},
		}, nil)

	_, err := Discover(ctx, clients, "my-app", "no-asg-env", 1*time.Second, 2*time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "discovery timeout")
	assert.Contains(t, err.Error(), "no ASG found")
}

func TestDiscover_Timeout_ReturnsErrorWithContext(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	// Always return empty environments so discovery never completes
	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{},
		}, nil)

	start := time.Now()
	_, err := Discover(ctx, clients, "my-app", "slow-env", 500*time.Millisecond, 1500*time.Millisecond)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "discovery timeout")
	// Should have taken at least 1s (at least one retry)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(1000))
}

func TestDiscover_NoTargetGroups_ReturnsEmptyLists(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentId:   aws.String("e-notg123"),
					EnvironmentName: aws.String("no-tg-env"),
					EndpointURL:     aws.String("lb.example.com"),
					PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
					HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
					CNAME:           aws.String("no-tg.example.com"),
				},
			},
		}, nil)

	mockEB.On("DescribeEnvironmentResources", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentResourcesInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
			EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
				AutoScalingGroups: []ebtypes.AutoScalingGroup{
					{Name: aws.String("asg-no-tg")},
				},
				Instances:       []ebtypes.Instance{},
				LaunchTemplates: []ebtypes.LaunchTemplate{},
			},
		}, nil)

	mockASG.On("DescribeAutoScalingGroups", ctx, mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.Anything).Return(
		&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("asg-no-tg"),
					AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:ap-southeast-2:123456789012:autoScalingGroup:id:autoScalingGroupName/asg-no-tg"),
					TargetGroupARNs:      []string{}, // No target groups
				},
			},
		}, nil)

	result, err := Discover(ctx, clients, "my-app", "no-tg-env", 1*time.Second, 5*time.Second)

	require.NoError(t, err)
	assert.Empty(t, result.TargetGroupARNs)
	assert.Empty(t, result.TargetGroupNames)
	assert.Empty(t, result.LoadBalancerARNs)
	assert.Empty(t, result.LoadBalancerDNS)

	// ELB should not have been called
	mockELB.AssertNotCalled(t, "DescribeTargetGroups")
	mockELB.AssertNotCalled(t, "DescribeLoadBalancers")
}

func TestDiscover_DescribeLoadBalancersFailure_ReturnsError(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentId:   aws.String("e-lberr123"),
					EnvironmentName: aws.String("lb-err-env"),
					EndpointURL:     aws.String("lb.example.com"),
					PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
					HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
					CNAME:           aws.String("lb-err.example.com"),
				},
			},
		}, nil)

	mockEB.On("DescribeEnvironmentResources", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentResourcesInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
			EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
				AutoScalingGroups: []ebtypes.AutoScalingGroup{
					{Name: aws.String("asg-lb-err")},
				},
				Instances:       []ebtypes.Instance{},
				LaunchTemplates: []ebtypes.LaunchTemplate{},
			},
		}, nil)

	mockASG.On("DescribeAutoScalingGroups", ctx, mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.Anything).Return(
		&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("asg-lb-err"),
					AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:ap-southeast-2:123456789012:autoScalingGroup:id:autoScalingGroupName/asg-lb-err"),
					TargetGroupARNs:      []string{"arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/tg1/abcdef"},
				},
			},
		}, nil)

	mockELB.On("DescribeTargetGroups", ctx, mock.AnythingOfType("*elasticloadbalancingv2.DescribeTargetGroupsInput"), mock.Anything).Return(
		&elasticloadbalancingv2.DescribeTargetGroupsOutput{
			TargetGroups: []elbtypes.TargetGroup{
				{
					TargetGroupName:  aws.String("tg1"),
					TargetGroupArn:   aws.String("arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/tg1/abcdef"),
					LoadBalancerArns: []string{"arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:loadbalancer/app/my-lb/123"},
				},
			},
		}, nil)

	mockELB.On("DescribeLoadBalancers", ctx, mock.AnythingOfType("*elasticloadbalancingv2.DescribeLoadBalancersInput"), mock.Anything).Return(
		nil, fmt.Errorf("access denied"))

	_, err := Discover(ctx, clients, "my-app", "lb-err-env", 1*time.Second, 2*time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DescribeLoadBalancers failed")
}

func TestDiscoverASG_Success(t *testing.T) {
	ctx := context.Background()
	mockASG := new(mocks.MockASGClient)

	clients := &awsclient.Clients{
		AutoScaling: mockASG,
	}

	mockASG.On("DescribeAutoScalingGroups", ctx, mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.Anything).Return(
		&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName:   aws.String("my-asg"),
					AutoScalingGroupARN:    aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:id:autoScalingGroupName/my-asg"),
					HealthCheckType:        aws.String("ELB"),
					HealthCheckGracePeriod: aws.Int32(300),
				},
			},
		}, nil)

	output, err := DiscoverASG(ctx, clients, "my-asg")

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.Len(t, output.AutoScalingGroups, 1)
	assert.Equal(t, "my-asg", aws.ToString(output.AutoScalingGroups[0].AutoScalingGroupName))

	mockASG.AssertExpectations(t)
}

func TestDiscoverASG_NotFound(t *testing.T) {
	ctx := context.Background()
	mockASG := new(mocks.MockASGClient)

	clients := &awsclient.Clients{
		AutoScaling: mockASG,
	}

	mockASG.On("DescribeAutoScalingGroups", ctx, mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.Anything).Return(
		&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{},
		}, nil)

	output, err := DiscoverASG(ctx, clients, "nonexistent-asg")

	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "ASG \"nonexistent-asg\" not found")

	mockASG.AssertExpectations(t)
}

func TestDiscoverASG_APIError(t *testing.T) {
	ctx := context.Background()
	mockASG := new(mocks.MockASGClient)

	clients := &awsclient.Clients{
		AutoScaling: mockASG,
	}

	mockASG.On("DescribeAutoScalingGroups", ctx, mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.Anything).Return(
		nil, fmt.Errorf("AccessDeniedException: not authorized"))

	output, err := DiscoverASG(ctx, clients, "my-asg")

	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "DescribeAutoScalingGroups failed")
	assert.Contains(t, err.Error(), "AccessDeniedException")

	mockASG.AssertExpectations(t)
}

func TestDiscoverOnce_ASGNotFoundInDescribeAutoScalingGroups(t *testing.T) {
	// This tests the edge case where EB reports an ASG name, but
	// DescribeAutoScalingGroups returns empty for that name.
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentId:   aws.String("e-ghost123"),
					EnvironmentName: aws.String("ghost-asg-env"),
					EndpointURL:     aws.String("lb.example.com"),
					PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
					HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
					CNAME:           aws.String("ghost.example.com"),
				},
			},
		}, nil)

	mockEB.On("DescribeEnvironmentResources", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentResourcesInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
			EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
				AutoScalingGroups: []ebtypes.AutoScalingGroup{
					{Name: aws.String("asg-that-exists-in-eb-but-not-asg-api")},
				},
				Instances:       []ebtypes.Instance{},
				LaunchTemplates: []ebtypes.LaunchTemplate{},
			},
		}, nil)

	// DescribeAutoScalingGroups returns empty — the ASG was deleted between EB and ASG calls
	mockASG.On("DescribeAutoScalingGroups", ctx, mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.Anything).Return(
		&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{},
		}, nil)

	result, err := discoverOnce(ctx, clients, "my-app", "ghost-asg-env")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found in DescribeAutoScalingGroups")
}

func TestDiscoverOnce_DescribeEnvironmentsAPIError(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		nil, fmt.Errorf("ThrottlingException: Rate exceeded"))

	result, err := discoverOnce(ctx, clients, "my-app", "throttled-env")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "DescribeEnvironments failed")
}

func TestDiscoverOnce_DescribeEnvironmentResourcesAPIError(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentId:   aws.String("e-reserr123"),
					EnvironmentName: aws.String("res-err-env"),
					EndpointURL:     aws.String("lb.example.com"),
					PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
					HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
					CNAME:           aws.String("res-err.example.com"),
				},
			},
		}, nil)

	mockEB.On("DescribeEnvironmentResources", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentResourcesInput"), mock.Anything).Return(
		nil, fmt.Errorf("InternalFailure: service error"))

	result, err := discoverOnce(ctx, clients, "my-app", "res-err-env")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "DescribeEnvironmentResources failed")
}

func TestDiscoverOnce_DescribeAutoScalingGroupsAPIError(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentId:   aws.String("e-asgerr123"),
					EnvironmentName: aws.String("asg-err-env"),
					EndpointURL:     aws.String("lb.example.com"),
					PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
					HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
					CNAME:           aws.String("asg-err.example.com"),
				},
			},
		}, nil)

	mockEB.On("DescribeEnvironmentResources", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentResourcesInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
			EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
				AutoScalingGroups: []ebtypes.AutoScalingGroup{
					{Name: aws.String("asg-err")},
				},
				Instances:       []ebtypes.Instance{},
				LaunchTemplates: []ebtypes.LaunchTemplate{},
			},
		}, nil)

	mockASG.On("DescribeAutoScalingGroups", ctx, mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.Anything).Return(
		nil, fmt.Errorf("ValidationError: group not found"))

	result, err := discoverOnce(ctx, clients, "my-app", "asg-err-env")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "DescribeAutoScalingGroups failed")
}

func TestDiscoverOnce_DescribeTargetGroupsAPIError(t *testing.T) {
	ctx := context.Background()
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	mockEB.On("DescribeEnvironments", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{
				{
					EnvironmentId:   aws.String("e-tgerr123"),
					EnvironmentName: aws.String("tg-err-env"),
					EndpointURL:     aws.String("lb.example.com"),
					PlatformArn:     aws.String("arn:aws:elasticbeanstalk:ap-southeast-2::platform/Test/1.0"),
					HealthStatus:    ebtypes.EnvironmentHealthStatusOk,
					CNAME:           aws.String("tg-err.example.com"),
				},
			},
		}, nil)

	mockEB.On("DescribeEnvironmentResources", ctx, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentResourcesInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentResourcesOutput{
			EnvironmentResources: &ebtypes.EnvironmentResourceDescription{
				AutoScalingGroups: []ebtypes.AutoScalingGroup{
					{Name: aws.String("asg-tg-err")},
				},
				Instances:       []ebtypes.Instance{},
				LaunchTemplates: []ebtypes.LaunchTemplate{},
			},
		}, nil)

	mockASG.On("DescribeAutoScalingGroups", ctx, mock.AnythingOfType("*autoscaling.DescribeAutoScalingGroupsInput"), mock.Anything).Return(
		&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []asgtypes.AutoScalingGroup{
				{
					AutoScalingGroupName: aws.String("asg-tg-err"),
					AutoScalingGroupARN:  aws.String("arn:aws:autoscaling:ap-southeast-2:123456789012:autoScalingGroup:id:autoScalingGroupName/asg-tg-err"),
					TargetGroupARNs:      []string{"arn:aws:elasticloadbalancing:ap-southeast-2:123456789012:targetgroup/tg1/abc"},
				},
			},
		}, nil)

	mockELB.On("DescribeTargetGroups", ctx, mock.AnythingOfType("*elasticloadbalancingv2.DescribeTargetGroupsInput"), mock.Anything).Return(
		nil, fmt.Errorf("TargetGroupNotFound"))

	result, err := discoverOnce(ctx, clients, "my-app", "tg-err-env")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "DescribeTargetGroups failed")
}

func TestDiscover_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mockEB := new(mocks.MockEBClient)
	mockASG := new(mocks.MockASGClient)
	mockELB := new(mocks.MockELBClient)

	clients := &awsclient.Clients{
		ElasticBeanstalk: mockEB,
		AutoScaling:      mockASG,
		ELBv2:            mockELB,
	}

	// Return empty so it keeps retrying
	mockEB.On("DescribeEnvironments", mock.Anything, mock.AnythingOfType("*elasticbeanstalk.DescribeEnvironmentsInput"), mock.Anything).Return(
		&elasticbeanstalk.DescribeEnvironmentsOutput{
			Environments: []ebtypes.EnvironmentDescription{},
		}, nil)

	// Cancel after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	_, err := Discover(ctx, clients, "my-app", "cancel-env", 100*time.Millisecond, 10*time.Second)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

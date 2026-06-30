// Package awsclient provides AWS service client interfaces and construction
// for the ebhelper Terraform provider.
package awsclient

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Config holds provider-level AWS configuration.
type Config struct {
	Region      string
	RoleARN     string
	SessionName string
	ExternalID  string
}

// Clients holds all AWS service clients needed by the provider.
type Clients struct {
	ElasticBeanstalk EBClient
	AutoScaling      ASGClient
	ELBv2            ELBClient
}

// EBClient abstracts Elastic Beanstalk API calls.
type EBClient interface {
	DescribeEnvironments(ctx context.Context, params *elasticbeanstalk.DescribeEnvironmentsInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentsOutput, error)
	DescribeEnvironmentResources(ctx context.Context, params *elasticbeanstalk.DescribeEnvironmentResourcesInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error)
}

// ASGClient abstracts Auto Scaling API calls.
type ASGClient interface {
	DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
	UpdateAutoScalingGroup(ctx context.Context, params *autoscaling.UpdateAutoScalingGroupInput, optFns ...func(*autoscaling.Options)) (*autoscaling.UpdateAutoScalingGroupOutput, error)
}

// ELBClient abstracts ELBv2 API calls.
type ELBClient interface {
	DescribeTargetGroups(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error)
	DescribeLoadBalancers(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error)
	ModifyLoadBalancerAttributes(ctx context.Context, params *elasticloadbalancingv2.ModifyLoadBalancerAttributesInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.ModifyLoadBalancerAttributesOutput, error)
	DescribeLoadBalancerAttributes(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancerAttributesInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancerAttributesOutput, error)
}

// NewClients builds AWS service clients from the provider config.
func NewClients(ctx context.Context, cfg Config) (*Clients, error) {
	// Build options for AWS config loading
	var opts []func(*awsconfig.LoadOptions) error

	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}

	// Load default AWS config
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("[ebhelper provider] failed to load AWS config: %w", err)
	}

	// If role ARN is provided, configure AssumeRole
	if cfg.RoleARN != "" {
		stsClient := sts.NewFromConfig(awsCfg)

		sessionName := cfg.SessionName
		if sessionName == "" {
			sessionName = "terraform-ebhelper"
		}

		var assumeRoleOpts []func(*stscreds.AssumeRoleOptions)
		if cfg.ExternalID != "" {
			assumeRoleOpts = append(assumeRoleOpts, func(o *stscreds.AssumeRoleOptions) {
				o.ExternalID = aws.String(cfg.ExternalID)
			})
		}

		creds := stscreds.NewAssumeRoleProvider(stsClient, cfg.RoleARN, assumeRoleOpts...)

		// Verify credentials work by retrieving them
		_, err := creds.Retrieve(ctx)
		if err != nil {
			return nil, fmt.Errorf("[ebhelper provider] assume role failed for %q: %w", cfg.RoleARN, err)
		}

		awsCfg.Credentials = aws.NewCredentialsCache(creds)
	}

	return &Clients{
		ElasticBeanstalk: elasticbeanstalk.NewFromConfig(awsCfg),
		AutoScaling:      autoscaling.NewFromConfig(awsCfg),
		ELBv2:            elasticloadbalancingv2.NewFromConfig(awsCfg),
	}, nil
}

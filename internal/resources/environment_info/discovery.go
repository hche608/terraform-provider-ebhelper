// Package environment_info implements the ebhelper_environment_info resource
// which discovers AWS Elastic Beanstalk environment infrastructure.
package environment_info

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hche608/terraform-provider-ebhelper/internal/awsclient"
)

// DiscoveryResult holds all data discovered from AWS APIs.
type DiscoveryResult struct {
	// EB Environment metadata
	EnvironmentID string
	EndpointURL   string
	PlatformARN   string
	HealthStatus  string
	CNAME         string

	// ASG
	ASGName     string
	ASGARN      string
	AllASGNames []string

	// Target Groups
	TargetGroupARNs  []string
	TargetGroupNames []string

	// Load Balancers
	LoadBalancerARNs []string
	LoadBalancerDNS  []string

	// Instances
	InstanceIDs      []string
	LaunchTemplateID string
}

// Discover runs the full discovery sequence with polling.
func Discover(ctx context.Context, clients *awsclient.Clients, appName, envName string, interval, timeout time.Duration) (*DiscoveryResult, error) {
	deadline := time.Now().Add(timeout)
	attempt := 0

	for {
		attempt++
		elapsed := time.Since(deadline.Add(-timeout))

		tflog.Debug(ctx, "Discovery attempt", map[string]interface{}{
			"attempt":  attempt,
			"elapsed":  elapsed.String(),
			"app_name": appName,
			"env_name": envName,
		})

		result, err := discoverOnce(ctx, clients, appName, envName)
		if err == nil {
			return result, nil
		}

		// Check if we've exceeded the timeout
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("[ebhelper_environment_info] discovery timeout after %s: %w", timeout, err)
		}

		tflog.Info(ctx, "Discovery not yet complete, retrying", map[string]interface{}{
			"attempt":       attempt,
			"error":         err.Error(),
			"next_retry_in": interval.String(),
		})

		// Wait for next interval or context cancellation
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("[ebhelper_environment_info] discovery cancelled: %w", ctx.Err())
		case <-time.After(interval):
		}
	}
}

// discoverOnce attempts a single discovery pass.
func discoverOnce(ctx context.Context, clients *awsclient.Clients, appName, envName string) (*DiscoveryResult, error) {
	result := &DiscoveryResult{}

	// Step 1: Resolve environment ID
	envOutput, err := clients.ElasticBeanstalk.DescribeEnvironments(ctx, &elasticbeanstalk.DescribeEnvironmentsInput{
		ApplicationName:  aws.String(appName),
		EnvironmentNames: []string{envName},
	})
	if err != nil {
		return nil, fmt.Errorf("DescribeEnvironments failed for %q/%q: %w", appName, envName, err)
	}

	if len(envOutput.Environments) == 0 {
		return nil, fmt.Errorf("environment %q/%q not found (0 results from DescribeEnvironments)", appName, envName)
	}

	env := envOutput.Environments[0]
	result.EnvironmentID = aws.ToString(env.EnvironmentId)
	result.EndpointURL = aws.ToString(env.EndpointURL)
	result.PlatformARN = aws.ToString(env.PlatformArn)
	result.HealthStatus = string(env.HealthStatus)
	result.CNAME = aws.ToString(env.CNAME)

	// Step 2: Get environment resources
	resOutput, err := clients.ElasticBeanstalk.DescribeEnvironmentResources(ctx, &elasticbeanstalk.DescribeEnvironmentResourcesInput{
		EnvironmentId: aws.String(result.EnvironmentID),
	})
	if err != nil {
		return nil, fmt.Errorf("DescribeEnvironmentResources failed for env_id %q: %w", result.EnvironmentID, err)
	}

	resources := resOutput.EnvironmentResources

	// Extract ASG names
	if len(resources.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("no ASG found for environment %q", envName)
	}

	for _, asg := range resources.AutoScalingGroups {
		result.AllASGNames = append(result.AllASGNames, aws.ToString(asg.Name))
	}

	// Use first ASG from EB API as the active one
	activeASGName := aws.ToString(resources.AutoScalingGroups[0].Name)
	result.ASGName = activeASGName

	if len(resources.AutoScalingGroups) > 1 {
		tflog.Warn(ctx, "Multiple ASGs found, using first from EB API as active", map[string]interface{}{
			"active_asg": activeASGName,
			"total_asgs": len(resources.AutoScalingGroups),
		})
	}

	// Extract launch template ID
	if len(resources.LaunchTemplates) > 0 {
		result.LaunchTemplateID = aws.ToString(resources.LaunchTemplates[0].Id)
	}

	// Extract instance IDs
	for _, inst := range resources.Instances {
		result.InstanceIDs = append(result.InstanceIDs, aws.ToString(inst.Id))
	}

	// Step 3: Get full ASG details
	asgOutput, err := clients.AutoScaling.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{activeASGName},
	})
	if err != nil {
		return nil, fmt.Errorf("DescribeAutoScalingGroups failed for %q: %w", activeASGName, err)
	}

	if len(asgOutput.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("ASG %q not found in DescribeAutoScalingGroups", activeASGName)
	}

	asg := asgOutput.AutoScalingGroups[0]
	result.ASGARN = aws.ToString(asg.AutoScalingGroupARN)
	result.TargetGroupARNs = asg.TargetGroupARNs

	// Step 4: Get target group details (if any)
	if len(result.TargetGroupARNs) > 0 {
		tgOutput, err := clients.ELBv2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
			TargetGroupArns: result.TargetGroupARNs,
		})
		if err != nil {
			return nil, fmt.Errorf("DescribeTargetGroups failed: %w", err)
		}

		// Collect TG names and LB ARNs (de-duplicated)
		lbArnSet := make(map[string]struct{})
		for _, tg := range tgOutput.TargetGroups {
			result.TargetGroupNames = append(result.TargetGroupNames, aws.ToString(tg.TargetGroupName))
			for _, lbArn := range tg.LoadBalancerArns {
				lbArnSet[lbArn] = struct{}{}
			}
		}

		for arn := range lbArnSet {
			result.LoadBalancerARNs = append(result.LoadBalancerARNs, arn)
		}

		// Step 5: Get load balancer DNS names
		if len(result.LoadBalancerARNs) > 0 {
			lbOutput, err := clients.ELBv2.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{
				LoadBalancerArns: result.LoadBalancerARNs,
			})
			if err != nil {
				return nil, fmt.Errorf("DescribeLoadBalancers failed: %w", err)
			}

			for _, lb := range lbOutput.LoadBalancers {
				result.LoadBalancerDNS = append(result.LoadBalancerDNS, aws.ToString(lb.DNSName))
			}
		}
	}

	// Ensure nil slices are empty slices
	if result.AllASGNames == nil {
		result.AllASGNames = []string{}
	}
	if result.TargetGroupARNs == nil {
		result.TargetGroupARNs = []string{}
	}
	if result.TargetGroupNames == nil {
		result.TargetGroupNames = []string{}
	}
	if result.LoadBalancerARNs == nil {
		result.LoadBalancerARNs = []string{}
	}
	if result.LoadBalancerDNS == nil {
		result.LoadBalancerDNS = []string{}
	}
	if result.InstanceIDs == nil {
		result.InstanceIDs = []string{}
	}

	return result, nil
}

// DiscoverASG is a lightweight lookup that only resolves the active ASG name.
// Used by health_check and maintenance_policy resources during Read.
func DiscoverASG(ctx context.Context, clients *awsclient.Clients, asgName string) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	output, err := clients.AutoScaling.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{asgName},
	})
	if err != nil {
		return nil, fmt.Errorf("[ebhelper] DescribeAutoScalingGroups failed for %q: %w", asgName, err)
	}
	if len(output.AutoScalingGroups) == 0 {
		return nil, fmt.Errorf("[ebhelper] ASG %q not found", asgName)
	}
	return output, nil
}

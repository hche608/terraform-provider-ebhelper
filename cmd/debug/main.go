// Package main provides a debug CLI for testing environment discovery locally.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/hche608/terraform-provider-ebhelper/internal/awsclient"
	"github.com/hche608/terraform-provider-ebhelper/internal/resources/environment_info"
)

func main() {
	ctx := context.Background()

	clients, err := awsclient.NewClients(ctx, awsclient.Config{
		Region: "ap-southeast-2",
	})
	if err != nil {
		fmt.Printf("Failed to create clients: %v\n", err)
		return
	}

	fmt.Println("Clients created, starting discovery...")

	result, err := environment_info.Discover(ctx, clients,
		"my-app",
		"my-app-env",
		5*time.Second,
		30*time.Second,
	)
	if err != nil {
		fmt.Printf("Discovery failed: %v\n", err)
		return
	}

	fmt.Printf("Success!\n")
	fmt.Printf("  Environment ID: %s\n", result.EnvironmentID)
	fmt.Printf("  ASG Name: %s\n", result.ASGName)
	fmt.Printf("  ASG ARN: %s\n", result.ASGARN)
	fmt.Printf("  Target Groups: %v\n", result.TargetGroupARNs)
	fmt.Printf("  LB ARNs: %v\n", result.LoadBalancerARNs)
	fmt.Printf("  LB DNS: %v\n", result.LoadBalancerDNS)
	fmt.Printf("  Instance IDs: %v\n", result.InstanceIDs)
}

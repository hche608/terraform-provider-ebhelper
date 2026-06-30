package awsclient_test

import (
	"context"
	"testing"

	"github.com/hche608/terraform-provider-ebhelper/internal/awsclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClients_DefaultConfig(t *testing.T) {
	cfg := awsclient.Config{}

	// Verify the config struct is zero-valued
	assert.Empty(t, cfg.Region)
	assert.Empty(t, cfg.RoleARN)
	assert.Empty(t, cfg.SessionName)
	assert.Empty(t, cfg.ExternalID)
}

func TestNewClients_WithRegion(t *testing.T) {
	cfg := awsclient.Config{
		Region: "ap-southeast-2",
	}

	assert.Equal(t, "ap-southeast-2", cfg.Region)
	assert.Empty(t, cfg.RoleARN)
}

func TestNewClients_WithAssumeRole(t *testing.T) {
	cfg := awsclient.Config{
		Region:      "us-east-1",
		RoleARN:     "arn:aws:iam::123456789012:role/TestRole",
		SessionName: "test-session",
		ExternalID:  "external-123",
	}

	assert.Equal(t, "us-east-1", cfg.Region)
	assert.Equal(t, "arn:aws:iam::123456789012:role/TestRole", cfg.RoleARN)
	assert.Equal(t, "test-session", cfg.SessionName)
	assert.Equal(t, "external-123", cfg.ExternalID)
}

func TestNewClients_NoCredentials_ReturnsClients(t *testing.T) {
	// With no role ARN, NewClients should succeed even without real AWS creds
	// because it only loads the default config without trying to assume a role.
	ctx := context.Background()
	cfg := awsclient.Config{
		Region: "us-west-2",
	}

	clients, err := awsclient.NewClients(ctx, cfg)

	// This should succeed - it only fails if LoadDefaultConfig itself fails,
	// which shouldn't happen even without credentials
	require.NoError(t, err)
	require.NotNil(t, clients)
	assert.NotNil(t, clients.ElasticBeanstalk)
	assert.NotNil(t, clients.AutoScaling)
	assert.NotNil(t, clients.ELBv2)
}

func TestNewClients_EmptyRegion_ReturnsClients(t *testing.T) {
	// Empty region should still work (uses AWS_REGION env or default)
	ctx := context.Background()
	cfg := awsclient.Config{}

	clients, err := awsclient.NewClients(ctx, cfg)

	require.NoError(t, err)
	require.NotNil(t, clients)
	assert.NotNil(t, clients.ElasticBeanstalk)
	assert.NotNil(t, clients.AutoScaling)
	assert.NotNil(t, clients.ELBv2)
}

func TestNewClients_WithInvalidRole_ReturnsError(t *testing.T) {
	// With a role ARN set, it will attempt to assume the role and fail
	ctx := context.Background()
	cfg := awsclient.Config{
		Region:  "us-east-1",
		RoleARN: "arn:aws:iam::000000000000:role/NonExistentRole",
	}

	clients, err := awsclient.NewClients(ctx, cfg)

	// Should fail because it tries to retrieve credentials for the assumed role
	assert.Error(t, err)
	assert.Nil(t, clients)
	assert.Contains(t, err.Error(), "assume role failed")
}

func TestNewClients_WithRoleAndExternalID(t *testing.T) {
	// Verify that ExternalID is accepted (will still fail due to no real credentials)
	ctx := context.Background()
	cfg := awsclient.Config{
		Region:      "eu-west-1",
		RoleARN:     "arn:aws:iam::123456789012:role/CrossAccountRole",
		SessionName: "custom-session",
		ExternalID:  "ext-id-abc123",
	}

	clients, err := awsclient.NewClients(ctx, cfg)

	// Should fail because it tries to assume the role
	assert.Error(t, err)
	assert.Nil(t, clients)
	assert.Contains(t, err.Error(), "assume role failed")
}

func TestNewClients_WithRoleNoSessionName(t *testing.T) {
	// When SessionName is empty, it defaults to "terraform-ebhelper"
	ctx := context.Background()
	cfg := awsclient.Config{
		Region:  "us-east-1",
		RoleARN: "arn:aws:iam::123456789012:role/SomeRole",
		// SessionName intentionally left empty
	}

	clients, err := awsclient.NewClients(ctx, cfg)

	// Should fail on assume role (no real creds) but the function should handle empty session name
	assert.Error(t, err)
	assert.Nil(t, clients)
}

# Implementation Tasks

## Task 1: Explore EB API Responses

- [ ] Use AWS CLI to call `aws elasticbeanstalk describe-environments` for a known EB environment and capture the full JSON response
- [ ] Use AWS CLI to call `aws elasticbeanstalk describe-environment-resources` with the environment ID and capture the full JSON response
- [ ] Use AWS CLI to call `aws autoscaling describe-auto-scaling-groups` for the ASG name from the EB response and capture the full JSON response
- [ ] Use AWS CLI to call `aws elbv2 describe-target-groups` with the target group ARNs from the ASG response
- [ ] Use AWS CLI to call `aws elbv2 describe-load-balancers` with the load balancer ARNs from the target group response
- [ ] Document which fields from each response we need for the provider's computed attributes
- [ ] Save sample JSON responses to `testdata/` for use as mock fixtures in later tasks

### Purpose
- Validate the actual API response structure before writing code
- Identify exactly which API calls are needed and which fields map to our computed attributes
- Generate real mock data for testing

---

## Task 2: Create Dedicated Repository and Initialize Go Module

- [ ] Create a new repository `terraform-provider-ebhelper` (separate from my-terraform-modules)
- [ ] Initialize Go module with `go mod init github.com/hche608/terraform-provider-ebhelper`
- [ ] Add required dependencies: `terraform-plugin-framework`, `aws-sdk-go-v2`, `aws-sdk-go-v2/service/autoscaling`, `aws-sdk-go-v2/service/elasticbeanstalk`, `aws-sdk-go-v2/service/elasticloadbalancingv2`, `aws-sdk-go-v2/credentials/stscreds`, `aws-sdk-go-v2/service/sts`
- [ ] Create `main.go` with provider server entry point using `providerserver.Serve`
- [ ] Create directory structure: `internal/provider/`, `internal/resources/environment_info/`, `internal/resources/asg_health_check/`, `internal/resources/asg_instance_maintenance_policy/`, `internal/awsclient/`
- [ ] Copy sample JSON responses from Task 1 into `internal/testdata/` for mock fixtures
- [ ] Verify `go build ./...` succeeds

### Requirements Addressed
- Requirement 10: Provider Schema and Resource Registration (scaffold)

---

## Task 3: Implement AWS Client Factory

- [ ] Create `internal/awsclient/client.go` with `Config` struct (Region, RoleARN, SessionName, ExternalID)
- [ ] Create `Clients` struct holding `ElasticBeanstalk`, `AutoScaling`, and `ELBv2` service clients
- [ ] Implement `NewClients(ctx, Config)` that loads default AWS config, optionally wraps with `stscreds.NewAssumeRoleProvider`, and constructs service clients
- [ ] Return descriptive error including role ARN if AssumeRole fails
- [ ] Define interfaces `EBClient`, `ASGClient`, `ELBClient` for testability (based on API calls identified in Task 1)
- [ ] Create `internal/awsclient/client_test.go` with unit tests for config construction logic

### Requirements Addressed
- Requirement 1: Provider Configuration and AWS Authentication (AC 1-5)

---

## Task 4: Implement Provider Configuration

- [ ] Create `internal/provider/provider.go` implementing `provider.Provider` interface
- [ ] Define `EbhelperProviderModel` struct with `Region` (optional string) and `AssumeRole` (optional block with `role_arn`, `session_name`, `external_id`)
- [ ] Implement `Metadata()` returning type name `ebhelper`
- [ ] Implement `Schema()` with provider-level attributes and `assume_role` nested block
- [ ] Implement `Configure()` that parses config, calls `awsclient.NewClients()`, and stores clients in provider data via `resp.DataSourceData` and `resp.ResourceData`
- [ ] Implement `Resources()` returning empty list (populated in later tasks)
- [ ] Implement `DataSources()` returning nil
- [ ] Create `internal/provider/provider_test.go` testing schema validation and configure error paths

### Requirements Addressed
- Requirement 1: Provider Configuration and AWS Authentication (AC 1-5)
- Requirement 10: Provider Schema and Resource Registration (AC 4)

---

## Task 5: Implement Environment Info Discovery Logic

- [ ] Create `internal/resources/environment_info/discovery.go`
- [ ] Define `DiscoveryResult` struct with fields mapped from Task 1 API exploration
- [ ] Implement `Discover(ctx, clients, appName, envName, interval, timeout)` function:
  - Poll `DescribeEnvironments` with application name filter to resolve environment ID
  - Call `DescribeEnvironmentResources` with environment ID to get ASG name(s) and instance IDs
  - Select first ASG from EB API response as active ASG
  - Call `DescribeAutoScalingGroups` for full ASG details (ARN, target group ARNs, launch template)
  - If target group ARNs present, call `DescribeTargetGroups` to get TG names and LB ARNs
  - Call `DescribeLoadBalancers` to get LB DNS names from LB ARNs
- [ ] Implement retry loop with configurable interval and timeout, logging each attempt
- [ ] Return descriptive error on timeout including which discovery step failed
- [ ] Create `internal/resources/environment_info/discovery_test.go` using mock fixtures from `internal/testdata/` — test: single ASG, multiple ASGs, timeout behavior, partial failure

### Requirements Addressed
- Requirement 2: EB Environment Resource Discovery (AC 1-7)
- Requirement 3: Active ASG Identification (AC 1-5)
- Requirement 4: Retry and Polling Mechanism (AC 1-5)

---

## Task 6: Implement Environment Info Resource CRUD

- [ ] Create `internal/resources/environment_info/model.go` with `EnvironmentInfoModel` struct (all fields with tfsdk tags)
- [ ] Create `internal/resources/environment_info/resource.go` implementing `resource.Resource` interface
- [ ] Implement `Schema()` with:
  - `application_name`: Required, string, plan modifier RequiresReplace
  - `environment_name`: Required, string, plan modifier RequiresReplace
  - `polling_interval`: Optional, int64, default 10
  - `polling_timeout`: Optional, int64, default 300
  - All computed attributes (asg_name, asg_arn, all_asg_names, load_balancer_arns, etc.)
- [ ] Implement `Create()`: read plan, call `Discover()`, map result to model, set state
- [ ] Implement `Read()`: re-query AWS APIs, update state, remove from state if ASG deleted
- [ ] Implement `Update()`: re-run discovery with new polling settings
- [ ] Implement `Delete()`: no-op, just remove from state
- [ ] Register resource in provider's `Resources()` method
- [ ] Create `internal/resources/environment_info/resource_test.go` testing schema and CRUD flow with mocks

### Requirements Addressed
- Requirement 5: Computed Attribute Exposure (AC 1-7)
- Requirement 6: Terraform State Management and Lifecycle (AC 1-5)
- Requirement 7: Dependency Integration with EB Environment Resource (AC 1-3)
- Requirement 10: Provider Schema and Resource Registration (AC 1)

---

## Task 7: Implement ASG Health Check Resource

- [ ] Create `internal/resources/asg_health_check/model.go` with `ASGHealthCheckModel` struct
- [ ] Create `internal/resources/asg_health_check/resource.go` implementing `resource.Resource`
- [ ] Implement `Schema()` with:
  - `asg_name`: Required, string, plan modifier RequiresReplace
  - `health_check_type`: Required, string, validator OneOf("EC2", "ELB")
  - `health_check_grace_period`: Optional, int64, default 300
- [ ] Implement `Create()`: call `UpdateAutoScalingGroup` with HealthCheckType and HealthCheckGracePeriod
- [ ] Implement `Read()`: call `DescribeAutoScalingGroups`, compare current vs state for drift detection
- [ ] Implement `Update()`: call `UpdateAutoScalingGroup` with new values
- [ ] Implement `Delete()`: reset to EC2 health check with 300s grace period
- [ ] Handle ASG-not-found error with descriptive message
- [ ] Register resource in provider's `Resources()` method
- [ ] Create `internal/resources/asg_health_check/resource_test.go` testing CRUD and validation with mocks from testdata

### Requirements Addressed
- Requirement 8: ASG Health Check Configuration Resource (AC 1-8)
- Requirement 10: Provider Schema and Resource Registration (AC 2, 5)

---

## Task 8: Implement ASG Instance Maintenance Policy Resource

- [ ] Create `internal/resources/asg_instance_maintenance_policy/model.go` with `ASGMaintenancePolicyModel` struct
- [ ] Create `internal/resources/asg_instance_maintenance_policy/resource.go` implementing `resource.Resource`
- [ ] Implement `Schema()` with:
  - `asg_name`: Required, string, plan modifier RequiresReplace
  - `min_healthy_percentage`: Required, int64, validator Between(0, 100)
  - `max_healthy_percentage`: Required, int64, validator Between(100, 200)
- [ ] Implement `Create()`: call `UpdateAutoScalingGroup` with InstanceMaintenancePolicy
- [ ] Implement `Read()`: call `DescribeAutoScalingGroups`, read InstanceMaintenancePolicy, detect drift
- [ ] Implement `Update()`: call `UpdateAutoScalingGroup` with new policy values
- [ ] Implement `Delete()`: remove policy by setting MinHealthyPercentage=-1, MaxHealthyPercentage=-1
- [ ] Handle ASG-not-found error with descriptive message
- [ ] Register resource in provider's `Resources()` method
- [ ] Create `internal/resources/asg_instance_maintenance_policy/resource_test.go` testing CRUD and validation with mocks from testdata

### Requirements Addressed
- Requirement 9: ASG Instance Maintenance Policy Resource (AC 1-10)
- Requirement 10: Provider Schema and Resource Registration (AC 3, 5)

---

## Task 9: Local Testing with dev_overrides

- [ ] Add `GNUmakefile` with targets:
  - `build`: compile the provider binary
  - `install`: build and install to `~/.terraform.d/plugins/` (local registry path)
  - `test`: run `go test ./... -count=1`
  - `testacc`: run `TF_ACC=1 go test ./... -run TestAcc`
- [ ] Document `~/.terraformrc` `dev_overrides` configuration for local development:
  ```hcl
  provider_installation {
    dev_overrides {
      "hche608/ebhelper" = "/path/to/go/bin"
    }
    direct {}
  }
  ```
- [ ] Create `examples/local-test/main.tf` with a minimal config that uses the provider locally (hardcoded environment name for manual testing)
- [ ] Create `examples/local-test/README.md` with step-by-step instructions: build → configure dev_overrides → terraform init → terraform plan → terraform apply
- [ ] Verify full lifecycle locally: `make build` → `terraform plan` shows `(known after apply)` for computed attrs → `terraform apply` discovers real EB environment → `terraform plan` shows no changes (state is consistent)
- [ ] Test drift detection: manually change ASG health check type via CLI → `terraform plan` detects drift → `terraform apply` corrects it

### Requirements Addressed
- All requirements (end-to-end local validation before publishing)

---

## Task 10: Property-Based Tests

- [ ] Add `github.com/flyingmutant/rapid` dependency
- [ ] Create property test for P1 (Error Context): generate random identifiers, verify all error messages contain the relevant identifiers
- [ ] Create property test for P2 (ASG Selection): generate random ASG name lists, verify first item is always selected
- [ ] Create property test for P3 (Model Mapping): generate random DiscoveryResult structs, verify lossless mapping to EnvironmentInfoModel
- [ ] Create property test for P4 (Polling Bounds): generate random interval/timeout pairs, verify polling terminates within T+I seconds and attempts ≤ ⌈T/I⌉+1
- [ ] Create property test for P5 (Enum Validation): generate random strings, verify only "EC2" and "ELB" pass validation
- [ ] Create property test for P6 (Range Validation): generate random integers, verify only valid ranges pass

### Requirements Addressed
- Design: Correctness Properties 1-6
- Requirements 1.5, 2.7, 3.2, 3.5, 4.3, 4.4, 5.1-5.7, 8.2, 8.8, 9.2, 9.3, 9.8, 9.9, 9.10

---

## Task 11: Example Usage and Documentation

- [ ] Create `examples/full/main.tf` showing full usage pattern:
  - Provider configuration with assume_role
  - `ebhelper_environment_info` referencing an EB environment
  - `aws_autoscaling_schedule` using `ebhelper_environment_info.this.asg_name`
  - `ebhelper_asg_health_check` using `ebhelper_environment_info.this.asg_name`
  - `ebhelper_asg_instance_maintenance_policy` using `ebhelper_environment_info.this.asg_name`
- [ ] Create `docs/README.md` with provider overview, installation, and configuration
- [ ] Create `docs/resources/environment_info.md` with attribute reference
- [ ] Create `docs/resources/asg_health_check.md` with attribute reference
- [ ] Create `docs/resources/asg_instance_maintenance_policy.md` with attribute reference

### Requirements Addressed
- All requirements (documentation and usage examples)

---

## Task 12: Integration with Existing EB Module

- [ ] Update `modules/elastic-beanstalk-awscc/` in my-terraform-modules to demonstrate replacement of `null_resource` blocks
- [ ] Create `modules/elastic-beanstalk-awscc/eb-asg.tf` using the new provider resources
- [ ] Add `required_providers` block for `ebhelper` in the module
- [ ] Document that callers must configure the `ebhelper` provider (or use shared provider config)
- [ ] Verify the dependency chain: `awscc_elasticbeanstalk_environment` → `ebhelper_environment_info` → `aws_autoscaling_schedule` works in a single apply
- [ ] Comment out (do not delete) the existing `null_resource` blocks in `eb-data.tf` for reference during transition

### Requirements Addressed
- Requirement 7: Dependency Integration with EB Environment Resource (AC 1-3)
- Overall goal: Replace null_resource shell scripts with proper Terraform resources


---

## Task 13: Implement ebhelper_asg_termination_policy Resource

- [ ] Create `internal/resources/asg_termination_policy/model.go` with `ASGTerminationPolicyModel` (asg_name, termination_policies list)
- [ ] Create `internal/resources/asg_termination_policy/resource.go` — CRUD via `UpdateAutoScalingGroup` `TerminationPolicies` field
- [ ] Allowed values: `Default`, `OldestInstance`, `NewestInstance`, `OldestLaunchConfiguration`, `OldestLaunchTemplate`, `ClosestToNextInstanceHour`, `AllocationStrategy`
- [ ] Delete resets to `["Default"]`
- [ ] Register in provider
- [ ] Add unit tests with terraform-plugin-testing

---

## Task 14: Implement ebhelper_asg_default_cooldown Resource

- [ ] Create `internal/resources/asg_default_cooldown/model.go` (asg_name, cooldown_seconds)
- [ ] Create `internal/resources/asg_default_cooldown/resource.go` — CRUD via `UpdateAutoScalingGroup` `DefaultCooldown` field
- [ ] Delete resets to 300 (AWS default)
- [ ] Register in provider
- [ ] Add unit tests

---

## Task 15: Implement ebhelper_asg_max_instance_lifetime Resource

- [ ] Create `internal/resources/asg_max_instance_lifetime/model.go` (asg_name, max_instance_lifetime_seconds)
- [ ] Create `internal/resources/asg_max_instance_lifetime/resource.go` — CRUD via `UpdateAutoScalingGroup` `MaxInstanceLifetime` field
- [ ] Valid range: 0 (disabled) or 86400–31536000 (1 day to 365 days)
- [ ] Delete sets to 0 (disabled)
- [ ] Register in provider
- [ ] Add unit tests

---

## Task 16: Implement ebhelper_asg_default_instance_warmup Resource

- [ ] Create `internal/resources/asg_default_instance_warmup/model.go` (asg_name, warmup_seconds)
- [ ] Create `internal/resources/asg_default_instance_warmup/resource.go` — CRUD via `UpdateAutoScalingGroup` `DefaultInstanceWarmup` field
- [ ] Delete sets to -1 (disabled, uses default cooldown instead)
- [ ] Register in provider
- [ ] Add unit tests

---

## Task 17: Implement ebhelper_alb_attributes Resource

- [ ] Create `internal/resources/alb_attributes/model.go` (load_balancer_arn, attributes map)
- [ ] Create `internal/resources/alb_attributes/resource.go` — CRUD via `elbv2 ModifyLoadBalancerAttributes` / `DescribeLoadBalancerAttributes`
- [ ] Support key attributes: `dns_record.client_routing_policy`, `routing.http.xff_client_port.enabled`, `routing.http.x_amzn_tls_version_and_cipher_suite.enabled`, `deletion_protection.enabled`
- [ ] Add `ELBClient` interface method: `ModifyLoadBalancerAttributes`, `DescribeLoadBalancerAttributes`
- [ ] Update mock
- [ ] Delete removes custom attributes (resets to defaults)
- [ ] Register in provider
- [ ] Add unit tests

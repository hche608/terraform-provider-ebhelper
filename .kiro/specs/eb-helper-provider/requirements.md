# Requirements Document

## Introduction

A custom Terraform provider called "ebhelper" that replaces fragile `null_resource` + `local-exec` shell scripts for managing AWS Elastic Beanstalk environment infrastructure. The provider solves two problems:

1. **Chicken-and-egg discovery**: Native Terraform `data` sources fail on first apply because EB-created resources (ASGs, target groups) don't exist yet. The provider implements a managed resource (`ebhelper_environment_info`) that behaves like a deferred data source — executing discovery during the apply phase with retry/polling.

2. **ASG configuration gaps**: Some ASG settings (health check type, instance maintenance policy) have no native Terraform resource that works with EB-managed ASGs. The provider offers managed resources (`ebhelper_asg_health_check`, `ebhelper_asg_instance_maintenance_policy`) with full state tracking and drift detection.

The provider is written in Go using the Terraform Plugin Framework and replaces shell scripts that suffered from no state tracking, no drift detection, fragile shell dependencies, no rollback capability, and temp file usage.

## Glossary

- **Provider**: The custom Terraform provider binary (`terraform-provider-ebhelper`) that registers resource types and handles provider-level configuration such as AWS credentials and role assumption
- **Environment_Info_Resource**: The `ebhelper_environment_info` Terraform managed resource that discovers and exposes all EB environment infrastructure details during the apply phase
- **Health_Check_Resource**: The `ebhelper_asg_health_check` Terraform managed resource that configures the health check type and grace period on an EB-managed ASG
- **Maintenance_Policy_Resource**: The `ebhelper_asg_instance_maintenance_policy` Terraform managed resource that configures the instance maintenance policy on an EB-managed ASG
- **ASG**: An AWS Auto Scaling Group created and managed by Elastic Beanstalk as part of the environment infrastructure
- **Active_ASG**: The primary ASG associated with an EB environment as reported by the Elastic Beanstalk DescribeEnvironmentResources API (as opposed to temporary ASGs created during immutable deployments)
- **Environment_ID**: The unique Elastic Beanstalk environment identifier (format `e-xxxxxxxxxx`) resolved by calling DescribeEnvironments with the application name and environment name
- **Application_Name**: The name of the Elastic Beanstalk application that owns the target environment, used together with environment name to locate the environment via the DescribeEnvironments API
- **Immutable_Deployment**: An Elastic Beanstalk deployment strategy that creates a new temporary ASG with new instances, then swaps them into the original ASG upon successful health checks
- **Target_Group**: An AWS Elastic Load Balancing target group that routes traffic to instances in the ASG
- **Load_Balancer**: An AWS Application Load Balancer or Network Load Balancer that distributes traffic to the environment's instances
- **EB_Environment**: An AWS Elastic Beanstalk environment that provisions and manages application infrastructure including ASGs, load balancers, and target groups
- **Polling_Mechanism**: A retry loop with configurable timeout and interval that repeatedly queries the AWS API until the EB environment resources become available or the timeout is exceeded
- **Computed_Attribute**: A Terraform resource attribute whose value is determined during the apply phase rather than at plan time, shown as `(known after apply)` in plan output
- **Launch_Template**: The EC2 launch template created by Elastic Beanstalk that defines the instance configuration for the ASG
- **Health_Check_Type**: The type of health check performed on ASG instances — either `EC2` (default, instance status checks only) or `ELB` (load balancer health check endpoint)
- **Instance_Maintenance_Policy**: An ASG policy that controls instance replacement behavior during updates by specifying minimum and maximum healthy percentage thresholds

## Requirements

### Requirement 1: Provider Configuration and AWS Authentication

**User Story:** As a Terraform practitioner, I want to configure the ebhelper provider with AWS credentials and role assumption, so that it can authenticate to the correct AWS account to discover EB environment resources.

#### Acceptance Criteria

1. THE Provider SHALL accept an optional `region` attribute specifying the AWS region for API calls
2. THE Provider SHALL accept an optional `assume_role` block containing `role_arn`, `session_name`, and `external_id` attributes for cross-account access
3. WHEN an `assume_role` block is provided, THE Provider SHALL perform an AWS STS AssumeRole operation using the specified `role_arn` before making any AWS API calls
4. WHEN no explicit credentials are configured, THE Provider SHALL use the default AWS credential chain (environment variables, shared credentials file, instance profile)
5. IF the STS AssumeRole operation fails, THEN THE Provider SHALL return a descriptive error including the role ARN and the underlying AWS error message

### Requirement 2: EB Environment Resource Discovery

**User Story:** As a Terraform practitioner, I want the provider to discover all infrastructure associated with my EB environment using the application name and environment name, so that I can reference ASG, load balancer, and target group attributes without relying on shell scripts.

#### Acceptance Criteria

1. THE Environment_Info_Resource SHALL accept a required `application_name` string attribute identifying the Elastic Beanstalk application
2. THE Environment_Info_Resource SHALL accept a required `environment_name` string attribute identifying the target EB_Environment
3. WHEN `application_name` and `environment_name` are provided, THE Environment_Info_Resource SHALL call the Elastic Beanstalk DescribeEnvironments API to resolve the Environment_ID for the specified application and environment combination
4. THE Environment_Info_Resource SHALL call the Elastic Beanstalk DescribeEnvironmentResources API using the resolved Environment_ID to retrieve the complete list of resources associated with the environment
5. THE Environment_Info_Resource SHALL query the Auto Scaling DescribeAutoScalingGroups API to retrieve full ASG details including target group ARNs for the discovered Active_ASG
6. WHEN the Active_ASG has target group ARNs, THE Environment_Info_Resource SHALL query the ELB DescribeTargetGroups API to retrieve target group details and resolve associated load balancer ARNs
7. IF no environment is found matching the provided `application_name` and `environment_name` after the Polling_Mechanism exhausts its timeout, THEN THE Environment_Info_Resource SHALL return an error stating the application name, environment name, and the timeout duration

### Requirement 3: Active ASG Identification

**User Story:** As a Terraform practitioner, I want the provider to reliably identify the active ASG using the Elastic Beanstalk DescribeEnvironmentResources API, so that downstream resources always target the correct production ASG even during immutable deployments.

#### Acceptance Criteria

1. THE Environment_Info_Resource SHALL use the Environment_ID resolved from the `application_name` and `environment_name` inputs to call the Elastic Beanstalk DescribeEnvironmentResources API
2. THE Environment_Info_Resource SHALL use the first ASG returned by DescribeEnvironmentResources as the Active_ASG
3. WHEN multiple ASGs are tagged with the environment name, THE Environment_Info_Resource SHALL log a diagnostic message indicating the count of ASGs found and which one was selected as active via the EB API
4. THE Environment_Info_Resource SHALL expose an `all_asg_names` computed list attribute containing the names of all ASGs tagged with the environment name for debugging purposes
5. IF the DescribeEnvironmentResources API returns no ASG for the resolved environment after the Polling_Mechanism exhausts its timeout, THEN THE Environment_Info_Resource SHALL return an error stating the environment name and that no ASG was found

### Requirement 4: Retry and Polling Mechanism

**User Story:** As a Terraform practitioner, I want the provider to retry environment and resource discovery with configurable timeouts, so that it handles the delay between EB environment creation and resource availability.

#### Acceptance Criteria

1. THE Environment_Info_Resource SHALL accept an optional `polling_interval` attribute with a default value of 10 seconds specifying the time between discovery retry attempts
2. THE Environment_Info_Resource SHALL accept an optional `polling_timeout` attribute with a default value of 300 seconds specifying the maximum total time to wait for successful resource discovery
3. WHILE the Polling_Mechanism is active, THE Environment_Info_Resource SHALL retry the full discovery sequence (DescribeEnvironments, DescribeEnvironmentResources, DescribeAutoScalingGroups) at the configured `polling_interval` until all required resources are found or the `polling_timeout` is exceeded
4. IF the `polling_timeout` is exceeded without successfully completing resource discovery, THEN THE Environment_Info_Resource SHALL return an error stating the timeout duration, the environment name, and which step of discovery failed
5. WHILE the Polling_Mechanism is active, THE Environment_Info_Resource SHALL log a diagnostic message at each retry attempt indicating the current attempt number and elapsed time

### Requirement 5: Computed Attribute Exposure

**User Story:** As a Terraform practitioner, I want the provider to expose all EB environment infrastructure details as computed attributes, so that I can reference ASG, load balancer, target group, and environment metadata in downstream Terraform resources.

#### Acceptance Criteria

1. THE Environment_Info_Resource SHALL expose the following ASG computed attributes: `asg_name` (string), `asg_arn` (string), and `all_asg_names` (list of strings)
2. THE Environment_Info_Resource SHALL expose the following load balancer computed attributes: `load_balancer_arns` (list of strings) and `load_balancer_dns_names` (list of strings)
3. THE Environment_Info_Resource SHALL expose the following target group computed attributes: `target_group_arns` (list of strings) and `target_group_names` (list of strings)
4. THE Environment_Info_Resource SHALL expose the following EB environment metadata attributes: `environment_id` (string), `endpoint_url` (string), `platform_arn` (string), and `health_status` (string)
5. THE Environment_Info_Resource SHALL expose the following instance attributes: `instance_ids` (list of strings) and `launch_template_id` (string)
6. WHEN the EB environment uses a shared load balancer, THE Environment_Info_Resource SHALL resolve the load balancer ARNs from the target group's `LoadBalancerArns` field via the ELB DescribeTargetGroups API
7. WHEN an attribute has no associated resource (e.g., a single-instance environment with no load balancer), THE Environment_Info_Resource SHALL set list attributes to an empty list and string attributes to an empty string

### Requirement 6: Terraform State Management and Lifecycle

**User Story:** As a Terraform practitioner, I want the resource to store discovered values in Terraform state and refresh them on subsequent applies, so that I get proper drift detection and plan output.

#### Acceptance Criteria

1. THE Environment_Info_Resource SHALL store all computed attributes in the Terraform state after a successful apply
2. WHEN Terraform executes a refresh operation, THE Environment_Info_Resource SHALL re-query the AWS APIs for current resource details and update the state accordingly
3. WHEN the `environment_name` or `application_name` input attribute changes, THE Environment_Info_Resource SHALL trigger a replacement (destroy and recreate) to discover the new environment's resources
4. THE Environment_Info_Resource SHALL implement the Terraform Read operation to detect state drift against actual AWS resources
5. WHEN a previously discovered ASG no longer exists during a refresh, THE Environment_Info_Resource SHALL remove the resource from state and report it as requiring recreation

### Requirement 7: Dependency Integration with EB Environment Resource

**User Story:** As a Terraform practitioner, I want to establish an explicit dependency between the ebhelper resource and the EB environment resource, so that the ASG discovery only runs after the EB environment is fully created.

#### Acceptance Criteria

1. THE Environment_Info_Resource SHALL support Terraform's `depends_on` meta-argument for establishing ordering dependencies
2. WHEN used with `depends_on` referencing an EB environment resource, THE Environment_Info_Resource SHALL defer its Create operation until the referenced resource completes its own Create operation
3. THE Environment_Info_Resource SHALL support Terraform's implicit dependency resolution through attribute references (e.g., using an EB environment output as the `environment_name` input)

### Requirement 8: ASG Health Check Configuration Resource

**User Story:** As a Terraform practitioner, I want a managed resource that configures the health check type on an EB-managed ASG, so that I get proper state tracking and drift detection for ELB health check settings that EB doesn't expose through Terraform.

#### Acceptance Criteria

1. THE Health_Check_Resource SHALL accept a required `asg_name` string attribute identifying the target Auto Scaling Group
2. THE Health_Check_Resource SHALL accept a required `health_check_type` string attribute with allowed values `EC2` or `ELB`
3. THE Health_Check_Resource SHALL accept an optional `health_check_grace_period` integer attribute with a default value of 300 seconds
4. WHEN created, THE Health_Check_Resource SHALL call the Auto Scaling UpdateAutoScalingGroup API to set the `HealthCheckType` and `HealthCheckGracePeriod` on the specified ASG
5. WHEN Terraform executes a refresh operation, THE Health_Check_Resource SHALL call DescribeAutoScalingGroups and compare the current health check type and grace period against the state to detect drift
6. WHEN the `health_check_type` or `health_check_grace_period` attribute changes, THE Health_Check_Resource SHALL call UpdateAutoScalingGroup to apply the new settings (in-place update, no replacement)
7. WHEN destroyed, THE Health_Check_Resource SHALL call UpdateAutoScalingGroup to reset the health check type to `EC2` with the default grace period of 300 seconds
8. IF the specified ASG does not exist, THEN THE Health_Check_Resource SHALL return an error stating the ASG name and that the ASG was not found

### Requirement 9: ASG Instance Maintenance Policy Resource

**User Story:** As a Terraform practitioner, I want a managed resource that configures the instance maintenance policy on an EB-managed ASG, so that I can control instance replacement behavior with proper state tracking and drift detection.

#### Acceptance Criteria

1. THE Maintenance_Policy_Resource SHALL accept a required `asg_name` string attribute identifying the target Auto Scaling Group
2. THE Maintenance_Policy_Resource SHALL accept a required `min_healthy_percentage` integer attribute specifying the minimum percentage of healthy instances during replacements (valid range: 0 to 100)
3. THE Maintenance_Policy_Resource SHALL accept a required `max_healthy_percentage` integer attribute specifying the maximum percentage of healthy instances during replacements (valid range: 100 to 200)
4. WHEN created, THE Maintenance_Policy_Resource SHALL call the Auto Scaling UpdateAutoScalingGroup API to set the `InstanceMaintenancePolicy` with the specified `MinHealthyPercentage` and `MaxHealthyPercentage`
5. WHEN Terraform executes a refresh operation, THE Maintenance_Policy_Resource SHALL call DescribeAutoScalingGroups and compare the current instance maintenance policy values against the state to detect drift
6. WHEN `min_healthy_percentage` or `max_healthy_percentage` attribute changes, THE Maintenance_Policy_Resource SHALL call UpdateAutoScalingGroup to apply the new policy (in-place update, no replacement)
7. WHEN destroyed, THE Maintenance_Policy_Resource SHALL call UpdateAutoScalingGroup to remove the instance maintenance policy by setting `MinHealthyPercentage` to -1 and `MaxHealthyPercentage` to -1
8. IF the specified ASG does not exist, THEN THE Maintenance_Policy_Resource SHALL return an error stating the ASG name and that the ASG was not found
9. IF `min_healthy_percentage` is greater than 100 or less than 0, THEN THE Maintenance_Policy_Resource SHALL return a validation error before making any API call
10. IF `max_healthy_percentage` is less than 100 or greater than 200, THEN THE Maintenance_Policy_Resource SHALL return a validation error before making any API call

### Requirement 10: Provider Schema and Resource Registration

**User Story:** As a Terraform practitioner, I want the provider to register all resource types with proper schema definitions, so that Terraform CLI can validate configurations and provide autocomplete.

#### Acceptance Criteria

1. THE Provider SHALL register the `ebhelper_environment_info` resource type with the Terraform Plugin Framework
2. THE Provider SHALL register the `ebhelper_asg_health_check` resource type with the Terraform Plugin Framework
3. THE Provider SHALL register the `ebhelper_asg_instance_maintenance_policy` resource type with the Terraform Plugin Framework
4. THE Provider SHALL implement the Terraform Plugin Framework provider interface with proper metadata returning `ebhelper` as the provider type name
5. THE Provider SHALL define schema validators for all required attributes that produce clear error messages when values are missing or invalid

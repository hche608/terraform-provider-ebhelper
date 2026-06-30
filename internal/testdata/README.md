# Test Data — AWS API Responses

Captured from a sandbox account (ap-southeast-2).

Two scenarios:
1. **Dedicated ALB** — `my-app-env` (EB creates and owns the ALB, pattern `awseb--*`)
2. **Shared ALB** — `my-webapp-env` (pre-existing ALB shared across environments, multiple target groups)

These JSON files serve as mock fixtures for provider unit/integration tests.

## Key Differences

| Aspect | Dedicated ALB | Shared ALB |
|--------|--------------|------------|
| LB name pattern | `awseb--AWSEB-*` (EB-created) | Custom name (pre-existing) |
| Target groups | 1 | 2+ (multi-port routing) |
| LB ownership | EB manages lifecycle | External, shared across envs |
| Instance Maintenance Policy | Not set | Set (min 90%, max 100%) |

## API Call Chain

```
DescribeEnvironments (app_name + env_name)
    → environment_id, endpoint_url, platform_arn, health_status
    
DescribeEnvironmentResources (environment_id)
    → ASG name(s), instance IDs, launch template ID, LB ARN (informational)

DescribeAutoScalingGroups (ASG name)
    → ASG ARN, target group ARNs, health check type/grace period,
      instance maintenance policy, launch template, instances

DescribeTargetGroups (target group ARNs)
    → TG names, LB ARNs (authoritative source for LB association)

DescribeLoadBalancers (LB ARNs)
    → DNS name, scheme, hosted zone ID
```

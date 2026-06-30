# terraform-provider-ebhelper

[![Tests](https://github.com/hche608/terraform-provider-ebhelper/actions/workflows/test.yml/badge.svg)](https://github.com/hche608/terraform-provider-ebhelper/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/hche608/terraform-provider-ebhelper)](https://goreportcard.com/report/github.com/hche608/terraform-provider-ebhelper)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

A Terraform provider that discovers AWS Elastic Beanstalk environment infrastructure and manages ASG properties that EB does not expose natively through Terraform.

## Why This Provider?

When managing Elastic Beanstalk with Terraform, you hit two problems:

1. **Chicken-and-egg discovery** — EB creates ASGs, target groups, and load balancers as side-effects. You can't reference them until the environment exists, and Terraform `data` sources fail on first apply because the resources don't exist yet during plan.

2. **ASG property gaps** — Some ASG settings (health check type, instance maintenance policy) can only be set on `aws_autoscaling_group`, which requires importing and managing the entire ASG lifecycle. EB would fight Terraform on every deployment.

This provider solves both:

- `ebhelper_environment_info` — a managed resource that discovers EB infrastructure **during apply** with polling/retry, then exposes attributes downstream
- `ebhelper_asg_health_check` / `ebhelper_asg_instance_maintenance_policy` — **ASG property patchers** that call `UpdateAutoScalingGroup` for specific fields without taking ownership of the full ASG

For settings that already have native standalone resources (`aws_autoscaling_schedule`, `aws_autoscaling_policy`, etc.), just use the discovered `asg_name` directly — no need for this provider.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.22 (for building from source)
- AWS credentials configured (standard credential chain)

## Installation

### From Source

```bash
git clone https://github.com/hche608/terraform-provider-ebhelper.git
cd terraform-provider-ebhelper
make install
```

This installs the binary to `~/.terraform.d/plugins/` for local use with `terraform init`.

### Using dev_overrides (skip `terraform init`)

Add to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/hche608/ebhelper" = "/path/to/terraform-provider-ebhelper"
  }
  direct {}
}
```

## Usage

```hcl
terraform {
  required_providers {
    ebhelper = {
      source  = "registry.terraform.io/hche608/ebhelper"
      version = "0.1.0"
    }
  }
}

provider "ebhelper" {
  region = "ap-southeast-2"

  # Optional: cross-account access
  assume_role {
    role_arn     = "arn:aws:iam::123456789012:role/CrossAccountRole"
    session_name = "terraform-ebhelper"
  }
}

# 1. Discover EB environment infrastructure
resource "ebhelper_environment_info" "app" {
  application_name = "my-app"
  environment_name = "my-app-env"
}

# 2. Configure ASG health check (ELB instead of default EC2)
resource "ebhelper_asg_health_check" "app" {
  asg_name                  = ebhelper_environment_info.app.asg_name
  health_check_type         = "ELB"
  health_check_grace_period = 300
}

# 3. Configure instance maintenance policy
resource "ebhelper_asg_instance_maintenance_policy" "app" {
  asg_name               = ebhelper_environment_info.app.asg_name
  min_healthy_percentage = 90
  max_healthy_percentage = 120  # Launch before terminate
}

# 4. Use discovered ASG name with native AWS resources
resource "aws_autoscaling_schedule" "scale_up" {
  autoscaling_group_name = ebhelper_environment_info.app.asg_name
  scheduled_action_name  = "scale-up-business-hours"
  recurrence             = "0 20 * * SUN-THU"
  desired_capacity       = 2
  min_size               = 2
  max_size               = 4
  time_zone              = "Pacific/Auckland"
}
```

## Resources

### ebhelper_environment_info

Discovers Elastic Beanstalk environment infrastructure during `terraform apply`. Acts as a deferred data source that resolves after the EB environment exists.

#### Inputs

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `application_name` | string | yes | — | EB application name |
| `environment_name` | string | yes | — | EB environment name |
| `polling_interval` | number | no | 10 | Seconds between discovery retries |
| `polling_timeout` | number | no | 300 | Max seconds to wait for discovery |

#### Outputs

| Name | Type | Description |
|------|------|-------------|
| `asg_name` | string | Active Auto Scaling Group name |
| `asg_arn` | string | ASG ARN |
| `all_asg_names` | list(string) | All ASGs (for debugging immutable deployments) |
| `target_group_arns` | list(string) | Target group ARNs attached to the ASG |
| `target_group_names` | list(string) | Target group names |
| `load_balancer_arns` | list(string) | Load balancer ARNs (de-duplicated) |
| `load_balancer_dns_names` | list(string) | Load balancer DNS names |
| `instance_ids` | list(string) | EC2 instance IDs in the environment |
| `environment_id` | string | EB environment ID (e-xxxxxxxxxx) |
| `endpoint_url` | string | Environment endpoint URL |
| `platform_arn` | string | Platform ARN |
| `health_status` | string | Environment health status |
| `cname` | string | Environment CNAME |
| `launch_template_id` | string | Launch template ID used by the ASG |

---

### ebhelper_asg_health_check

Configures the health check type and grace period on an EB-managed ASG. On destroy, resets to `EC2` with 300s grace period.

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `asg_name` | string | yes | — | ASG name (ForceNew) |
| `health_check_type` | string | yes | — | `EC2` or `ELB` |
| `health_check_grace_period` | number | no | 300 | Grace period in seconds |

---

### ebhelper_asg_instance_maintenance_policy

Configures the instance maintenance policy on an EB-managed ASG. On destroy, removes the policy.

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `asg_name` | string | yes | — | ASG name (ForceNew) |
| `min_healthy_percentage` | number | yes | — | 0–100 |
| `max_healthy_percentage` | number | yes | — | 100–200 (100 = terminate-then-launch, >100 = launch-before-terminate) |

## How It Works

```
EB Environment Created
        │
        ▼
ebhelper_environment_info (discovers ASG via EB API with polling)
        │
        ├──► asg_name ──► ebhelper_asg_health_check
        ├──► asg_name ──► ebhelper_asg_instance_maintenance_policy
        └──► asg_name ──► aws_autoscaling_schedule (native)
```

All in a **single `terraform apply`** — no two-phase apply needed.

## Development

```bash
# Build
make build

# Run tests
make test

# Install locally
make install

# Run acceptance tests (requires AWS credentials)
make testacc
```

### Project Structure

```
├── internal/
│   ├── awsclient/       # AWS client interfaces and factory
│   ├── mocks/           # testify/mock implementations
│   ├── provider/        # Provider definition and test helper
│   └── resources/
│       ├── environment_info/           # Discovery resource
│       ├── asg_health_check/           # Health check patcher
│       └── asg_instance_maintenance_policy/  # Maintenance policy patcher
├── examples/            # Example Terraform configurations
├── cmd/debug/           # CLI tool for testing discovery locally
├── main.go
└── GNUmakefile
```

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-06-30

### Added

- `ebhelper_environment_info` resource — discovers EB environment infrastructure (ASG, LB, target groups) during apply with configurable polling/retry
- `ebhelper_asg_health_check` resource — manages health check type (EC2/ELB) and grace period on EB-managed ASGs with drift detection
- `ebhelper_asg_instance_maintenance_policy` resource — manages instance maintenance policy (min/max healthy percentage) on EB-managed ASGs with drift detection
- Provider configuration with optional `region` and `assume_role` block for cross-account access
- Handles multiple ASGs from immutable deployments (selects active ASG via EB API)
- De-duplicates load balancer ARNs when multiple target groups point to the same ALB
- Full Terraform lifecycle: Create, Read (drift detection), Update, Delete (reset to defaults)
- Unit tests with testify/mock and terraform-plugin-testing framework
- CI pipeline via GitHub Actions
- Apache 2.0 license

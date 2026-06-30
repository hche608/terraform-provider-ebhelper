# Local Testing

## Prerequisites

1. Go installed (managed via mise)
2. AWS credentials configured for the target account
3. An existing Elastic Beanstalk environment

## Steps

### 1. Build and install the provider locally

```bash
cd /path/to/terraform-provider-ebhelper
make install
```

### 2. Configure dev_overrides (alternative to `make install`)

Add to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/hche608/ebhelper" = "/path/to/terraform-provider-ebhelper"
  }
  direct {}
}
```

When using `dev_overrides`, you don't need `terraform init` — Terraform uses the binary directly.

### 3. Run Terraform

```bash
cd examples/local-test

# With dev_overrides, skip init:
terraform plan
terraform apply

# Verify no drift:
terraform plan
```

### 4. Test drift detection

```bash
# Manually change health check type via CLI:
aws autoscaling update-auto-scaling-group \
  --auto-scaling-group-name "$(terraform output -raw asg_name)" \
  --health-check-type EC2

# Plan should detect drift:
terraform plan
# Expected: ebhelper_asg_health_check.ewb will be updated in-place

# Correct the drift:
terraform apply
```

### 5. Clean up

```bash
terraform destroy
```

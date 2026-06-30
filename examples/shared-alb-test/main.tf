terraform {
  required_providers {
    ebhelper = {
      source  = "registry.terraform.io/hche608/ebhelper"
      version = "0.1.0"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "ebhelper" {
  region = "ap-southeast-2"
}

provider "aws" {
  region = "ap-southeast-2"
}

# Discover shared ALB environment (multiple target groups)
resource "ebhelper_environment_info" "webapp" {
  application_name = "my-webapp"
  environment_name = "my-webapp-env"

  polling_interval = 10
  polling_timeout  = 60
}

# Health check on shared ALB environment
resource "ebhelper_asg_health_check" "webapp" {
  asg_name                  = ebhelper_environment_info.webapp.asg_name
  health_check_type         = "ELB"
  health_check_grace_period = 300
}

# Maintenance policy
resource "ebhelper_asg_instance_maintenance_policy" "webapp" {
  asg_name               = ebhelper_environment_info.webapp.asg_name
  min_healthy_percentage = 90
  max_healthy_percentage = 100
}

# Example: scheduled scaling with native aws_autoscaling_schedule
resource "aws_autoscaling_schedule" "scale_up" {
  autoscaling_group_name = ebhelper_environment_info.webapp.asg_name
  scheduled_action_name  = "scale-up-business-hours"
  recurrence             = "0 20 * * SUN-THU"
  desired_capacity       = 1
  min_size               = 1
  max_size               = 2
  time_zone              = "Pacific/Auckland"
}

resource "aws_autoscaling_schedule" "scale_down" {
  autoscaling_group_name = ebhelper_environment_info.webapp.asg_name
  scheduled_action_name  = "scale-down-after-hours"
  recurrence             = "0 8 * * MON-FRI"
  desired_capacity       = 1
  min_size               = 1
  max_size               = 1
  time_zone              = "Pacific/Auckland"
}

# Outputs to verify multi-TG and shared LB
output "environment_id" {
  value = ebhelper_environment_info.webapp.environment_id
}

output "asg_name" {
  value = ebhelper_environment_info.webapp.asg_name
}

output "target_group_arns" {
  description = "Should show 2 target groups (multi-port routing)"
  value       = ebhelper_environment_info.webapp.target_group_arns
}

output "target_group_names" {
  value = ebhelper_environment_info.webapp.target_group_names
}

output "load_balancer_arns" {
  description = "Should show 1 shared ALB (de-duplicated)"
  value       = ebhelper_environment_info.webapp.load_balancer_arns
}

output "load_balancer_dns" {
  value = ebhelper_environment_info.webapp.load_balancer_dns_names
}

output "instance_ids" {
  value = ebhelper_environment_info.webapp.instance_ids
}

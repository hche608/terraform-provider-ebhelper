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

# Discover EB environment infrastructure
resource "ebhelper_environment_info" "app" {
  application_name = "my-app"
  environment_name = "my-app-env"

  polling_interval = 10
  polling_timeout  = 300
}

# Use discovered ASG name with native Terraform resources
resource "ebhelper_asg_health_check" "app" {
  asg_name                  = ebhelper_environment_info.app.asg_name
  health_check_type         = "ELB"
  health_check_grace_period = 300
}

resource "ebhelper_asg_instance_maintenance_policy" "app" {
  asg_name               = ebhelper_environment_info.app.asg_name
  min_healthy_percentage = 90
  max_healthy_percentage = 100
}

# Example: Use discovered ASG name with native aws_autoscaling_schedule
# resource "aws_autoscaling_schedule" "scale_up" {
#   autoscaling_group_name = ebhelper_environment_info.app.asg_name
#   scheduled_action_name  = "scale-up-business-hours"
#   recurrence             = "0 20 * * SUN-THU"
#   desired_capacity       = 2
#   min_size               = 2
#   max_size               = 4
#   time_zone              = "Pacific/Auckland"
# }

# Outputs for verification
output "environment_id" {
  value = ebhelper_environment_info.app.environment_id
}

output "asg_name" {
  value = ebhelper_environment_info.app.asg_name
}

output "target_group_arns" {
  value = ebhelper_environment_info.app.target_group_arns
}

output "load_balancer_dns" {
  value = ebhelper_environment_info.app.load_balancer_dns_names
}

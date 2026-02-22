# Base name for all ALB resources
variable "service_name" {
  description = "Base name for ALB resources"
  type        = string
}

# VPC ID where ALB and target group will reside
variable "vpc_id" {
  description = "VPC ID for the target group"
  type        = string
}

# Subnets for the ALB
variable "subnet_ids" {
  description = "Subnets for the ALB (needs at least 2 AZs)"
  type        = list(string)
}

# Security group to attach to the ALB
variable "security_group_id" {
  description = "Security group for the ALB"
  type        = string
}

# Port that ECS containers are listening on
variable "container_port" {
  description = "Port the containers listen on"
  type        = number
}
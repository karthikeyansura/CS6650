# Base name for ECS resources
variable "service_name" {
  type        = string
  description = "Base name for ECS resources"
}

# Container image URI in ECR
variable "image" {
  type        = string
  description = "ECR image URI (with tag)"
}

# Port that the container listens on
variable "container_port" {
  type        = number
  description = "Port your app listens on"
}

# Subnets to deploy Fargate tasks
variable "subnet_ids" {
  type        = list(string)
  description = "Subnets for FARGATE tasks"
}

# Security groups attached to Fargate tasks
variable "security_group_ids" {
  type        = list(string)
  description = "SGs for FARGATE tasks"
}

# ECS execution role (for pulling images, logging, etc.)
variable "execution_role_arn" {
  type        = string
  description = "ECS Task Execution Role ARN"
}

# Task role for app permissions
variable "task_role_arn" {
  type        = string
  description = "IAM Role ARN for app permissions"
}

# CloudWatch log group to store container logs
variable "log_group_name" {
  type        = string
  description = "CloudWatch log group name"
}

# Number of Fargate tasks to run
variable "ecs_count" {
  type        = number
  default     = 2
  description = "Desired Fargate task count"
}

# AWS region
variable "region" {
  type        = string
  description = "AWS region (for awslogs driver)"
}

# vCPU units for task definition
variable "cpu" {
  type        = string
  default     = "256"
  description = "vCPU units"
}

# Memory for task definition
variable "memory" {
  type        = string
  default     = "512"
  description = "Memory (MiB)"
}

# Target group ARN for ALB registration
variable "target_group_arn" {
  type        = string
  description = "ALB target group ARN for service registration"
}
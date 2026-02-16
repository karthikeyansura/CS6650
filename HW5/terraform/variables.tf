# AWS region to deploy all resources into
variable "aws_region" {
  type    = string
  default = "us-east-1"
}

# Name for the ECR repository
variable "ecr_repository_name" {
  type    = string
  default = "product-service"
}

# Base name used across ECS cluster, service, SG, and log group
variable "service_name" {
  type    = string
  default = "product-service"
}

# Port the Go app listens on inside the container
variable "container_port" {
  type    = number
  default = 8080
}

# Number of Fargate tasks to run
variable "ecs_count" {
  type    = number
  default = 1
}

# CloudWatch log retention period
variable "log_retention_days" {
  type    = number
  default = 7
}
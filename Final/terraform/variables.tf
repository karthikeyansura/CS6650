variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "us-west-2"
}

variable "project_name" {
  description = "Project name prefix for resource naming"
  type        = string
  default     = "album-store"
}

variable "environment" {
  description = "Deployment environment"
  type        = string
  default     = "prod"
}

variable "api_image" {
  description = "ECR image URI for the API service"
  type        = string
}

variable "worker_image" {
  description = "ECR image URI for the worker service"
  type        = string
}

variable "api_desired_count" {
  description = "Number of API ECS tasks"
  type        = number
  default     = 4
}

variable "worker_desired_count" {
  description = "Number of worker ECS tasks"
  type        = number
  default     = 6
}

variable "api_cpu" {
  description = "CPU units for API task (1024 = 1 vCPU)"
  type        = number
  default     = 512
}

variable "api_memory" {
  description = "Memory in MiB for API task"
  type        = number
  default     = 1024
}

variable "worker_cpu" {
  description = "CPU units for worker task"
  type        = number
  default     = 256
}

variable "worker_memory" {
  description = "Memory in MiB for worker task"
  type        = number
  default     = 512
}

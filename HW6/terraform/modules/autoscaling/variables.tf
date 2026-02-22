# Base name for auto scaling resources
variable "service_name" {
  description = "Base name for scaling resources"
  type        = string
}

# ECS cluster name
variable "ecs_cluster_name" {
  description = "Name of the ECS cluster"
  type        = string
}

# ECS service name
variable "ecs_service_name" {
  description = "Name of the ECS service"
  type        = string
}

# Minimum number of Fargate tasks
variable "min_capacity" {
  description = "Minimum number of tasks"
  type        = number
  default     = 2
}

# Maximum number of Fargate tasks
variable "max_capacity" {
  description = "Maximum number of tasks"
  type        = number
  default     = 4
}

# Target average CPU utilization for scaling
variable "cpu_target_value" {
  description = "Target CPU utilization percentage"
  type        = number
  default     = 70
}

# Cooldown period in seconds after scaling out
variable "scale_out_cooldown" {
  description = "Cooldown (seconds) after scale-out"
  type        = number
  default     = 300
}

# Cooldown period in seconds after scaling in
variable "scale_in_cooldown" {
  description = "Cooldown (seconds) after scale-in"
  type        = number
  default     = 300
}
variable "service_name" {
  description = "Base name for scaling resources"
  type        = string
}

variable "ecs_cluster_name" {
  description = "Name of the ECS cluster"
  type        = string
}

variable "ecs_service_name" {
  description = "Name of the ECS service"
  type        = string
}

variable "min_capacity" {
  description = "Minimum number of tasks"
  type        = number
  default     = 2
}

variable "max_capacity" {
  description = "Maximum number of tasks"
  type        = number
  default     = 4
}

variable "cpu_target_value" {
  description = "Target CPU utilization percentage"
  type        = number
  default     = 70
}

variable "scale_out_cooldown" {
  description = "Cooldown (seconds) after scale-out"
  type        = number
  default     = 300
}

variable "scale_in_cooldown" {
  description = "Cooldown (seconds) after scale-in"
  type        = number
  default     = 300
}

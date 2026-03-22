# AWS region to deploy all resources into
variable "aws_region" {
  type    = string
  default = "us-east-1"
}

# Name for the ECR repository
variable "ecr_repository_name" {
  type    = string
  default = "shopping-cart-service"
}

# Base name used across ECS cluster, service, SG, and log group
variable "service_name" {
  type    = string
  default = "shopping-cart"
}

# Port the Go app listens on inside the container
variable "container_port" {
  type    = number
  default = 8080
}

# Number of Fargate tasks to run
variable "ecs_count" {
  type    = number
  default = 2
}

# CloudWatch log retention period
variable "log_retention_days" {
  type    = number
  default = 7
}

# Minimum number of ECS tasks allowed by the autoscaling policy
variable "autoscaling_min" {
  description = "Minimum number of ECS tasks"
  type        = number
  default     = 2
}

# Maximum number of ECS tasks allowed by the autoscaling policy
variable "autoscaling_max" {
  description = "Maximum number of ECS tasks"
  type        = number
  default     = 4
}

# Target average CPU utilization percentage for ECS service autoscaling
variable "cpu_target_value" {
  description = "Target average CPU utilization for scaling policy"
  type        = number
  default     = 70
}

# Cooldown period (in seconds) after scaling out
variable "scale_out_cooldown" {
  description = "Seconds to wait after a scale-out before allowing another"
  type        = number
  default     = 300
}

# Cooldown period (in seconds) after scaling in
variable "scale_in_cooldown" {
  description = "Seconds to wait after a scale-in before allowing another"
  type        = number
  default     = 300
}

# ── RDS Variables ──

variable "db_username" {
  description = "RDS master username"
  type        = string
  default     = "admin"
}

variable "db_password" {
  description = "RDS master password"
  type        = string
  sensitive   = true
}

variable "db_name" {
  description = "Name of the MySQL database"
  type        = string
  default     = "shopping_cart_db"
}

# ── DynamoDB Variables ──

variable "dynamodb_table_name" {
  description = "Name of the DynamoDB table"
  type        = string
  default     = "shopping-carts"
}

# ── DB Mode ──

variable "db_mode" {
  description = "Database backend: 'mysql' or 'dynamodb'"
  type        = string
  default     = "mysql"

  validation {
    condition     = contains(["mysql", "dynamodb"], var.db_mode)
    error_message = "db_mode must be 'mysql' or 'dynamodb'"
  }
}

variable "service_name" {
  description = "Base name for ALB resources"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID for the target group"
  type        = string
}

variable "subnet_ids" {
  description = "Subnets for the ALB (needs at least 2 AZs)"
  type        = list(string)
}

variable "security_group_id" {
  description = "Security group for the ALB"
  type        = string
}

variable "container_port" {
  description = "Port the containers listen on"
  type        = number
}

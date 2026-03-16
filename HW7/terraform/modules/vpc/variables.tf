variable "service_name" {
  type        = string
  description = "Base name for VPC resources"
}

variable "container_port" {
  type        = number
  default     = 8080
  description = "Port ECS tasks expose"
}

variable "public_subnet_cidrs" {
  type    = list(string)
  default = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "private_subnet_cidrs" {
  type    = list(string)
  default = ["10.0.10.0/24", "10.0.11.0/24"]
}

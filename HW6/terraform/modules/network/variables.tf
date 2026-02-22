# Base name used to generate security group names
variable "service_name" {
  description = "Base name for SG"
  type        = string
}

# Port number that ECS tasks will expose
variable "container_port" {
  description = "Port to expose on the SG"
  type        = number
}

# List of CIDR blocks allowed to reach ECS tasks directly
variable "cidr_blocks" {
  description = "Which CIDRs can reach the service directly"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}
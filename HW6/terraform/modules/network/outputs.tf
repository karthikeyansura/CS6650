# ID of the default VPC used for all network resources
output "vpc_id" {
  description = "ID of the default VPC"
  value       = data.aws_vpc.default.id
}

# List of all subnet IDs in the default VPC
output "subnet_ids" {
  description = "IDs of the default VPC subnets"
  value       = data.aws_subnets.default.ids
}

# Security group ID for ECS tasks
output "security_group_id" {
  description = "Security group ID for ECS tasks"
  value       = aws_security_group.ecs.id
}

# Security group ID for the Application Load Balancer
output "alb_security_group_id" {
  description = "Security group ID for ALB"
  value       = aws_security_group.alb.id
}
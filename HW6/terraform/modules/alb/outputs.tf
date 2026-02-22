# DNS name of the ALB (used for client requests)
output "alb_dns_name" {
  description = "DNS name of the ALB"
  value       = aws_lb.this.dns_name
}

# ARN of the target group for ECS service registration
output "target_group_arn" {
  description = "ARN of the target group"
  value       = aws_lb_target_group.this.arn
}
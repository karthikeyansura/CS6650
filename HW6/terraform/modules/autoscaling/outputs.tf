# ARN of the CPU-based scaling policy
output "scaling_policy_arn" {
  description = "ARN of the CPU scaling policy"
  value       = aws_appautoscaling_policy.cpu.arn
}
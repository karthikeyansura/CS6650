output "task_role_arn" {
  description = "ARN of the ECS task role with DynamoDB access"
  value       = aws_iam_role.task_role.arn
}

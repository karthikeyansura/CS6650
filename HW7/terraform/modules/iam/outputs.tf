output "execution_role_arn" {
  value = data.aws_iam_role.execution_role.arn
}

output "receiver_task_role_arn" {
  value = aws_iam_role.receiver_task.arn
}

output "processor_task_role_arn" {
  value = aws_iam_role.processor_task.arn
}

output "lambda_role_arn" {
  value = aws_iam_role.lambda.arn
}

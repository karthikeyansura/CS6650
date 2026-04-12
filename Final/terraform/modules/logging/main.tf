variable "project_name" { type = string }

resource "aws_cloudwatch_log_group" "api" {
  name              = "/ecs/${var.project_name}-api"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "worker" {
  name              = "/ecs/${var.project_name}-worker"
  retention_in_days = 7
}

output "api_log_group_name" { value = aws_cloudwatch_log_group.api.name }
output "worker_log_group_name" { value = aws_cloudwatch_log_group.worker.name }

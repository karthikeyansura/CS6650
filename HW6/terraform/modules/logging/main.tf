# CloudWatch log group for ECS container logs
resource "aws_cloudwatch_log_group" "this" {
  name              = "/ecs/${var.service_name}"
  retention_in_days = var.retention_in_days
}
# Name of the created ECS cluster
output "cluster_name" {
  description = "ECS cluster name"
  value       = aws_ecs_cluster.this.name
}

# Name of the ECS service running Fargate tasks
output "service_name" {
  description = "ECS service name"
  value       = aws_ecs_service.this.name
}
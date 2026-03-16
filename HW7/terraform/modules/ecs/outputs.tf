output "cluster_name" {
  value = aws_ecs_cluster.this.name
}

output "receiver_service_name" {
  value = aws_ecs_service.receiver.name
}

output "processor_service_name" {
  value = aws_ecs_service.processor.name
}

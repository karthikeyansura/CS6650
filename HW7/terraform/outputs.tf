output "alb_dns_name" {
  description = "DNS name of the ALB (use for Locust target host)"
  value       = module.alb.alb_dns_name
}

output "ecs_cluster_name" {
  value = module.ecs.cluster_name
}

output "receiver_service_name" {
  value = module.ecs.receiver_service_name
}

output "processor_service_name" {
  value = module.ecs.processor_service_name
}

output "sns_topic_arn" {
  value = module.sns_sqs.sns_topic_arn
}

output "sqs_queue_url" {
  value = module.sns_sqs.sqs_queue_url
}

output "lambda_function_name" {
  value = var.deploy_lambda ? module.lambda[0].function_name : "not deployed"
}

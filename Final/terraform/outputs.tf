output "alb_dns_name" {
  description = "ALB DNS name (use as base_url for ChaosArena submission)"
  value       = module.alb.dns_name
}

output "api_ecr_repository_url" {
  description = "ECR repository URL for API image"
  value       = module.ecr.api_repository_url
}

output "worker_ecr_repository_url" {
  description = "ECR repository URL for worker image"
  value       = module.ecr.worker_repository_url
}

output "s3_bucket_name" {
  description = "S3 bucket name for photo storage"
  value       = module.s3.bucket_name
}

output "sqs_queue_url" {
  description = "SQS queue URL"
  value       = module.sqs.queue_url
}

output "albums_table_name" {
  value = module.dynamodb.albums_table_name
}

output "counters_table_name" {
  value = module.dynamodb.counters_table_name
}

output "photos_table_name" {
  value = module.dynamodb.photos_table_name
}

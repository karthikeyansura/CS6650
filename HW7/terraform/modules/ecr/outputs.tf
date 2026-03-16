output "receiver_repository_url" {
  value = aws_ecr_repository.receiver.repository_url
}

output "processor_repository_url" {
  value = aws_ecr_repository.processor.repository_url
}

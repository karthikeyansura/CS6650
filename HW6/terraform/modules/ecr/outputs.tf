# Exposes the full repository URL for use in ECS task definitions or Docker pushes
output "repository_url" {
  description = "Full URL of the ECR repository"
  value       = aws_ecr_repository.this.repository_url
}
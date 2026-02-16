# ECR repository to store Docker images
resource "aws_ecr_repository" "this" {
  name = var.repository_name
}
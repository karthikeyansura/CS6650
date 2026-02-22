# Creates an ECR repository for storing container images
resource "aws_ecr_repository" "this" {
  name = var.repository_name
}
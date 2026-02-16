# Compose infrastructure from focused modules: network, ecr, logging, ecs.

module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
}

module "ecr" {
  source          = "./modules/ecr"
  repository_name = var.ecr_repository_name
}

module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

# Look up existing ECS task execution role from IAM
data "aws_iam_role" "execution_role" {
  name = "ecsTaskExecutionRole"
}

module "ecs" {
  source             = "./modules/ecs"
  service_name       = var.service_name
  image              = "${module.ecr.repository_url}:latest"
  container_port     = var.container_port
  subnet_ids         = module.network.subnet_ids
  security_group_ids = [module.network.security_group_id]
  execution_role_arn = data.aws_iam_role.execution_role.arn
  task_role_arn      = data.aws_iam_role.execution_role.arn
  log_group_name     = module.logging.log_group_name
  ecs_count          = var.ecs_count
  region             = var.aws_region
}


# Build the Go app image from ../src/ and tag it for ECR
resource "docker_image" "app" {
  name = "${module.ecr.repository_url}:latest"

  build {
    context = "../src"
  }
}

# Push the built image to ECR
resource "docker_registry_image" "app" {
  name = docker_image.app.name
}
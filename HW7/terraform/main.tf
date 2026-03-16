# Networking

module "vpc" {
  source         = "./modules/vpc"
  service_name   = var.service_name
  container_port = var.container_port
}

# Container Registries

module "ecr" {
  source       = "./modules/ecr"
  service_name = var.service_name
}

# Logging

module "logging" {
  source       = "./modules/logging"
  service_name = var.service_name
}

# Messaging (SNS + SQS)

module "sns_sqs" {
  source = "./modules/sns_sqs"
}

# IAM Roles

module "iam" {
  source        = "./modules/iam"
  service_name  = var.service_name
  sns_topic_arn = module.sns_sqs.sns_topic_arn
  sqs_queue_arn = module.sns_sqs.sqs_queue_arn
}

# ALB

module "alb" {
  source            = "./modules/alb"
  service_name      = var.service_name
  vpc_id            = module.vpc.vpc_id
  subnet_ids        = module.vpc.public_subnet_ids
  security_group_id = module.vpc.alb_security_group_id
  container_port    = var.container_port
}

# ECS (Receiver + Processor)

module "ecs" {
  source                 = "./modules/ecs"
  service_name           = var.service_name
  receiver_image         = "${module.ecr.receiver_repository_url}:latest"
  processor_image        = "${module.ecr.processor_repository_url}:latest"
  container_port         = var.container_port
  private_subnet_ids     = module.vpc.private_subnet_ids
  security_group_ids     = [module.vpc.ecs_security_group_id]
  execution_role_arn     = module.iam.execution_role_arn
  receiver_task_role_arn = module.iam.receiver_task_role_arn
  processor_task_role_arn = module.iam.processor_task_role_arn
  target_group_arn       = module.alb.target_group_arn
  sns_topic_arn          = module.sns_sqs.sns_topic_arn
  sqs_queue_url          = module.sns_sqs.sqs_queue_url
  receiver_log_group     = module.logging.receiver_log_group_name
  processor_log_group    = module.logging.processor_log_group_name
  region                 = var.aws_region
  worker_count           = var.worker_count
}

# Lambda (conditionally deployed)

module "lambda" {
  count           = var.deploy_lambda ? 1 : 0
  source          = "./modules/lambda"
  service_name    = var.service_name
  lambda_role_arn = module.iam.lambda_role_arn
  sns_topic_arn   = module.sns_sqs.sns_topic_arn
  lambda_zip_path = var.lambda_zip_path
}

# Docker Image Build & Push

# Build receiver image from ../src/receiver/
resource "docker_image" "receiver" {
  name = "${module.ecr.receiver_repository_url}:latest"

  build {
    context    = "${path.module}/../src/receiver"
    dockerfile = "Dockerfile"
    platform   = "linux/amd64"
  }

  triggers = {
    dir_sha = sha1(join("", [
      filesha1("${path.module}/../src/receiver/main.go"),
      filesha1("${path.module}/../src/receiver/go.mod"),
      filesha1("${path.module}/../src/receiver/Dockerfile"),
    ]))
  }
}

resource "docker_registry_image" "receiver" {
  name          = docker_image.receiver.name
  keep_remotely = true
}

# Build processor image from ../src/processor/
resource "docker_image" "processor" {
  name = "${module.ecr.processor_repository_url}:latest"

  build {
    context    = "${path.module}/../src/processor"
    dockerfile = "Dockerfile"
    platform   = "linux/amd64"
  }

  triggers = {
    dir_sha = sha1(join("", [
      filesha1("${path.module}/../src/processor/main.go"),
      filesha1("${path.module}/../src/processor/go.mod"),
      filesha1("${path.module}/../src/processor/Dockerfile"),
    ]))
  }
}

resource "docker_registry_image" "processor" {
  name          = docker_image.processor.name
  keep_remotely = true
}

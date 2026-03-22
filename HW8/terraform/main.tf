# Networking
module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
}

# ECR
module "ecr" {
  source          = "./modules/ecr"
  repository_name = var.ecr_repository_name
}

# Logging
module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

# RDS MySQL
module "rds" {
  source            = "./modules/rds"
  service_name      = var.service_name
  subnet_ids        = module.network.subnet_ids
  security_group_id = module.network.rds_security_group_id
  db_name           = var.db_name
  db_username       = var.db_username
  db_password       = var.db_password
}

# DynamoDB
module "dynamodb" {
  source     = "./modules/dynamodb"
  table_name = var.dynamodb_table_name
}

# IAM (task role with DynamoDB access)
module "iam" {
  source             = "./modules/iam"
  service_name       = var.service_name
  dynamodb_table_arn = module.dynamodb.table_arn
}

# ECS execution role (existing)
data "aws_iam_role" "execution_role" {
  name = "ecsTaskExecutionRole"
}

# ALB
module "alb" {
  source            = "./modules/alb"
  service_name      = var.service_name
  vpc_id            = module.network.vpc_id
  subnet_ids        = module.network.subnet_ids
  security_group_id = module.network.alb_security_group_id
  container_port    = var.container_port
}

# Environment variables for ECS container
locals {
  # Build the MySQL DSN from RDS outputs
  mysql_dsn = "${var.db_username}:${var.db_password}@tcp(${module.rds.address}:${module.rds.port})/${var.db_name}?parseTime=true"

  env_vars = [
    { name = "DB_MODE",        value = var.db_mode },
    { name = "MYSQL_DSN",      value = local.mysql_dsn },
    { name = "DYNAMODB_TABLE", value = var.dynamodb_table_name },
    { name = "AWS_REGION",     value = var.aws_region },
    { name = "GIN_MODE",       value = "release" },
  ]
}

# ECS
module "ecs" {
  source                = "./modules/ecs"
  service_name          = var.service_name
  image                 = "${module.ecr.repository_url}:latest"
  container_port        = var.container_port
  subnet_ids            = module.network.subnet_ids
  security_group_ids    = [module.network.security_group_id]
  execution_role_arn    = data.aws_iam_role.execution_role.arn
  task_role_arn         = module.iam.task_role_arn
  log_group_name        = module.logging.log_group_name
  ecs_count             = var.ecs_count
  region                = var.aws_region
  target_group_arn      = module.alb.target_group_arn
  environment_variables = local.env_vars
}

# Autoscaling
module "autoscaling" {
  source             = "./modules/autoscaling"
  service_name       = var.service_name
  ecs_cluster_name   = module.ecs.cluster_name
  ecs_service_name   = module.ecs.service_name
  min_capacity       = var.autoscaling_min
  max_capacity       = var.autoscaling_max
  cpu_target_value   = var.cpu_target_value
  scale_out_cooldown = var.scale_out_cooldown
  scale_in_cooldown  = var.scale_in_cooldown
}

# Docker build & push
resource "docker_image" "app" {
  name = "${module.ecr.repository_url}:latest"
  build {
    context = "../src"
  }
}

resource "docker_registry_image" "app" {
  name = docker_image.app.name
}

module "network" {
  source       = "./modules/network"
  project_name = var.project_name
  aws_region   = var.aws_region
}

module "ecr" {
  source       = "./modules/ecr"
  project_name = var.project_name
}

module "s3" {
  source       = "./modules/s3"
  project_name = var.project_name
  environment  = var.environment
}

module "dynamodb" {
  source       = "./modules/dynamodb"
  project_name = var.project_name
}

module "sqs" {
  source       = "./modules/sqs"
  project_name = var.project_name
}

module "logging" {
  source       = "./modules/logging"
  project_name = var.project_name
}

module "iam" {
  source             = "./modules/iam"
  project_name       = var.project_name
  aws_region         = var.aws_region
  s3_bucket_arn      = module.s3.bucket_arn
  albums_table_arn   = module.dynamodb.albums_table_arn
  counters_table_arn = module.dynamodb.counters_table_arn
  photos_table_arn   = module.dynamodb.photos_table_arn
  sqs_queue_arn      = module.sqs.queue_arn
}

module "alb" {
  source            = "./modules/alb"
  project_name      = var.project_name
  vpc_id            = module.network.vpc_id
  public_subnet_ids = module.network.public_subnet_ids
  security_group_id = module.network.alb_security_group_id
}

module "ecs_api" {
  source                = "./modules/ecs_api"
  project_name          = var.project_name
  aws_region            = var.aws_region
  cluster_id            = module.network.ecs_cluster_id
  api_image             = var.api_image
  api_cpu               = var.api_cpu
  api_memory            = var.api_memory
  api_desired_count     = var.api_desired_count
  private_subnet_ids    = module.network.private_subnet_ids
  ecs_security_group_id = module.network.ecs_security_group_id
  target_group_arn      = module.alb.target_group_arn
  execution_role_arn    = module.iam.execution_role_arn
  api_task_role_arn     = module.iam.api_task_role_arn
  log_group_name        = module.logging.api_log_group_name
  albums_table          = module.dynamodb.albums_table_name
  counters_table        = module.dynamodb.counters_table_name
  photos_table          = module.dynamodb.photos_table_name
  s3_bucket             = module.s3.bucket_name
  s3_region             = var.aws_region
  sqs_queue_url         = module.sqs.queue_url
}

module "ecs_worker" {
  source                = "./modules/ecs_worker"
  project_name          = var.project_name
  aws_region            = var.aws_region
  cluster_id            = module.network.ecs_cluster_id
  worker_image          = var.worker_image
  worker_cpu            = var.worker_cpu
  worker_memory         = var.worker_memory
  worker_desired_count  = var.worker_desired_count
  private_subnet_ids    = module.network.private_subnet_ids
  ecs_security_group_id = module.network.ecs_security_group_id
  execution_role_arn    = module.iam.execution_role_arn
  worker_task_role_arn  = module.iam.worker_task_role_arn
  log_group_name        = module.logging.worker_log_group_name
  albums_table          = module.dynamodb.albums_table_name
  counters_table        = module.dynamodb.counters_table_name
  photos_table          = module.dynamodb.photos_table_name
  s3_bucket             = module.s3.bucket_name
  s3_region             = var.aws_region
  sqs_queue_url         = module.sqs.queue_url
}

module "autoscaling" {
  source              = "./modules/autoscaling"
  project_name        = var.project_name
  ecs_cluster_name    = module.network.ecs_cluster_name
  api_service_name    = module.ecs_api.service_name
  worker_service_name = module.ecs_worker.service_name
  sqs_queue_name      = module.sqs.queue_name
}

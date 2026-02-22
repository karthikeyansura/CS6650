# Compose infrastructure from focused modules: network, ecr, logging, ALB, ECS, and autoscaling.

# Create VPC, subnets, and security groups for ECS tasks and ALB
module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
}

# Create an ECR repository to store the Docker image
module "ecr" {
  source          = "./modules/ecr"
  repository_name = var.ecr_repository_name
}

# Create a CloudWatch Log Group for ECS task logs
module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

# Look up the existing ECS task execution role from IAM
# This role allows ECS tasks to pull images from ECR and send logs to CloudWatch
data "aws_iam_role" "execution_role" {
  name = "ecsTaskExecutionRole"
}

# Create an Application Load Balancer, listener, and target group
# This distributes incoming HTTP traffic across ECS tasks
module "alb" {
  source            = "./modules/alb"
  service_name      = var.service_name
  vpc_id            = module.network.vpc_id
  subnet_ids        = module.network.subnet_ids
  security_group_id = module.network.alb_security_group_id
  container_port    = var.container_port
}

# Create ECS cluster, task definition, and Fargate service
# The service registers tasks with the ALB target group
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
  target_group_arn   = module.alb.target_group_arn
}

# Configure ECS Service Auto Scaling based on average CPU utilization
# Automatically scales the number of running tasks between defined min and max limits
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

# Build the Go application Docker image from ../src/
# Tag the image using the ECR repository URL
resource "docker_image" "app" {
  name = "${module.ecr.repository_url}:latest"
  build {
    context = "../src"
  }
}

# Push the locally built Docker image to Amazon ECR
resource "docker_registry_image" "app" {
  name = docker_image.app.name
}
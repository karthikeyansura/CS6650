variable "project_name" { type = string }
variable "aws_region" { type = string }
variable "cluster_id" { type = string }
variable "worker_image" { type = string }
variable "worker_cpu" { type = number }
variable "worker_memory" { type = number }
variable "worker_desired_count" { type = number }
variable "private_subnet_ids" { type = list(string) }
variable "ecs_security_group_id" { type = string }
variable "execution_role_arn" { type = string }
variable "worker_task_role_arn" { type = string }
variable "log_group_name" { type = string }
variable "albums_table" { type = string }
variable "counters_table" { type = string }
variable "photos_table" { type = string }
variable "s3_bucket" { type = string }
variable "s3_region" { type = string }
variable "sqs_queue_url" { type = string }

resource "aws_ecs_task_definition" "worker" {
  family                   = "${var.project_name}-worker"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = var.worker_cpu
  memory                   = var.worker_memory
  execution_role_arn       = var.execution_role_arn
  task_role_arn            = var.worker_task_role_arn

  container_definitions = jsonencode([
    {
      name      = "worker"
      image     = var.worker_image
      essential = true

      environment = [
        { name = "AWS_REGION", value = var.aws_region },
        { name = "ALBUMS_TABLE", value = var.albums_table },
        { name = "COUNTERS_TABLE", value = var.counters_table },
        { name = "PHOTOS_TABLE", value = var.photos_table },
        { name = "S3_BUCKET", value = var.s3_bucket },
        { name = "S3_REGION", value = var.s3_region },
        { name = "SQS_QUEUE_URL", value = var.sqs_queue_url },
        { name = "WORKER_CONCURRENCY", value = "10" },
      ]

      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = var.log_group_name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "worker"
        }
      }
    }
  ])
}

resource "aws_ecs_service" "worker" {
  name            = "${var.project_name}-worker"
  cluster         = var.cluster_id
  task_definition = aws_ecs_task_definition.worker.arn
  desired_count   = var.worker_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [var.ecs_security_group_id]
    assign_public_ip = false
  }

  deployment_minimum_healthy_percent = 100
  deployment_maximum_percent         = 200
}

output "service_name" { value = aws_ecs_service.worker.name }

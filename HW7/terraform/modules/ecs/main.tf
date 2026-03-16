# ECS Cluster
resource "aws_ecs_cluster" "this" {
  name = "${var.service_name}-cluster"
}

# Order Receiver Service

resource "aws_ecs_task_definition" "receiver" {
  family                   = "${var.service_name}-receiver-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = var.execution_role_arn
  task_role_arn            = var.receiver_task_role_arn

  container_definitions = jsonencode([{
    name      = "${var.service_name}-receiver"
    image     = var.receiver_image
    essential = true

    portMappings = [{
      containerPort = var.container_port
    }]

    environment = [
      { name = "SNS_TOPIC_ARN", value = var.sns_topic_arn }
    ]

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = var.receiver_log_group
        "awslogs-region"        = var.region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

resource "aws_ecs_service" "receiver" {
  name            = "${var.service_name}-receiver"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.receiver.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = var.security_group_ids
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = var.target_group_arn
    container_name   = "${var.service_name}-receiver"
    container_port   = var.container_port
  }

  health_check_grace_period_seconds = 60
}

# Order Processor Service

resource "aws_ecs_task_definition" "processor" {
  family                   = "${var.service_name}-processor-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.cpu
  memory                   = var.memory
  execution_role_arn       = var.execution_role_arn
  task_role_arn            = var.processor_task_role_arn

  container_definitions = jsonencode([{
    name      = "${var.service_name}-processor"
    image     = var.processor_image
    essential = true

    environment = [
      { name = "SQS_QUEUE_URL", value = var.sqs_queue_url },
      { name = "WORKER_COUNT", value = tostring(var.worker_count) }
    ]

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = var.processor_log_group
        "awslogs-region"        = var.region
        "awslogs-stream-prefix" = "ecs"
      }
    }
  }])
}

resource "aws_ecs_service" "processor" {
  name            = "${var.service_name}-processor"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.processor.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = var.security_group_ids
    assign_public_ip = false
  }
}

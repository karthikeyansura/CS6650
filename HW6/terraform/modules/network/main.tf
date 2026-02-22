# Fetch the default VPC
data "aws_vpc" "default" {
  default = true
}

# List all subnets in the default VPC
data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# Security group for ECS tasks
# Allows inbound from the ALB and from direct IPs
resource "aws_security_group" "ecs" {
  name        = "${var.service_name}-ecs-sg"
  description = "Allow inbound from ALB on ${var.container_port}"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port       = var.container_port
    to_port         = var.container_port
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
    description     = "Allow traffic from ALB"
  }

  ingress {
    from_port   = var.container_port
    to_port     = var.container_port
    protocol    = "tcp"
    cidr_blocks = var.cidr_blocks
    description = "Allow direct HTTP traffic"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound"
  }
}

# Security group for the Application Load Balancer (ALB)
# Allows inbound HTTP traffic on port 80
resource "aws_security_group" "alb" {
  name        = "${var.service_name}-alb-sg"
  description = "Allow inbound HTTP on port 80 for ALB"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow HTTP traffic"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound"
  }
}
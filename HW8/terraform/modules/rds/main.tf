# DB subnet group using default VPC subnets
resource "aws_db_subnet_group" "this" {
  name       = "${var.service_name}-db-subnet"
  subnet_ids = var.subnet_ids

  tags = {
    Name = "${var.service_name}-db-subnet"
  }
}

# RDS MySQL 8.0 instance on free tier
resource "aws_db_instance" "this" {
  identifier     = "${var.service_name}-mysql"
  engine         = "mysql"
  engine_version = "8.0"
  instance_class = "db.t3.micro"

  allocated_storage = 20
  storage_type      = "gp2"

  db_name  = var.db_name
  username = var.db_username
  password = var.db_password

  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [var.security_group_id]

  # Skip snapshot, allow easy teardown
  skip_final_snapshot    = true
  deletion_protection    = false
  publicly_accessible    = false
  multi_az               = false
  backup_retention_period = 0

  tags = {
    Name = "${var.service_name}-mysql"
  }
}

resource "aws_ecr_repository" "receiver" {
  name = "${var.service_name}-receiver"
}

resource "aws_ecr_repository" "processor" {
  name = "${var.service_name}-processor"
}

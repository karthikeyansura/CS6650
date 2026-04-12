variable "project_name" { type = string }

resource "aws_sqs_queue" "dlq" {
  name                      = "${var.project_name}-dlq"
  message_retention_seconds = 1209600 # 14 days

  tags = { Name = "${var.project_name}-dlq" }
}

resource "aws_sqs_queue" "main" {
  name                       = "${var.project_name}-photo-queue"
  visibility_timeout_seconds = 120
  message_retention_seconds  = 345600  # 4 days
  receive_wait_time_seconds  = 20      # long polling

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq.arn
    maxReceiveCount     = 3
  })

  tags = { Name = "${var.project_name}-photo-queue" }
}

output "queue_url" { value = aws_sqs_queue.main.url }
output "queue_arn" { value = aws_sqs_queue.main.arn }
output "queue_name" { value = aws_sqs_queue.main.name }
output "dlq_arn" { value = aws_sqs_queue.dlq.arn }

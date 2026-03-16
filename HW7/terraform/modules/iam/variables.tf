variable "service_name" {
  type = string
}

variable "sns_topic_arn" {
  type        = string
  description = "ARN of the SNS topic for receiver publish permissions"
}

variable "sqs_queue_arn" {
  type        = string
  description = "ARN of the SQS queue for processor consume permissions"
}

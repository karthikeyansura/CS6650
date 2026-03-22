variable "service_name" {
  description = "Base name for IAM resources"
  type        = string
}

variable "dynamodb_table_arn" {
  description = "ARN of the DynamoDB table to grant access to"
  type        = string
}

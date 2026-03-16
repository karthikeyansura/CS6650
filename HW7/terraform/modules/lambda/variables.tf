variable "service_name" {
  type = string
}

variable "lambda_role_arn" {
  type = string
}

variable "sns_topic_arn" {
  type = string
}

variable "lambda_zip_path" {
  type        = string
  description = "Path to the compiled Lambda deployment zip"
}

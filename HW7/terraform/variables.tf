variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "service_name" {
  type    = string
  default = "order-service"
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "worker_count" {
  type        = number
  default     = 1
  description = "Number of concurrent worker goroutines in the processor task"
}

variable "deploy_lambda" {
  type        = bool
  default     = false
  description = "Set to true to deploy the Lambda function (Part III)"
}

variable "lambda_zip_path" {
  type        = string
  default     = "../src/lambda/function.zip"
  description = "Path to the compiled Lambda deployment zip"
}

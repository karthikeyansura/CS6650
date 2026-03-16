variable "service_name" {
  type = string
}

variable "receiver_image" {
  type = string
}

variable "processor_image" {
  type = string
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "security_group_ids" {
  type = list(string)
}

variable "execution_role_arn" {
  type = string
}

variable "receiver_task_role_arn" {
  type = string
}

variable "processor_task_role_arn" {
  type = string
}

variable "target_group_arn" {
  type = string
}

variable "sns_topic_arn" {
  type = string
}

variable "sqs_queue_url" {
  type = string
}

variable "receiver_log_group" {
  type = string
}

variable "processor_log_group" {
  type = string
}

variable "region" {
  type = string
}

variable "cpu" {
  type    = string
  default = "256"
}

variable "memory" {
  type    = string
  default = "512"
}

variable "worker_count" {
  type        = number
  default     = 1
  description = "Number of concurrent worker goroutines in the processor"
}

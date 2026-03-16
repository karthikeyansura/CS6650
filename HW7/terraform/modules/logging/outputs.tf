output "receiver_log_group_name" {
  value = aws_cloudwatch_log_group.receiver.name
}

output "processor_log_group_name" {
  value = aws_cloudwatch_log_group.processor.name
}

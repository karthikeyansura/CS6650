# Lambda function for order processing (Part III)
resource "aws_lambda_function" "order_processor" {
  function_name = "${var.service_name}-order-processor"
  role          = var.lambda_role_arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  memory_size   = 512
  timeout       = 30

  filename         = var.lambda_zip_path
  source_code_hash = filebase64sha256(var.lambda_zip_path)

  tags = { Name = "${var.service_name}-order-processor" }

  depends_on = [aws_cloudwatch_log_group.lambda]
}

# Explicitly manage the Lambda log group so terraform destroy removes it
resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${var.service_name}-order-processor"
  retention_in_days = 7
}

# Allow SNS to invoke the Lambda function
resource "aws_lambda_permission" "sns" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.order_processor.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = var.sns_topic_arn
}

# Subscribe Lambda directly to the SNS topic (no SQS needed)
resource "aws_sns_topic_subscription" "lambda" {
  topic_arn = var.sns_topic_arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.order_processor.arn
}

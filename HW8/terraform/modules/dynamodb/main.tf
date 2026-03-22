# DynamoDB table for shopping carts
# Single-table design: cart_id as partition key, items embedded as a list attribute
resource "aws_dynamodb_table" "this" {
  name         = var.table_name
  billing_mode = "PAY_PER_REQUEST" # On-demand for variable load

  hash_key = "cart_id"

  attribute {
    name = "cart_id"
    type = "S"
  }

  tags = {
    Name = var.table_name
  }
}

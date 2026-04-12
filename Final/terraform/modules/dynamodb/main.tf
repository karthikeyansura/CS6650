variable "project_name" { type = string }

resource "aws_dynamodb_table" "albums" {
  name         = "${var.project_name}-albums"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"

  attribute {
    name = "album_id"
    type = "S"
  }

  tags = { Name = "${var.project_name}-albums" }
}

resource "aws_dynamodb_table" "counters" {
  name         = "${var.project_name}-album-counters"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"

  attribute {
    name = "album_id"
    type = "S"
  }

  tags = { Name = "${var.project_name}-album-counters" }
}

resource "aws_dynamodb_table" "photos" {
  name         = "${var.project_name}-photos"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"
  range_key    = "photo_id"

  attribute {
    name = "album_id"
    type = "S"
  }

  attribute {
    name = "photo_id"
    type = "S"
  }

  tags = { Name = "${var.project_name}-photos" }
}

output "albums_table_name" { value = aws_dynamodb_table.albums.name }
output "albums_table_arn" { value = aws_dynamodb_table.albums.arn }
output "counters_table_name" { value = aws_dynamodb_table.counters.name }
output "counters_table_arn" { value = aws_dynamodb_table.counters.arn }
output "photos_table_name" { value = aws_dynamodb_table.photos.name }
output "photos_table_arn" { value = aws_dynamodb_table.photos.arn }

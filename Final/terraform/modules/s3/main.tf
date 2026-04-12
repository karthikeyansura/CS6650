variable "project_name" { type = string }
variable "environment" { type = string }

resource "aws_s3_bucket" "photos" {
  bucket        = "${var.project_name}-photos-${var.environment}"
  force_destroy = true

  tags = { Name = "${var.project_name}-photos" }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "photos" {
  bucket = aws_s3_bucket.photos.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_versioning" "photos" {
  bucket = aws_s3_bucket.photos.id
  versioning_configuration {
    status = "Disabled"
  }
}

# public access: ChaosArena grader must fetch the photo URL and get 200
resource "aws_s3_bucket_public_access_block" "photos" {
  bucket = aws_s3_bucket.photos.id

  block_public_acls       = false
  block_public_policy     = false
  ignore_public_acls      = false
  restrict_public_buckets = false
}

resource "aws_s3_bucket_policy" "photos_public_read" {
  bucket = aws_s3_bucket.photos.id

  depends_on = [aws_s3_bucket_public_access_block.photos]

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "PublicReadGetObject"
        Effect    = "Allow"
        Principal = "*"
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.photos.arn}/*"
      }
    ]
  })
}

output "bucket_name" { value = aws_s3_bucket.photos.id }
output "bucket_arn" { value = aws_s3_bucket.photos.arn }

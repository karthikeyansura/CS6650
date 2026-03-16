#!/bin/bash
# Build the Lambda function for deployment to AWS Lambda (provided.al2 runtime).
# Run from the project root: ./scripts/build_lambda.sh

set -euo pipefail

LAMBDA_DIR="src/lambda"
OUTPUT_DIR="${LAMBDA_DIR}"

echo "Building Lambda function..."
cd "${LAMBDA_DIR}"

# Cross-compile for Amazon Linux 2 (x86_64)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bootstrap .

# Package into a zip for deployment
zip -j function.zip bootstrap

# Clean up the binary
rm bootstrap

echo "Lambda package created: ${OUTPUT_DIR}/function.zip"
echo "Deploy with: terraform apply -var deploy_lambda=true"

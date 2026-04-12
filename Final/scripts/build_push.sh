#!/usr/bin/env bash
set -euo pipefail

REGION="us-west-2"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_REGISTRY="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"

echo "Logging into ECR..."
aws ecr get-login-password --region "${REGION}" | \
  docker login --username AWS --password-stdin "${ECR_REGISTRY}"

echo "Building API image..."
docker build --platform linux/amd64 \
  -t "${ECR_REGISTRY}/album-store-api:latest" \
  -f deployments/docker/Dockerfile.api \
  .

echo "Pushing API image..."
docker push "${ECR_REGISTRY}/album-store-api:latest"

echo "Building worker image..."
docker build --platform linux/amd64 \
  -t "${ECR_REGISTRY}/album-store-worker:latest" \
  -f deployments/docker/Dockerfile.worker \
  .

echo "Pushing worker image..."
docker push "${ECR_REGISTRY}/album-store-worker:latest"

echo ""
echo "Done. Image URIs:"
echo "  API:    ${ECR_REGISTRY}/album-store-api:latest"
echo "  Worker: ${ECR_REGISTRY}/album-store-worker:latest"

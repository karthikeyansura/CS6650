#!/usr/bin/env bash
set -euo pipefail

REGION="us-west-2"
CLUSTER="album-store-cluster"

echo "Applying Terraform..."
cd terraform
terraform apply -auto-approve
cd ..

echo "Forcing new deployment for API service..."
aws ecs update-service \
  --cluster "${CLUSTER}" \
  --service album-store-api \
  --force-new-deployment \
  --region "${REGION}" \
  --no-cli-pager

echo "Forcing new deployment for worker service..."
aws ecs update-service \
  --cluster "${CLUSTER}" \
  --service album-store-worker \
  --force-new-deployment \
  --region "${REGION}" \
  --no-cli-pager

echo ""
echo "Deployment triggered. Monitor with:"
echo "  aws ecs describe-services --cluster ${CLUSTER} --services album-store-api album-store-worker --region ${REGION} --query 'services[].deployments'"

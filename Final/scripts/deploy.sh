#!/usr/bin/env bash
set -euo pipefail

REGION="us-west-2"
CLUSTER="album-store-cluster"

echo "=== Step 1: Terraform Apply ==="
cd terraform
terraform apply -auto-approve
ALB_DNS=$(terraform output -raw alb_dns_name)
cd ..
echo "ALB: http://${ALB_DNS}"

echo ""
echo "=== Step 2: Build and Push Images ==="
./scripts/build_push.sh

echo ""
echo "=== Step 3: Force New Deployment ==="
aws ecs update-service \
  --cluster "${CLUSTER}" \
  --service album-store-api \
  --force-new-deployment \
  --region "${REGION}" \
  --no-cli-pager > /dev/null

aws ecs update-service \
  --cluster "${CLUSTER}" \
  --service album-store-worker \
  --force-new-deployment \
  --region "${REGION}" \
  --no-cli-pager > /dev/null

echo "Waiting for tasks to stabilize..."
for i in $(seq 1 60); do
  RESULT=$(aws ecs describe-services \
    --cluster "${CLUSTER}" \
    --services album-store-api album-store-worker \
    --region "${REGION}" \
    --query 'services[].{name:serviceName,desired:desiredCount,running:runningCount}' \
    --output json --no-cli-pager)

  API_DESIRED=$(echo "${RESULT}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0]['desired'])")
  API_RUNNING=$(echo "${RESULT}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0]['running'])")
  WORKER_DESIRED=$(echo "${RESULT}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d[1]['desired'])")
  WORKER_RUNNING=$(echo "${RESULT}" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d[1]['running'])")

  echo "  API: ${API_RUNNING}/${API_DESIRED}  Worker: ${WORKER_RUNNING}/${WORKER_DESIRED}"

  if [ "${API_RUNNING}" = "${API_DESIRED}" ] && [ "${WORKER_RUNNING}" = "${WORKER_DESIRED}" ]; then
    echo "All tasks running."
    break
  fi

  if [ "${i}" = "60" ]; then
    echo "ERROR: Tasks did not stabilize within 5 minutes."
    exit 1
  fi

  sleep 5
done

echo ""
echo "=== Step 4: Smoke Test ==="
./scripts/smoke_test.sh "http://${ALB_DNS}"

echo ""
echo "=== Deployment Complete ==="
echo "ALB URL: http://${ALB_DNS}"
echo ""
echo "To submit:"
echo "  ./scripts/submit.sh sura.sa@northeastern.edu Karthikeyan http://${ALB_DNS}"
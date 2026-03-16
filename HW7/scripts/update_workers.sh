#!/bin/bash
# Update the processor's worker goroutine count and force a new deployment.
# Usage: ./scripts/update_workers.sh <worker_count>
# Example: ./scripts/update_workers.sh 20

set -euo pipefail

if [ $# -ne 1 ]; then
  echo "Usage: $0 <worker_count>"
  echo "Example: $0 20"
  exit 1
fi

WORKER_COUNT=$1

echo "Updating processor worker count to ${WORKER_COUNT}..."

cd terraform

# Apply with new worker count (forces new task definition revision)
terraform apply -var worker_count="${WORKER_COUNT}" -auto-approve

echo "Processor updated to ${WORKER_COUNT} workers."
echo ""
echo "NOTE: ECS rolling update runs both old and new tasks briefly."
echo "Wait ~2 minutes for the old task to drain before starting Locust."
echo "Verify only 1 processor task is running in the ECS console."
echo ""
echo "Monitor SQS queue depth in CloudWatch: Metrics > SQS > ApproximateNumberOfMessagesVisible"

#!/usr/bin/env bash
set -euo pipefail

ARENA_URL="http://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com"
EMAIL="${1:?Usage: ./submit.sh <email> <nickname> <base_url>}"
NICKNAME="${2:?}"
BASE_URL="${3:?}"

echo "Submitting to ChaosArena..."
RESP=$(curl -s -X POST "${ARENA_URL}/submit" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"${EMAIL}\",
    \"nickname\": \"${NICKNAME}\",
    \"base_url\": \"${BASE_URL}\",
    \"contract\": \"v1-album-store\"
  }")

echo "Response: ${RESP}"

RUN_ID=$(echo "${RESP}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('run_id',''))" 2>/dev/null || echo "")

if [ -z "${RUN_ID}" ]; then
  echo "ERROR: no run_id in response"
  exit 1
fi

echo ""
echo "Run ID: ${RUN_ID}"
echo "Polling for results..."

while true; do
  RESULT=$(curl -s "${ARENA_URL}/runs/${RUN_ID}")
  STATUS=$(echo "${RESULT}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null || echo "unknown")

  if [ "${STATUS}" = "completed" ]; then
    echo ""
    echo "=== Run Completed ==="
    echo "${RESULT}" | python3 -m json.tool
    break
  elif [ "${STATUS}" = "running" ] || [ "${STATUS}" = "queued" ]; then
    echo "  status: ${STATUS}..."
    sleep 5
  else
    echo "  unexpected status: ${STATUS}"
    echo "${RESULT}" | python3 -m json.tool 2>/dev/null || echo "${RESULT}"
    break
  fi
done

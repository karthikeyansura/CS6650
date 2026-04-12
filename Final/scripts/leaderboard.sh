#!/usr/bin/env bash
set -euo pipefail

ARENA_URL="http://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com"

echo "=== ChaosArena Leaderboard ==="
curl -s "${ARENA_URL}/leaderboard" | python3 -m json.tool

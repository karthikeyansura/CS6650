#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${1:?Usage: ./smoke_test.sh <base_url>}"
ALBUM_ID="smoke-$(date +%s)-$(openssl rand -hex 4)"
PASSED=0
FAILED=0

check() {
  local desc="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then
    echo "  PASS: ${desc}"
    PASSED=$((PASSED + 1))
  else
    echo "  FAIL: ${desc} (expected=${expected}, got=${actual})"
    FAILED=$((FAILED + 1))
  fi
}

echo "=== Smoke testing ${BASE_URL} ==="

# 1. health check
echo ""
echo "[1] GET /health"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${BASE_URL}/health")
check "status code" "200" "${STATUS}"
BODY=$(curl -s "${BASE_URL}/health")
echo "  body: ${BODY}"

# 2. create album
echo ""
echo "[2] PUT /albums/${ALBUM_ID}"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "${BASE_URL}/albums/${ALBUM_ID}" \
  -H "Content-Type: application/json" \
  -d "{\"album_id\":\"${ALBUM_ID}\",\"title\":\"Smoke Test\",\"description\":\"Testing\",\"owner\":\"smoke@test.com\"}")
check "create album status" "201" "${STATUS}"

# 3. get album
echo ""
echo "[3] GET /albums/${ALBUM_ID}"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${BASE_URL}/albums/${ALBUM_ID}")
check "get album status" "200" "${STATUS}"

# 4. list albums
echo ""
echo "[4] GET /albums"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${BASE_URL}/albums")
check "list albums status" "200" "${STATUS}"

# 5. upload photo
echo ""
echo "[5] POST /albums/${ALBUM_ID}/photos"
# create a tiny dummy file
echo "smoke test photo bytes" > /tmp/smoke_photo.jpg
UPLOAD_RESP=$(curl -s -X POST "${BASE_URL}/albums/${ALBUM_ID}/photos" \
  -F "photo=@/tmp/smoke_photo.jpg")
echo "  response: ${UPLOAD_RESP}"
PHOTO_ID=$(echo "${UPLOAD_RESP}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('photo_id',''))" 2>/dev/null || echo "")
SEQ=$(echo "${UPLOAD_RESP}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('seq',''))" 2>/dev/null || echo "")
echo "  photo_id=${PHOTO_ID} seq=${SEQ}"

if [ -n "${PHOTO_ID}" ]; then
  # 6. poll until completed
  echo ""
  echo "[6] Polling GET /albums/${ALBUM_ID}/photos/${PHOTO_ID}"
  for i in $(seq 1 30); do
    POLL_RESP=$(curl -s "${BASE_URL}/albums/${ALBUM_ID}/photos/${PHOTO_ID}")
    POLL_STATUS=$(echo "${POLL_RESP}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null || echo "")
    if [ "${POLL_STATUS}" = "completed" ]; then
      echo "  completed after ${i} polls"
      PHOTO_URL=$(echo "${POLL_RESP}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('url',''))" 2>/dev/null || echo "")
      echo "  url: ${PHOTO_URL}"

      # verify URL returns 200
      if [ -n "${PHOTO_URL}" ]; then
        URL_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${PHOTO_URL}")
        check "photo url accessible" "200" "${URL_STATUS}"
      fi
      break
    elif [ "${POLL_STATUS}" = "failed" ]; then
      echo "  FAIL: photo processing failed"
      FAILED=$((FAILED + 1))
      break
    fi
    sleep 1
  done

  # 7. delete photo
  echo ""
  echo "[7] DELETE /albums/${ALBUM_ID}/photos/${PHOTO_ID}"
  DEL_STATUS=$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "${BASE_URL}/albums/${ALBUM_ID}/photos/${PHOTO_ID}")
  check "delete status 204" "204" "${DEL_STATUS}"

  # verify 404 after delete
  GET_AFTER=$(curl -s -o /dev/null -w '%{http_code}' "${BASE_URL}/albums/${ALBUM_ID}/photos/${PHOTO_ID}")
  check "get after delete returns 404" "404" "${GET_AFTER}"

  # verify URL no longer returns 200
  if [ -n "${PHOTO_URL}" ]; then
    URL_AFTER=$(curl -s -o /dev/null -w '%{http_code}' "${PHOTO_URL}")
    if [ "${URL_AFTER}" != "200" ]; then
      echo "  PASS: url no longer returns 200 (got ${URL_AFTER})"
      PASSED=$((PASSED + 1))
    else
      echo "  FAIL: url still returns 200 after delete"
      FAILED=$((FAILED + 1))
    fi
  fi
fi

# 8. get nonexistent album
echo ""
echo "[8] GET /albums/nonexistent-id"
STATUS=$(curl -s -o /dev/null -w '%{http_code}' "${BASE_URL}/albums/nonexistent-id")
check "nonexistent album 404" "404" "${STATUS}"

echo ""
echo "=== Results: ${PASSED} passed, ${FAILED} failed ==="

rm -f /tmp/smoke_photo.jpg

if [ "${FAILED}" -gt 0 ]; then
  exit 1
fi

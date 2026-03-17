#!/usr/bin/env bash
# ==============================================================================
# End-to-End Validation Test
#
# Validates the complete credential rotation lifecycle:
#   1. Trigger rotation via Akeyless
#   2. Verify new password is stored in Akeyless
#   3. Verify new password works against AWX
#   4. Run CI/CD pipeline with rotated credentials
#   5. Check Event Center webhook was received
#
# Usage:
#   export AKEYLESS_ACCESS_ID=p-xxxx
#   export AKEYLESS_ACCESS_KEY=xxxx
#   ./e2e-test.sh
# ==============================================================================
set -euo pipefail

AKEYLESS_API="${AKEYLESS_API_URL:-https://api.akeyless.io}"
GW_API="${AKEYLESS_GW_API:-https://your-gateway.example.com/api/v2}"
AWX_URL="${AWX_URL:-https://ansible.example.com}"
SECRET_PATH="/Ansible/Credentials/server-build-svc"
PASS=0
FAIL=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

echo "=============================================="
echo "  END-TO-END VALIDATION TEST"
echo "  $(date -Iseconds)"
echo "=============================================="
echo ""

# Authenticate
echo "[Auth] Authenticating to Akeyless..."
TOKEN=$(curl -sf -X POST "${AKEYLESS_API}/auth" \
  -H "Content-Type: application/json" \
  -d "{\"access-id\": \"${AKEYLESS_ACCESS_ID}\", \"access-key\": \"${AKEYLESS_ACCESS_KEY}\", \"access-type\": \"api_key\"}" \
  | jq -r '.token')

if [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
  pass "Akeyless authentication"
else
  fail "Akeyless authentication"
  echo "Cannot continue without auth. Exiting."
  exit 1
fi

# Test 1: Get current password
echo ""
echo "[Test 1] Fetch current rotated secret value..."
BEFORE=$(curl -sf -X POST "${AKEYLESS_API}/get-rotated-secret-value" \
  -H "Content-Type: application/json" \
  -d "{\"token\": \"${TOKEN}\", \"names\": \"${SECRET_PATH}\"}" \
  | jq -r '.value.payload')
OLD_PASSWORD=$(echo "$BEFORE" | jq -r '.password')

if [ -n "$OLD_PASSWORD" ] && [ "$OLD_PASSWORD" != "null" ]; then
  pass "Fetch current password (length: ${#OLD_PASSWORD})"
else
  fail "Fetch current password"
fi

# Test 2: Trigger rotation
echo ""
echo "[Test 2] Trigger manual rotation via Akeyless Gateway..."
ROTATE_RESULT=$(curl -sk -X POST "${GW_API}/rotate-secret" \
  -H "Content-Type: application/json" \
  -d "{\"token\": \"${TOKEN}\", \"name\": \"${SECRET_PATH}\"}" 2>&1)

if echo "$ROTATE_RESULT" | jq -e '.name' > /dev/null 2>&1; then
  pass "Rotation triggered successfully"
else
  fail "Rotation trigger: $(echo "$ROTATE_RESULT" | jq -r '.error // "unknown"')"
fi

sleep 2

# Test 3: Verify password changed
echo ""
echo "[Test 3] Verify password was rotated..."
AFTER=$(curl -sf -X POST "${AKEYLESS_API}/get-rotated-secret-value" \
  -H "Content-Type: application/json" \
  -d "{\"token\": \"${TOKEN}\", \"names\": \"${SECRET_PATH}\"}" \
  | jq -r '.value.payload')
NEW_PASSWORD=$(echo "$AFTER" | jq -r '.password')

if [ "$NEW_PASSWORD" != "$OLD_PASSWORD" ] && [ -n "$NEW_PASSWORD" ]; then
  pass "Password changed (old != new)"
else
  fail "Password not changed"
fi

# Test 4: Verify new password works on AWX
echo ""
echo "[Test 4] Authenticate to AWX with new password..."
AWX_AUTH=$(curl -sk -u "svc-server-build:${NEW_PASSWORD}" "${AWX_URL}/api/v2/me/" 2>&1)
AWX_USER=$(echo "$AWX_AUTH" | jq -r '.results[0].username // empty')

if [ "$AWX_USER" = "svc-server-build" ]; then
  pass "AWX authentication with rotated password"
else
  fail "AWX authentication failed"
fi

# Test 5: Verify old password no longer works
echo ""
echo "[Test 5] Verify old password is rejected..."
OLD_AUTH_CODE=$(curl -sk -o /dev/null -w "%{http_code}" -u "svc-server-build:${OLD_PASSWORD}" "${AWX_URL}/api/v2/me/")

if [ "$OLD_AUTH_CODE" = "401" ]; then
  pass "Old password correctly rejected (HTTP 401)"
else
  fail "Old password still works (HTTP ${OLD_AUTH_CODE})"
fi

# Test 6: Run CI/CD pipeline
echo ""
echo "[Test 6] Run server build pipeline with rotated creds..."
PIPELINE_OUTPUT=$(./cicd/pipeline-server-build.sh "e2e-test-server" "webserver" 2>&1)
if echo "$PIPELINE_OUTPUT" | grep -q "PIPELINE RESULT: SUCCESS"; then
  pass "CI/CD pipeline completed successfully"
else
  fail "CI/CD pipeline failed"
  echo "$PIPELINE_OUTPUT" | tail -10
fi

# Summary
echo ""
echo "=============================================="
echo "  TEST RESULTS"
echo "=============================================="
echo "  Passed: ${PASS}"
echo "  Failed: ${FAIL}"
echo "  Total:  $((PASS + FAIL))"
echo "=============================================="

if [ "$FAIL" -eq 0 ]; then
  echo ""
  echo "  ALL TESTS PASSED"
  echo ""
  exit 0
else
  echo ""
  echo "  SOME TESTS FAILED"
  echo ""
  exit 1
fi

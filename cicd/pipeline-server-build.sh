#!/usr/bin/env bash
# ==============================================================================
# Server Build Pipeline
#
# Simulates the customer's CI/CD pipeline that:
#   1. Authenticates to Akeyless
#   2. Fetches the rotated service account credentials
#   3. Authenticates to Ansible AWX using those credentials
#   4. Launches the server build job template
#   5. Waits for job completion
#
# This demonstrates that rotated credentials are always current and pipelines
# never fail due to stale passwords.
#
# Usage:
#   export AKEYLESS_ACCESS_ID=p-xxxx
#   export AKEYLESS_ACCESS_KEY=xxxx
#   ./pipeline-server-build.sh [server_name] [server_role]
#
# ==============================================================================
set -euo pipefail

# Configuration
AKEYLESS_API="${AKEYLESS_API_URL:-https://api.akeyless.io}"
AWX_URL="${AWX_URL:-https://ansible.example.com}"
SECRET_PATH="${SECRET_PATH:-/Ansible/Credentials/server-build-svc}"
JOB_TEMPLATE_NAME="${JOB_TEMPLATE_NAME:-Server Build}"
SERVER_NAME="${1:-pipeline-server-$(date +%s)}"
SERVER_ROLE="${2:-webserver}"

echo "==============================================="
echo "  SERVER BUILD PIPELINE"
echo "==============================================="
echo "Server:  ${SERVER_NAME}"
echo "Role:    ${SERVER_ROLE}"
echo "AWX:     ${AWX_URL}"
echo "Secret:  ${SECRET_PATH}"
echo "==============================================="
echo ""

# Step 1: Authenticate to Akeyless
echo "[1/5] Authenticating to Akeyless..."
TOKEN=$(curl -sf -X POST "${AKEYLESS_API}/auth" \
  -H "Content-Type: application/json" \
  -d "{
    \"access-id\": \"${AKEYLESS_ACCESS_ID}\",
    \"access-key\": \"${AKEYLESS_ACCESS_KEY}\",
    \"access-type\": \"api_key\"
  }" | jq -r '.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "FAILED: Could not authenticate to Akeyless"
  exit 1
fi
echo "  Authenticated successfully."

# Step 2: Fetch rotated credentials
echo "[2/5] Fetching rotated credentials from Akeyless..."
CRED_PAYLOAD=$(curl -sf -X POST "${AKEYLESS_API}/get-rotated-secret-value" \
  -H "Content-Type: application/json" \
  -d "{
    \"token\": \"${TOKEN}\",
    \"names\": \"${SECRET_PATH}\"
  }" | jq -r '.value.payload')

AWX_USERNAME=$(echo "$CRED_PAYLOAD" | jq -r '.target_username')
AWX_PASSWORD=$(echo "$CRED_PAYLOAD" | jq -r '.password')

if [ -z "$AWX_PASSWORD" ] || [ "$AWX_PASSWORD" = "null" ]; then
  echo "FAILED: Could not fetch credentials from Akeyless"
  exit 1
fi
echo "  Retrieved credentials for user '${AWX_USERNAME}'."

# Step 3: Authenticate to AWX
echo "[3/5] Authenticating to AWX as '${AWX_USERNAME}'..."
ME=$(curl -sk -u "${AWX_USERNAME}:${AWX_PASSWORD}" "${AWX_URL}/api/v2/me/" 2>&1)
USER_ID=$(echo "$ME" | jq -r '.results[0].id // empty')

if [ -z "$USER_ID" ]; then
  echo "FAILED: Could not authenticate to AWX with rotated credentials"
  echo "  This might mean the credentials are stale or rotation failed."
  exit 1
fi
echo "  Authenticated as user ID ${USER_ID}."

# Step 4: Launch the server build job
echo "[4/5] Launching job template '${JOB_TEMPLATE_NAME}'..."
JT_SEARCH=$(curl -sk -u "${AWX_USERNAME}:${AWX_PASSWORD}" \
  "${AWX_URL}/api/v2/job_templates/?name=$(echo "$JOB_TEMPLATE_NAME" | sed 's/ /%20/g')" 2>&1)
JT_ID=$(echo "$JT_SEARCH" | jq -r '.results[0].id // empty')

if [ -z "$JT_ID" ]; then
  echo "FAILED: Job template '${JOB_TEMPLATE_NAME}' not found"
  exit 1
fi

LAUNCH=$(curl -sk -u "${AWX_USERNAME}:${AWX_PASSWORD}" \
  -X POST "${AWX_URL}/api/v2/job_templates/${JT_ID}/launch/" \
  -H "Content-Type: application/json" \
  -d "{\"extra_vars\": {\"server_name\": \"${SERVER_NAME}\", \"server_role\": \"${SERVER_ROLE}\"}}" 2>&1)
JOB_ID=$(echo "$LAUNCH" | jq -r '.id // empty')

if [ -z "$JOB_ID" ]; then
  echo "FAILED: Could not launch job"
  echo "$LAUNCH" | jq '.'
  exit 1
fi
echo "  Job ${JOB_ID} launched."

# Step 5: Wait for job completion
echo "[5/5] Waiting for job ${JOB_ID} to complete..."
for i in $(seq 1 60); do
  JOB_STATUS=$(curl -sk -u "${AWX_USERNAME}:${AWX_PASSWORD}" \
    "${AWX_URL}/api/v2/jobs/${JOB_ID}/" 2>&1 | jq -r '.status')

  case "$JOB_STATUS" in
    successful)
      echo ""
      echo "==============================================="
      echo "  PIPELINE RESULT: SUCCESS"
      echo "==============================================="
      echo "  Job ID:     ${JOB_ID}"
      echo "  Server:     ${SERVER_NAME}"
      echo "  Role:       ${SERVER_ROLE}"
      echo "  AWX URL:    ${AWX_URL}/#/jobs/${JOB_ID}"
      echo "==============================================="
      exit 0
      ;;
    failed|error|canceled)
      echo ""
      echo "==============================================="
      echo "  PIPELINE RESULT: FAILED"
      echo "==============================================="
      echo "  Job ID:     ${JOB_ID}"
      echo "  Status:     ${JOB_STATUS}"
      echo "==============================================="
      exit 1
      ;;
    *)
      printf "  Status: %s (attempt %d/60)\r" "$JOB_STATUS" "$i"
      sleep 2
      ;;
  esac
done

echo "TIMEOUT: Job did not complete within 120 seconds"
exit 1

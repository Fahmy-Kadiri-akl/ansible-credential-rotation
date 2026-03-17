#!/usr/bin/env bash
# ==============================================================================
# Akeyless Setup for Ansible Credential Rotation
#
# This script configures the Akeyless side of the integration:
#   1. Web Target pointing to the custom producer
#   2. Rotated secrets for Ansible user password and API keys
#   3. Event Center webhook forwarder to Ansible EDA
#
# Prerequisites:
#   - akeyless CLI authenticated (akeyless auth)
#   - Custom producer deployed and reachable from the gateway
#   - Ansible EDA endpoint reachable from the gateway
# ==============================================================================
set -euo pipefail

# ----- Configuration (edit these) -----
GATEWAY_URL="${AKEYLESS_GATEWAY_URL:-https://your-gateway.example.com:8000}"
PRODUCER_URL="${PRODUCER_URL:-http://ansible-cred-producer.akeyless-producers.svc.cluster.local:8080}"
EDA_WEBHOOK_URL="${EDA_WEBHOOK_URL:-http://ansible-eda.example.com:5000/endpoint}"
EDA_WEBHOOK_TOKEN="${EDA_WEBHOOK_TOKEN:-REPLACE_WITH_EDA_TOKEN}"
SECRETS_FOLDER="${SECRETS_FOLDER:-/Ansible/Credentials}"

# Ansible AAP/AWX details for the initial payload
ANSIBLE_URL="${ANSIBLE_URL:-https://ansible-controller.example.com}"
ANSIBLE_ADMIN_USER="${ANSIBLE_ADMIN_USER:-admin}"
ANSIBLE_ADMIN_PASSWORD="${ANSIBLE_ADMIN_PASSWORD:-REPLACE_ME}"
TARGET_USERNAME="${TARGET_USERNAME:-svc-server-build}"
TARGET_INITIAL_PASSWORD="${TARGET_INITIAL_PASSWORD:-REPLACE_ME}"
# ----- End Configuration -----

echo "=== Step 1: Create Web Target for custom producer ==="
akeyless target create web \
  --name "${SECRETS_FOLDER}/ansible-producer-target" \
  --url "${PRODUCER_URL}"
echo "Web target created: ${SECRETS_FOLDER}/ansible-producer-target"

echo ""
echo "=== Step 2: Create rotated secret for Ansible user password ==="
# The payload contains everything the custom producer needs to rotate
# the password on the Ansible controller.
PASSWORD_PAYLOAD=$(cat <<PAYLOAD
{
  "type": "password",
  "ansible_url": "${ANSIBLE_URL}",
  "admin_user": "${ANSIBLE_ADMIN_USER}",
  "admin_password": "${ANSIBLE_ADMIN_PASSWORD}",
  "target_username": "${TARGET_USERNAME}",
  "target_user_id": 0,
  "password": "${TARGET_INITIAL_PASSWORD}",
  "skip_tls_verify": true
}
PAYLOAD
)

akeyless rotated-secret create custom \
  --name "${SECRETS_FOLDER}/server-build-svc" \
  --gateway-url "${GATEWAY_URL}" \
  --target-name "${SECRETS_FOLDER}/ansible-producer-target" \
  --authentication-credentials use-user-creds \
  --rotator-type custom \
  --custom-payload "${PASSWORD_PAYLOAD}" \
  --auto-rotate true \
  --rotation-interval 7
echo "Rotated secret created: ${SECRETS_FOLDER}/server-build-svc (7-day rotation)"

echo ""
echo "=== Step 3: Create rotated secret for Ansible API token ==="
API_KEY_PAYLOAD=$(cat <<PAYLOAD
{
  "type": "api_key",
  "ansible_url": "${ANSIBLE_URL}",
  "admin_user": "${ANSIBLE_ADMIN_USER}",
  "admin_password": "${ANSIBLE_ADMIN_PASSWORD}",
  "target_user_id": 0,
  "token_id": 0,
  "token": "",
  "token_scope": "write",
  "description": "akeyless-managed-api-token",
  "skip_tls_verify": true
}
PAYLOAD
)

akeyless rotated-secret create custom \
  --name "${SECRETS_FOLDER}/api-token-eda" \
  --gateway-url "${GATEWAY_URL}" \
  --target-name "${SECRETS_FOLDER}/ansible-producer-target" \
  --authentication-credentials use-user-creds \
  --rotator-type custom \
  --custom-payload "${API_KEY_PAYLOAD}" \
  --auto-rotate true \
  --rotation-interval 7
echo "Rotated secret created: ${SECRETS_FOLDER}/api-token-eda (7-day rotation)"

echo ""
echo "=== Step 4: Create Event Center webhook forwarder to Ansible EDA ==="
akeyless event-forwarder create webhook \
  --name "ansible-eda-rotation-forwarder" \
  --gateway-url "${GATEWAY_URL}" \
  --url "${EDA_WEBHOOK_URL}" \
  --items-event-source-locations "${SECRETS_FOLDER}/*" \
  --event-types "rotated-secret-success,rotated-secret-failure" \
  --auth-type bearer-token \
  --auth-token "$(echo -n "${EDA_WEBHOOK_TOKEN}" | base64)" \
  --runner-type immediate
echo "Webhook forwarder created: ansible-eda-rotation-forwarder"

echo ""
echo "=== Setup complete ==="
echo ""
echo "Summary:"
echo "  Web Target:      ${SECRETS_FOLDER}/ansible-producer-target -> ${PRODUCER_URL}"
echo "  Password Secret:  ${SECRETS_FOLDER}/server-build-svc (rotates every 7 days)"
echo "  API Key Secret:   ${SECRETS_FOLDER}/api-token-eda (rotates every 7 days)"
echo "  Event Forwarder: ansible-eda-rotation-forwarder -> ${EDA_WEBHOOK_URL}"
echo ""
echo "Next steps:"
echo "  1. Verify the custom producer is reachable from the gateway"
echo "  2. Test rotation: akeyless rotated-secret rotate --name ${SECRETS_FOLDER}/server-build-svc"
echo "  3. Verify the EDA webhook endpoint receives events"
echo "  4. Deploy the EDA rulebook: ansible-rulebook --rulebook eda/rulebooks/akeyless-rotation.yml"

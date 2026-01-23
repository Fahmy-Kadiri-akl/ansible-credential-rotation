#!/bin/bash
# Development script for cred-server
# Builds and runs the server locally for fast iteration

set -e

PORT=${PORT:-8085}
# Gateway URL without /api/v2 - the SDK adds it automatically for non-SaaS endpoints
AKEYLESS_GATEWAY_URL=${AKEYLESS_GATEWAY_URL:-"https://akeyless.fklab.local"}
AKEYLESS_ACCESS_ID=${AKEYLESS_ACCESS_ID:-""}
AKEYLESS_ACCESS_KEY=${AKEYLESS_ACCESS_KEY:-""}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Cred Server Development Mode${NC}"
echo "================================"

# Kill any existing instance
if pgrep -f "cred-server-dev" > /dev/null; then
    echo -e "${YELLOW}Stopping existing instance...${NC}"
    pkill -f "cred-server-dev" || true
    sleep 1
fi

# Build
echo -e "${YELLOW}Building...${NC}"
go build -o cred-server-dev ./cmd/main.go

# Run
echo -e "${GREEN}Starting server on port ${PORT}${NC}"
echo -e "  Web UI:  http://localhost:${PORT}/"
echo -e "  Health:  http://localhost:${PORT}/health"
echo -e "  API:     http://localhost:${PORT}/api/v1/"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
echo ""

PORT=$PORT \
  AKEYLESS_GATEWAY_URL=$AKEYLESS_GATEWAY_URL \
  AKEYLESS_ACCESS_ID=$AKEYLESS_ACCESS_ID \
  AKEYLESS_ACCESS_KEY=$AKEYLESS_ACCESS_KEY \
  ./cred-server-dev

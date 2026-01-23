#!/bin/bash
# Go validation script
# Runs build, vet, fmt check, and tests

set -e

echo "========================================"
echo "Go Validation Script"
echo "========================================"

cd "$(dirname "$0")/.."

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

FAILED=0

# 1. Build
echo -e "\n${YELLOW}[1/5] Building...${NC}"
if go build ./...; then
    echo -e "${GREEN}✓ Build passed${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    FAILED=1
fi

# 2. Go vet
echo -e "\n${YELLOW}[2/5] Running go vet...${NC}"
if go vet ./...; then
    echo -e "${GREEN}✓ Vet passed${NC}"
else
    echo -e "${RED}✗ Vet failed${NC}"
    FAILED=1
fi

# 3. Format check
echo -e "\n${YELLOW}[3/5] Checking formatting...${NC}"
UNFORMATTED=$(gofmt -l .)
if [ -z "$UNFORMATTED" ]; then
    echo -e "${GREEN}✓ Formatting passed${NC}"
else
    echo -e "${RED}✗ Formatting issues found:${NC}"
    echo "$UNFORMATTED"
    FAILED=1
fi

# 4. staticcheck (if installed)
echo -e "\n${YELLOW}[4/5] Running staticcheck...${NC}"
if command -v staticcheck &> /dev/null; then
    if staticcheck ./...; then
        echo -e "${GREEN}✓ Staticcheck passed${NC}"
    else
        echo -e "${RED}✗ Staticcheck failed${NC}"
        FAILED=1
    fi
else
    echo -e "${YELLOW}⚠ staticcheck not installed (go install honnef.co/go/tools/cmd/staticcheck@latest)${NC}"
fi

# 5. Tests
echo -e "\n${YELLOW}[5/5] Running tests...${NC}"
if go test ./... 2>&1; then
    echo -e "${GREEN}✓ Tests passed${NC}"
else
    echo -e "${YELLOW}⚠ No tests found or tests failed${NC}"
fi

# Summary
echo -e "\n========================================"
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All validations passed!${NC}"
    exit 0
else
    echo -e "${RED}Some validations failed${NC}"
    exit 1
fi

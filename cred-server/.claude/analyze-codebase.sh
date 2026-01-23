#!/bin/bash
# Go Codebase Analyzer
# Scans existing code for simplification opportunities

set -e

echo "=========================================="
echo "  CRED-SERVER CODEBASE ANALYSIS"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

ISSUES=0
WARNINGS=0

# Find all Go files
GO_FILES=$(find . -name "*.go" -not -path "./.git/*" | sort)

echo -e "${BLUE}Analyzing $(echo "$GO_FILES" | wc -l) Go files...${NC}"
echo ""

# ============================================================================
# CHECK 1: File sizes
# ============================================================================
echo -e "${YELLOW}=== FILE SIZE ANALYSIS ===${NC}"
for file in $GO_FILES; do
    lines=$(wc -l < "$file")
    if [ "$lines" -gt 400 ]; then
        echo -e "${RED}LARGE FILE: $file ($lines lines)${NC}"
        ISSUES=$((ISSUES + 1))
    elif [ "$lines" -gt 300 ]; then
        echo -e "${YELLOW}GROWING FILE: $file ($lines lines)${NC}"
        WARNINGS=$((WARNINGS + 1))
    fi
done
echo ""

# ============================================================================
# CHECK 2: Function complexity (long functions)
# ============================================================================
echo -e "${YELLOW}=== FUNCTION LENGTH ANALYSIS ===${NC}"
for file in $GO_FILES; do
    # Simple line counting between func and closing brace
    awk '
    /^func / {
        func_start = NR;
        func_name = $0;
        sub(/\{.*/, "", func_name);
    }
    /^}$/ && func_start > 0 {
        length = NR - func_start;
        if (length > 50) {
            print "LONG FUNCTION (" length " lines): " FILENAME;
            print "  " func_name;
        }
        func_start = 0;
    }
    ' "$file"
done
echo ""

# ============================================================================
# CHECK 3: Error handling patterns
# ============================================================================
echo -e "${YELLOW}=== ERROR HANDLING ANALYSIS ===${NC}"
total_err_checks=0
for file in $GO_FILES; do
    count=$(grep -c "if err != nil" "$file" 2>/dev/null || echo 0)
    total_err_checks=$((total_err_checks + count))
    if [ "$count" -gt 10 ]; then
        echo -e "${YELLOW}MANY ERROR CHECKS: $file ($count checks)${NC}"
        WARNINGS=$((WARNINGS + 1))
    fi
done
echo "Total error checks across codebase: $total_err_checks"

# Check for ignored errors
ignored=$(grep -rn ", _ :=" $GO_FILES 2>/dev/null | grep -v "_test.go" | grep -v "range" || true)
if [ -n "$ignored" ]; then
    echo -e "${RED}IGNORED ERRORS FOUND:${NC}"
    echo "$ignored" | head -10
    ISSUES=$((ISSUES + $(echo "$ignored" | wc -l)))
fi
echo ""

# ============================================================================
# CHECK 4: Code duplication patterns
# ============================================================================
echo -e "${YELLOW}=== DUPLICATION ANALYSIS ===${NC}"

# JSON response pattern
json_encode=$(grep -rn "json.NewEncoder" $GO_FILES 2>/dev/null | wc -l)
echo "JSON encoding calls: $json_encode"
if [ "$json_encode" -gt 5 ]; then
    echo -e "${YELLOW}  -> Consider creating respondJSON() helper${NC}"
    WARNINGS=$((WARNINGS + 1))
fi

# HTTP error pattern
http_error=$(grep -rn "http.Error" $GO_FILES 2>/dev/null | wc -l)
echo "HTTP error calls: $http_error"
if [ "$http_error" -gt 5 ]; then
    echo -e "${YELLOW}  -> Consider creating errorResponse() helper${NC}"
    WARNINGS=$((WARNINGS + 1))
fi

# Log statements
log_calls=$(grep -rn "log\." $GO_FILES 2>/dev/null | wc -l)
echo "Log calls: $log_calls"
echo ""

# ============================================================================
# CHECK 5: Missing documentation
# ============================================================================
echo -e "${YELLOW}=== DOCUMENTATION ANALYSIS ===${NC}"
exported_funcs=$(grep -rn "^func [A-Z]" $GO_FILES 2>/dev/null | grep -v "_test.go" | wc -l)
commented_funcs=$(grep -rn -B1 "^func [A-Z]" $GO_FILES 2>/dev/null | grep -v "_test.go" | grep "^.*\.go-[0-9]*-//" | wc -l)
echo "Exported functions: $exported_funcs"
echo "With comments: $commented_funcs"
missing=$((exported_funcs - commented_funcs))
if [ "$missing" -gt 0 ]; then
    echo -e "${RED}Missing documentation: $missing functions${NC}"
    ISSUES=$((ISSUES + 1))
fi
echo ""

# ============================================================================
# CHECK 6: Package structure
# ============================================================================
echo -e "${YELLOW}=== PACKAGE STRUCTURE ===${NC}"
packages=$(find . -name "*.go" -not -path "./.git/*" -exec dirname {} \; | sort -u)
echo "Packages found:"
for pkg in $packages; do
    file_count=$(find "$pkg" -maxdepth 1 -name "*.go" | wc -l)
    echo "  $pkg ($file_count files)"
done
echo ""

# ============================================================================
# CHECK 7: Struct sizes
# ============================================================================
echo -e "${YELLOW}=== STRUCT ANALYSIS ===${NC}"
for file in $GO_FILES; do
    awk '
    /^type [A-Za-z]+ struct/ {
        struct_name = $2;
        in_struct = 1;
        field_count = 0;
    }
    in_struct && /^[[:space:]]+[A-Z]/ {
        field_count++;
    }
    in_struct && /^}/ {
        if (field_count > 10) {
            print "LARGE STRUCT: " struct_name " (" field_count " fields) in " FILENAME;
        }
        in_struct = 0;
    }
    ' "$file"
done
echo ""

# ============================================================================
# CHECK 8: Magic numbers
# ============================================================================
echo -e "${YELLOW}=== MAGIC NUMBER ANALYSIS ===${NC}"
magic_numbers=$(grep -rn "[^a-zA-Z0-9_][3-9][0-9]*[^0-9a-zA-Z_]" $GO_FILES 2>/dev/null | \
    grep -v "_test.go" | \
    grep -v "const " | \
    grep -v "// " | \
    grep -v "http.Status" | \
    grep -v ":8" | \
    grep -v "time\." | \
    head -10 || true)
if [ -n "$magic_numbers" ]; then
    echo -e "${YELLOW}Potential magic numbers:${NC}"
    echo "$magic_numbers"
    WARNINGS=$((WARNINGS + 1))
fi
echo ""

# ============================================================================
# CHECK 9: Handler complexity
# ============================================================================
echo -e "${YELLOW}=== HANDLER COMPLEXITY ===${NC}"
for file in $GO_FILES; do
    if grep -q "http.ResponseWriter" "$file"; then
        handler_count=$(grep -c "func.*http.ResponseWriter" "$file" 2>/dev/null || echo 0)
        echo "$file: $handler_count handlers"
        if [ "$handler_count" -gt 5 ]; then
            echo -e "${YELLOW}  -> Consider splitting into multiple handler files${NC}"
            WARNINGS=$((WARNINGS + 1))
        fi
    fi
done
echo ""

# ============================================================================
# SUMMARY
# ============================================================================
echo "=========================================="
echo -e "${BLUE}  ANALYSIS SUMMARY${NC}"
echo "=========================================="
echo -e "Issues found: ${RED}$ISSUES${NC}"
echo -e "Warnings: ${YELLOW}$WARNINGS${NC}"
echo ""

if [ $ISSUES -gt 0 ] || [ $WARNINGS -gt 0 ]; then
    echo "Recommendations:"
    echo "  1. Split large files into focused modules"
    echo "  2. Extract common patterns into helpers"
    echo "  3. Add documentation for exported functions"
    echo "  4. Replace magic numbers with named constants"
    echo "  5. Group related handlers"
fi

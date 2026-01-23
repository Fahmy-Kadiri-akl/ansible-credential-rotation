#!/bin/bash
# Go Coding Standards Validator
# Version: 1.0.0
# Adapted from SOL Python validator for Go projects
#
# BLOCKING RULES:
# - File organization (logical package structure)
# - File size (max 400 lines per file)
# - Security (no hardcoded secrets, dangerous commands)
# - Error handling (no ignored errors, proper wrapping)
# - Code quality (no magic numbers, proper naming)
# - Go idioms (no panic in library code, proper defer usage)
# - Documentation (exported functions need comments)
# - Simplicity (no over-engineering, single responsibility)
# - DRY (no duplicated code patterns)
#
# Dependencies: go, gofmt, staticcheck (optional)

set -e

# Read JSON input from stdin
INPUT=$(cat)

# Extract tool information
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty')

# Extract common fields
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // .tool_input.path // empty')
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // .tool_input.new_string // empty')
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Tracking
BLOCKED=false
declare -a VIOLATIONS=()

# ============================================================================
# RULE 1: FILE SIZE - Max 400 lines for Go files
# ============================================================================
validate_file_size() {
    [ -z "$CONTENT" ] && return 0
    [ -z "$FILE_PATH" ] && return 0

    # Only check Go files
    [[ "$FILE_PATH" != *.go ]] && return 0

    local line_count=$(echo "$CONTENT" | wc -l)

    if [ "$line_count" -gt 400 ]; then
        VIOLATIONS+=("FILE SIZE: File has $line_count lines (max: 400)")
        VIOLATIONS+=("  -> Split into smaller, focused files")
        VIOLATIONS+=("  -> Extract types to separate files")
        VIOLATIONS+=("  -> Group related handlers in their own package")
        BLOCKED=true
    fi
}

# ============================================================================
# RULE 2: SECURITY - No hardcoded secrets
# ============================================================================
validate_security() {
    [ -z "$CONTENT" ] && return 0

    local secret_patterns=(
        'sk-[a-zA-Z0-9]{20,}'
        'ghp_[a-zA-Z0-9]{36}'
        'gho_[a-zA-Z0-9]{36}'
        'xox[baprs]-[a-zA-Z0-9-]+'
        'AKIA[0-9A-Z]{16}'
        'password["\x27]?\s*[:=]\s*["\x27][^"\x27]{8,}'
        'api[_-]?key["\x27]?\s*[:=]\s*["\x27][^"\x27]{16,}'
        'secret["\x27]?\s*[:=]\s*["\x27][^"\x27]{16,}'
        'token["\x27]?\s*[:=]\s*["\x27][^"\x27]{16,}'
    )

    for pattern in "${secret_patterns[@]}"; do
        if echo "$CONTENT" | grep -qiE "$pattern"; then
            if echo "$CONTENT" | grep -q "os.Getenv\|viper\|\.env"; then
                continue
            fi
            VIOLATIONS+=("SECURITY: Potential hardcoded secret detected")
            VIOLATIONS+=("  -> Use environment variables: os.Getenv()")
            VIOLATIONS+=("  -> Or use viper/config files")
            BLOCKED=true
            break
        fi
    done
}

# ============================================================================
# RULE 3: SECURITY - No dangerous commands
# ============================================================================
validate_dangerous_commands() {
    [ -z "$COMMAND" ] && return 0

    local dangerous_patterns=(
        'rm -rf /'
        'rm -rf /\*'
        'dd if=/dev/zero'
        'curl.*\|.*sh'
        'curl.*\|.*bash'
    )

    for pattern in "${dangerous_patterns[@]}"; do
        if echo "$COMMAND" | grep -qE "$pattern"; then
            VIOLATIONS+=("SECURITY: Dangerous command blocked: $pattern")
            BLOCKED=true
            return
        fi
    done
}

# ============================================================================
# RULE 4: ERROR HANDLING - No ignored errors
# ============================================================================
validate_error_handling() {
    [ -z "$CONTENT" ] && return 0
    [ -z "$FILE_PATH" ] && return 0

    [[ "$FILE_PATH" != *.go ]] && return 0

    # Pattern 1: Ignored error from function call (result, _ := func())
    local ignored_errors=$(echo "$CONTENT" | grep -E ",[[:space:]]*_[[:space:]]*:?=[[:space:]]" | \
        grep -v "range" | grep -v "for" | head -1)

    if [ -n "$ignored_errors" ]; then
        VIOLATIONS+=("ERROR HANDLING: Ignored error detected")
        VIOLATIONS+=("  Found: $(echo "$ignored_errors" | sed 's/^[[:space:]]*//' | cut -c1-60)")
        VIOLATIONS+=("  -> Handle all errors: if err != nil { return err }")
        VIOLATIONS+=("  -> Or wrap with context: fmt.Errorf(\"context: %w\", err)")
        BLOCKED=true
    fi

    # Pattern 2: Empty error check (if err != nil { })
    local empty_error_check=$(echo "$CONTENT" | grep -A1 "if err != nil" | grep -E "^[[:space:]]*\}[[:space:]]*$" | head -1)

    if [ -n "$empty_error_check" ]; then
        VIOLATIONS+=("ERROR HANDLING: Empty error check detected")
        VIOLATIONS+=("  -> Always handle errors: log, return, or wrap")
        VIOLATIONS+=("  -> Never silently ignore errors")
        BLOCKED=true
    fi
}

# ============================================================================
# RULE 5: NO PANIC IN LIBRARY CODE
# ============================================================================
validate_no_panic() {
    [ -z "$CONTENT" ] && return 0
    [ -z "$FILE_PATH" ] && return 0

    [[ "$FILE_PATH" != *.go ]] && return 0

    # Skip main.go and test files
    local filename=$(basename "$FILE_PATH")
    [[ "$filename" == "main.go" || "$filename" == *_test.go ]] && return 0

    # Check for panic() calls
    local panic_calls=$(echo "$CONTENT" | grep -E "^[[:space:]]*panic\(" | head -1)

    if [ -n "$panic_calls" ]; then
        VIOLATIONS+=("GO IDIOM: panic() in library code")
        VIOLATIONS+=("  Found: $(echo "$panic_calls" | sed 's/^[[:space:]]*//' | cut -c1-60)")
        VIOLATIONS+=("  -> Return errors instead of panicking")
        VIOLATIONS+=("  -> panic() is only appropriate in main() or init()")
        BLOCKED=true
    fi
}

# ============================================================================
# RULE 6: PROPER DEFER USAGE
# ============================================================================
validate_defer_usage() {
    [ -z "$CONTENT" ] && return 0
    [ -z "$FILE_PATH" ] && return 0

    [[ "$FILE_PATH" != *.go ]] && return 0

    # Check for defer in loop (common bug)
    local defer_in_loop=$(echo "$CONTENT" | grep -B5 "defer " | grep -E "for .* range|for .*;.*;")

    if [ -n "$defer_in_loop" ]; then
        VIOLATIONS+=("GO IDIOM: defer inside loop detected")
        VIOLATIONS+=("  -> defer runs at function exit, not loop iteration")
        VIOLATIONS+=("  -> Move resource cleanup inside loop or extract to function")
        BLOCKED=true
    fi

    # Check for missing defer after resource acquisition
    local file_open=$(echo "$CONTENT" | grep -E "os\.Open\(|os\.Create\(" | head -1)
    if [ -n "$file_open" ]; then
        local has_defer_close=$(echo "$CONTENT" | grep -E "defer.*\.Close\(\)")
        if [ -z "$has_defer_close" ]; then
            VIOLATIONS+=("GO IDIOM: File opened without defer Close()")
            VIOLATIONS+=("  -> Always: defer file.Close() after os.Open/Create")
            BLOCKED=true
        fi
    fi
}

# ============================================================================
# RULE 7: EXPORTED FUNCTIONS NEED COMMENTS
# ============================================================================
validate_exported_comments() {
    [ -z "$CONTENT" ] && return 0
    [ -z "$FILE_PATH" ] && return 0

    [[ "$FILE_PATH" != *.go ]] && return 0

    # Skip test files
    local filename=$(basename "$FILE_PATH")
    [[ "$filename" == *_test.go ]] && return 0

    # Find exported functions without preceding comment
    local line_num=0
    local prev_line=""

    while IFS= read -r line; do
        line_num=$((line_num + 1))

        # Check for exported function (starts with capital letter)
        if echo "$line" | grep -qE "^func[[:space:]]+[A-Z]|^func[[:space:]]+\([^)]+\)[[:space:]]+[A-Z]"; then
            # Check if previous non-empty line is a comment
            if ! echo "$prev_line" | grep -qE "^//"; then
                local func_name=$(echo "$line" | grep -oE "func[[:space:]]+(\([^)]+\)[[:space:]]+)?[A-Z][a-zA-Z0-9_]*" | head -1)
                VIOLATIONS+=("DOCUMENTATION: Exported function missing comment")
                VIOLATIONS+=("  Line $line_num: $(echo "$line" | sed 's/^[[:space:]]*//' | cut -c1-60)")
                VIOLATIONS+=("  -> Add: // FuncName does X and returns Y")
                BLOCKED=true
                break
            fi
        fi

        # Track previous non-empty line
        if [ -n "$(echo "$line" | tr -d '[:space:]')" ]; then
            prev_line="$line"
        fi
    done <<< "$CONTENT"
}

# ============================================================================
# RULE 8: MAGIC NUMBERS
# ============================================================================
validate_magic_numbers() {
    [ -z "$CONTENT" ] && return 0
    [ -z "$FILE_PATH" ] && return 0

    [[ "$FILE_PATH" != *.go ]] && return 0

    # Skip test files and config files
    local filename=$(basename "$FILE_PATH")
    [[ "$filename" == *_test.go || "$filename" == *config* ]] && return 0

    # Remove comments and strings
    local code_only=$(echo "$CONTENT" | \
        sed 's|//.*$||' | \
        sed 's|/\*.*\*/||g' | \
        sed 's/"[^"]*"//g' | \
        sed "s/'[^']*'//g")

    # Find magic numbers (excluding 0, 1, 2, -1 and common ports)
    local magic=$(echo "$code_only" | grep -oE '[^a-zA-Z0-9_]([3-9]|[1-9][0-9]+)[^0-9a-zA-Z_]' | \
        grep -vE '80|443|8080|8000|8443|3000|5000|6379|27017|5432|3306' | \
        head -1)

    if [ -n "$magic" ]; then
        VIOLATIONS+=("MAGIC NUMBER: Unexplained numeric literal")
        VIOLATIONS+=("  Found: $magic")
        VIOLATIONS+=("  -> Define as named constant: const MaxRetries = 3")
        VIOLATIONS+=("  -> Or add comment explaining the value")
        BLOCKED=true
    fi
}

# ============================================================================
# RULE 9: FUNCTION LENGTH
# ============================================================================
validate_function_length() {
    [ -z "$CONTENT" ] && return 0
    [ -z "$FILE_PATH" ] && return 0

    [[ "$FILE_PATH" != *.go ]] && return 0

    # Count lines in each function (simplified check)
    local in_func=false
    local func_start=0
    local func_name=""
    local brace_count=0
    local line_num=0

    while IFS= read -r line; do
        line_num=$((line_num + 1))

        # Function start
        if echo "$line" | grep -qE "^func[[:space:]]"; then
            in_func=true
            func_start=$line_num
            func_name=$(echo "$line" | grep -oE "func[[:space:]]+(\([^)]+\)[[:space:]]+)?[a-zA-Z_][a-zA-Z0-9_]*" | sed 's/func[[:space:]]*//')
            brace_count=0
        fi

        if [ "$in_func" = true ]; then
            # Count braces
            local opens=$(echo "$line" | grep -o "{" | wc -l)
            local closes=$(echo "$line" | grep -o "}" | wc -l)
            brace_count=$((brace_count + opens - closes))

            # Function end
            if [ $brace_count -eq 0 ] && echo "$line" | grep -q "}"; then
                local func_length=$((line_num - func_start))
                if [ $func_length -gt 50 ]; then
                    VIOLATIONS+=("FUNCTION LENGTH: Function too long ($func_length lines)")
                    VIOLATIONS+=("  Function: $func_name (lines $func_start-$line_num)")
                    VIOLATIONS+=("  -> Max recommended: 50 lines")
                    VIOLATIONS+=("  -> Extract helper functions for clarity")
                    BLOCKED=true
                    break
                fi
                in_func=false
            fi
        fi
    done <<< "$CONTENT"
}

# ============================================================================
# RULE 10: DRY - No duplicated patterns
# ============================================================================
validate_dry() {
    [ -z "$CONTENT" ] && return 0
    [ -z "$FILE_PATH" ] && return 0

    [[ "$FILE_PATH" != *.go ]] && return 0

    # Check for repeated error handling patterns (3+ similar blocks)
    local error_blocks=$(echo "$CONTENT" | grep -c "if err != nil" || true)

    if [ "$error_blocks" -gt 10 ]; then
        VIOLATIONS+=("DRY: Excessive error handling blocks ($error_blocks)")
        VIOLATIONS+=("  -> Consider wrapping repetitive operations")
        VIOLATIONS+=("  -> Use helper functions to reduce boilerplate")
        BLOCKED=true
    fi

    # Check for repeated JSON encoding patterns
    local json_encode=$(echo "$CONTENT" | grep -c "json.NewEncoder\|json.Marshal" || true)
    if [ "$json_encode" -gt 5 ]; then
        VIOLATIONS+=("DRY: Multiple JSON encoding calls ($json_encode)")
        VIOLATIONS+=("  -> Create a helper: respondJSON(w, status, data)")
        BLOCKED=true
    fi
}

# ============================================================================
# RULE 11: HTTP HANDLER PATTERNS
# ============================================================================
validate_http_handlers() {
    [ -z "$CONTENT" ] && return 0
    [ -z "$FILE_PATH" ] && return 0

    [[ "$FILE_PATH" != *.go ]] && return 0

    # Check for http.Error without return
    local http_error=$(echo "$CONTENT" | grep -B1 "http.Error(" | grep -v "return" | grep "http.Error" | head -1)

    if [ -n "$http_error" ]; then
        VIOLATIONS+=("HTTP HANDLER: http.Error without return")
        VIOLATIONS+=("  Found: $(echo "$http_error" | sed 's/^[[:space:]]*//' | cut -c1-60)")
        VIOLATIONS+=("  -> Always return after http.Error()")
        BLOCKED=true
    fi

    # Check for missing Content-Type header before JSON response
    local json_response=$(echo "$CONTENT" | grep -c "json.NewEncoder(w)" || true)
    local content_type=$(echo "$CONTENT" | grep -c 'w.Header().Set("Content-Type"' || true)

    if [ "$json_response" -gt 0 ] && [ "$content_type" -lt "$json_response" ]; then
        VIOLATIONS+=("HTTP HANDLER: JSON response without Content-Type header")
        VIOLATIONS+=("  -> Add: w.Header().Set(\"Content-Type\", \"application/json\")")
        BLOCKED=true
    fi
}

# ============================================================================
# RULE 12: STRUCT ORGANIZATION
# ============================================================================
validate_struct_organization() {
    [ -z "$CONTENT" ] && return 0
    [ -z "$FILE_PATH" ] && return 0

    [[ "$FILE_PATH" != *.go ]] && return 0

    # Check for very large structs (>15 fields)
    local in_struct=false
    local field_count=0
    local struct_name=""

    while IFS= read -r line; do
        if echo "$line" | grep -qE "^type[[:space:]]+[A-Za-z]+[[:space:]]+struct"; then
            in_struct=true
            field_count=0
            struct_name=$(echo "$line" | grep -oE "type[[:space:]]+[A-Za-z]+" | sed 's/type[[:space:]]*//')
        elif [ "$in_struct" = true ]; then
            if echo "$line" | grep -qE "^}"; then
                if [ $field_count -gt 15 ]; then
                    VIOLATIONS+=("STRUCT ORGANIZATION: Struct too large ($field_count fields)")
                    VIOLATIONS+=("  Struct: $struct_name")
                    VIOLATIONS+=("  -> Consider breaking into smaller embedded structs")
                    VIOLATIONS+=("  -> Or grouping related fields")
                    BLOCKED=true
                fi
                in_struct=false
            elif echo "$line" | grep -qE "^[[:space:]]+[A-Za-z]"; then
                field_count=$((field_count + 1))
            fi
        fi
    done <<< "$CONTENT"
}

# ============================================================================
# MAIN EXECUTION
# ============================================================================

case "$TOOL_NAME" in
    Write|Edit|MultiEdit)
        validate_file_size
        validate_security
        validate_error_handling
        validate_no_panic
        validate_defer_usage
        validate_exported_comments
        validate_magic_numbers
        validate_function_length
        validate_dry
        validate_http_handlers
        validate_struct_organization
        ;;
    Bash)
        validate_dangerous_commands
        ;;
esac

# ============================================================================
# REPORT RESULTS
# ============================================================================

if [ ${#VIOLATIONS[@]} -gt 0 ]; then
    echo "" >&2
    echo "============================================" >&2
    echo "  GO CODING STANDARDS VIOLATION - BLOCKED" >&2
    echo "============================================" >&2
    for violation in "${VIOLATIONS[@]}"; do
        echo "$violation" >&2
    done
    echo "============================================" >&2
    echo "" >&2
fi

if [ "$BLOCKED" = true ]; then
    exit 2
fi

# Pass through input for non-blocking
echo "$INPUT"
exit 0

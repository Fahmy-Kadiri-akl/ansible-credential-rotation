# Claude Code Guidelines for cred-server

## Constant Naming Convention

**Format**: `DOMAIN__CONTEXT__THING__UNIT__QUALIFIER`

All constants use uppercase with double underscores (`__`) separating hierarchical components.

### Examples

```go
// HTTP Configuration
HTTP__TIMEOUT__CLIENT__SECONDS__DEFAULT      = 30 * time.Second
HTTP__TIMEOUT__DISCOVERY__SECONDS__DEFAULT   = 10 * time.Second

// Size Limits
LIMIT__FORM__SIZE__BYTES__MAX       = 10 << 20
LIMIT__BODY__TRUNCATE__CHARS__MAX   = 10000

// Credential Generation
CREDENTIAL__ID__LENGTH__BYTES__DEFAULT       = 8
CREDENTIAL__TOKEN__LENGTH__CHARS__DEFAULT    = 40
CREDENTIAL__PASSWORD__LENGTH__CHARS__DEFAULT = 32

// Confidence Scoring
CONFIDENCE__THRESHOLD__SCORE__MIN  = 0.3
CONFIDENCE__THRESHOLD__SCORE__HIGH = 0.7

// Authentication Types
AUTH__TYPE__BEARER     = "bearer"
AUTH__TYPE__BASIC      = "basic"

// Indicators (slices/maps)
INDICATOR__CREDENTIAL__KEYWORDS = []string{...}
INDICATOR__PATH__CREATE         = map[string]float64{...}

// Scoring Weights
SCORE__WEIGHT__CREDENTIAL_IN_RESPONSE = 0.4
SCORE__PENALTY__LOGIN_PATH            = -0.3
```

### Component Breakdown

| Component | Purpose | Examples |
|-----------|---------|----------|
| DOMAIN | Top-level category | `HTTP`, `LIMIT`, `CREDENTIAL`, `AUTH`, `SCORE` |
| CONTEXT | Sub-category or usage | `TIMEOUT`, `FORM`, `TOKEN`, `TYPE`, `WEIGHT` |
| THING | What it describes | `CLIENT`, `SIZE`, `LENGTH`, `THRESHOLD` |
| UNIT | Unit of measurement | `SECONDS`, `BYTES`, `CHARS`, `SCORE` |
| QUALIFIER | Final distinction | `DEFAULT`, `MAX`, `MIN`, `HIGH`, `LOW` |

### Rules

1. **ALL_CAPS** with double underscores
2. Include units where applicable (`SECONDS`, `BYTES`, `CHARS`)
3. End with qualifier (`DEFAULT`, `MAX`, `MIN`)
4. Group related constants in `const ()` blocks
5. All constants live in `internal/constants/constants.go`

## Code Standards

### No Magic Numbers

```go
// ❌ WRONG
if score > 0.7 {
    confidence = "high"
}
time.Sleep(30 * time.Second)

// ✅ GOOD
if score > constants.CONFIDENCE__THRESHOLD__SCORE__HIGH {
    confidence = constants.CONFIDENCE__LABEL__HIGH
}
time.Sleep(constants.HTTP__TIMEOUT__CLIENT__SECONDS__DEFAULT)
```

### SSOT (Single Source of Truth)

- All configuration values in `internal/constants/`
- No duplicate definitions across files
- Helper functions in constants package for common checks

### Error Handling

- Never ignore errors from `rand.Read()`, `io.ReadAll()`, etc.
- Log errors even if handling gracefully

### HTTP Methods

Use `net/http` constants:
```go
// ❌ WRONG
if req.Method == "POST" { ... }

// ✅ GOOD
if req.Method == http.MethodPost { ... }
```

## Project Structure

```
cred-server/
├── cmd/main.go              # Entry point
├── internal/
│   ├── analysis/            # Traffic analysis logic
│   ├── config/              # Configuration loading
│   ├── constants/           # All constants (SSOT)
│   ├── handler/             # HTTP handlers
│   ├── har/                 # HAR file types
│   ├── producer/            # Producer registry
│   └── rewrite/             # URL rewriting
└── CLAUDE.md                # This file
```

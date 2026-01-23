# Cred-Server Simplification Plan

## Current State Analysis

### File Sizes (CRITICAL ISSUES)
| File | Lines | Status |
|------|-------|--------|
| proxy.go | 1236 | 🔴 Extremely large |
| management.go | 879 | 🔴 Very large |
| analyzer.go | 554 | 🔴 Large |
| sync.go | 535 | 🔴 Large |
| akeyless.go | 243 | 🟡 Acceptable |
| discovery.go | 67 | 🟢 Good |
| **Total** | **3514** | |

**Target**: Max 400 lines per file

### Code Duplication
- JSON encoding: 21 calls across files
- HTTP errors: 39 calls across files
- Error checks: 31 `if err != nil` blocks

### Key Problems

1. **proxy.go (1236 lines)** - Monolithic file with mixed concerns:
   - Session management (start/stop/get)
   - HTTP proxy logic
   - Traffic analysis
   - API spec discovery
   - Endpoint scoring algorithms
   - URL rewriting utilities
   - Body sanitization

2. **management.go (879 lines)** - Contains embedded HTML:
   - ~200 lines of handler code
   - ~680 lines of embedded HTML/CSS/JS template
   - Should use Go embed or separate static files

3. **Repeated Patterns**:
   - No shared response helpers
   - No shared error handling
   - Each handler does its own JSON encoding

---

## Simplification Plan

### Phase 1: Extract Common Utilities

Create `internal/handler/response.go`:
```go
// respondJSON writes JSON response with proper headers
func respondJSON(w http.ResponseWriter, status int, data interface{})

// respondError writes error response
func respondError(w http.ResponseWriter, status int, message string)

// decodeJSON decodes request body to struct
func decodeJSON(r *http.Request, v interface{}) error
```

**Impact**: Remove ~50 lines of boilerplate

### Phase 2: Extract UI to Embedded Files

Create `internal/handler/ui/` directory:
```
ui/
├── index.html
├── styles.css
└── app.js
```

Use Go 1.16+ embed:
```go
//go:embed ui/*
var uiFS embed.FS
```

**Impact**: Remove ~680 lines from management.go

### Phase 3: Split proxy.go

New file structure:
```
internal/
├── handler/
│   ├── proxy.go          (~150 lines - session CRUD + proxy handler)
│   ├── proxy_session.go  (~100 lines - RecordingSession type + methods)
│   └── response.go       (~50 lines - shared response helpers)
├── analysis/
│   ├── traffic.go        (~200 lines - analyzeTraffic)
│   ├── scoring.go        (~200 lines - endpoint scoring)
│   └── discovery.go      (~150 lines - API spec discovery)
└── rewrite/
    ├── url.go            (~100 lines - URL rewriting)
    └── body.go           (~100 lines - body rewriting/sanitization)
```

**Impact**: proxy.go reduced from 1236 → ~150 lines

### Phase 4: Split sync.go

New structure:
```
internal/handler/
├── sync.go           (~100 lines - handler routing)
├── sync_create.go    (~150 lines - create handler)
├── sync_revoke.go    (~100 lines - revoke handler)
└── sync_rotate.go    (~100 lines - rotate handler)
```

**Impact**: Each file focused on single operation

### Phase 5: Extract Analyzer Logic

```
internal/analysis/
├── har.go            (~200 lines - HAR file parsing)
├── flow.go           (~150 lines - flow detection)
└── generator.go      (~150 lines - producer generation)
```

**Impact**: analyzer.go reduced from 554 → ~100 lines

---

## Priority Order

1. **HIGH**: Extract response helpers (immediate DRY win)
2. **HIGH**: Extract embedded HTML (biggest single reduction)
3. **MEDIUM**: Split proxy.go (complex but valuable)
4. **LOW**: Split sync.go and analyzer.go

---

## Metrics After Simplification

| File | Before | After | Reduction |
|------|--------|-------|-----------|
| proxy.go | 1236 | ~150 | 88% |
| management.go | 879 | ~200 | 77% |
| analyzer.go | 554 | ~100 | 82% |
| sync.go | 535 | ~100 | 81% |

**New files**: 10-12 focused modules, each < 200 lines

---

## Implementation Notes

- Use Go interfaces for testability
- Add proper godoc comments during refactor
- Keep backward compatibility (same API endpoints)
- Add unit tests for extracted modules

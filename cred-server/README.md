# Akeyless Credential Server

**Automated credential endpoint discovery and Akeyless dynamic secret integration**

## What It Does

The Cred Server automatically discovers credential management endpoints (CREATE, REVOKE, ROTATE) from any application by recording and analyzing API traffic. It then creates Akeyless custom producers for dynamic secret management.

## Features

- 🔍 **Smart Discovery**: Automatically detects credential endpoints regardless of naming
- 🎯 **Pattern Recognition**: Finds CREATE/REVOKE/ROTATE equivalents using ML-style scoring
- 🌐 **Proxy Recorder**: Records API traffic through transparent proxy
- 🔴 **Visual Feedback**: Button turns red during recording, green when done
- ⚡ **One-Click Deploy**: Instantly create Akeyless dynamic secrets
- 📊 **Debug Tools**: Built-in debugging endpoint for troubleshooting

## Quick Start

### 1. Deploy

```bash
./deploy.sh
```

### 2. Discover Credentials

1. Open: `https://cred-server.fklab.local`
2. Enter application URL (e.g., `https://argo.fklab.local`)
3. Click **"🔍 Discover Application"** (button turns 🔴 RED)
4. Click the proxy link to access your app
5. Perform credential operations:
   - Create a new API token
   - Delete/revoke a token
   - Rotate/update credentials
6. Click the 🔴 RED button to stop (turns 🟢 GREEN)
7. Review discovered endpoints
8. Click **"➕ Create Producer"**
9. Click **"Deploy to Akeyless"**

### 3. Use Dynamic Secret

```bash
akeyless dynamic-secret get-value --name /Dynamic/your-producer-id
```

## How Discovery Works

### Intelligent Pattern Detection

The system scores each request to determine if it's a credential operation:

**CREATE Detection** (Score: 0.0 - 1.0)
- POST method with credential-related response
- Paths: `/tokens`, `/api_keys`, `/credentials`, `/service_accounts`
- Response contains: `token`, `api_key`, `secret`, `password`
- No ID in path (creating new, not updating)

**REVOKE Detection**
- DELETE to credential paths
- POST/PUT with `revoke`, `delete`, `disable` in path/body
- Path contains ID pattern

**ROTATE Detection**
- PUT/PATCH to credential/password endpoints
- POST with `rotate`, `renew`, `refresh`, `reset`
- Request/response contains password fields

### Examples

```
✅ POST /api/v4/personal_access_tokens → CREATE (score: 0.9)
✅ DELETE /api/tokens/123 → REVOKE (score: 0.9)
✅ PUT /user/password → ROTATE (score: 0.8)
✅ POST /api/keys/rotate → ROTATE (score: 0.9)
❌ POST /api/login → Not credential (excluded)
```

## Architecture

```
┌─────────────┐
│   Browser   │
│   (User)    │
└──────┬──────┘
       │ 1. Enter URL
       ↓
┌─────────────────────┐
│   Cred Server UI    │
│  🔵 → 🔴 → 🟢      │ Button State Machine
└──────┬──────────────┘
       │ 2. Start Recording
       ↓
┌─────────────────────┐
│  Proxy Recorder     │
│  (Session: abc123)  │ Records all traffic
└──────┬──────────────┘
       │ 3. Proxied requests
       ↓
┌─────────────────────┐
│  Target App         │
│  (argo.fklab.local) │ User performs operations
└─────────────────────┘
       │ 4. Responses captured
       ↓
┌─────────────────────┐
│  Traffic Analyzer   │
│  Score-based ML     │ Detects patterns
└──────┬──────────────┘
       │ 5. Detected endpoints
       ↓
┌─────────────────────┐
│  Producer Config    │
│  Generated          │ CREATE/REVOKE/ROTATE
└──────┬──────────────┘
       │ 6. Deploy
       ↓
┌─────────────────────┐
│  Akeyless Gateway   │
│  Dynamic Secret     │ Ready to use!
└─────────────────────┘
```

## API Reference

### Recording Sessions

**Start Recording**
```bash
POST /api/v1/sessions
Content-Type: application/json

{
  "target_url": "https://argo.fklab.local"
}

Response:
{
  "session_id": "abc123",
  "proxy_url": "/proxy/abc123"
}
```

**Access Through Proxy**
```
https://cred-server.fklab.local/proxy/abc123/
```

**Stop & Analyze**
```bash
POST /api/v1/sessions/abc123/stop

Response:
{
  "success": true,
  "message": "Analysis complete",
  "create_endpoint": {
    "path": "/api/v4/personal_access_tokens",
    "method": "POST",
    "confidence": "high"
  },
  "revoke_endpoint": {
    "path": "/api/v4/personal_access_tokens/{id}",
    "method": "DELETE",
    "confidence": "high"
  }
}
```

**Debug Session** 🆕
```bash
GET /api/v1/sessions/abc123/debug

Response:
{
  "session_id": "abc123",
  "target_url": "https://argo.fklab.local",
  "active": true,
  "total_requests": 47,
  "request_groups": {
    "html": ["GET /"],
    "js": ["GET /assets/main.js", ...],
    "api": ["POST /api/tokens", ...]
  }
}
```

### Producer Management

**Create Producer**
```bash
POST /api/v1/producers
Content-Type: application/json

{
  "name": "ArgoCD Credentials",
  "type": "rest-api",
  "target_url": "https://argo.fklab.local",
  "config": {
    "auth_type": "bearer",
    "create_endpoint": {
      "path": "/api/tokens",
      "method": "POST"
    }
  }
}
```

**Deploy to Akeyless**
```bash
POST /api/v1/producers/{id}/deploy

{
  "secret_path": "/Dynamic/argocd-tokens",
  "ttl": "1h"
}
```

## Troubleshooting

### Proxy Shows Blank Screen

**Quick Debug**
```bash
# Check what's being captured
curl -s "https://cred-server.fklab.local/api/v1/sessions/abc123/debug" -k | jq

# Look for:
# - html: [] → No HTML loaded (bad)
# - js: [] → No JavaScript loaded (bad)
# - total_requests: 0 → Proxy not working (bad)
```

**Common Fixes**
1. **Add trailing slash**: `https://cred-server.fklab.local/proxy/abc123/`
2. **Check browser console** (F12) for errors
3. **Try base path**: If app uses `/app`, try `/proxy/abc123/app/`

See [TESTING.md](TESTING.md) for comprehensive troubleshooting guide.

### No Endpoints Detected

**Check captured traffic**
```bash
curl -s "https://cred-server.fklab.local/api/v1/sessions/abc123" -k | \
  jq '.requests[] | select(.method == "POST") | {path, status}'
```

**Ensure you performed operations**
- Actually **create** a token (not just view tokens)
- Actually **delete** a token if possible
- Check if operations returned 200 status

### Deployment Fails

**Error: producer_id required**
- Fixed in latest version
- Fallback logic automatically handles this

**Error: Connection refused**
```bash
# Verify sync URL is reachable from Akeyless Gateway
kubectl exec -n infra-security deployment/akeyless-gateway -- \
  curl -v http://cred-server.cred-server.svc.cluster.local:8080/sync/create
```

## Configuration

### Environment Variables

```bash
PORT=8080                                    # Server port
AKEYLESS_GATEWAY_URL=https://gateway.local  # Akeyless Gateway
AKEYLESS_ACCESS_ID=p-abc123                 # For deployment
ALLOWED_ACCESS_IDS=p-abc123,p-def456        # Whitelist (optional)
LOG_FORMAT=json                             # json or console
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cred-server
  namespace: cred-server
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: cred-server
        image: localhost:32000/cred-server:latest
        ports:
        - containerPort: 8080
        env:
        - name: AKEYLESS_GATEWAY_URL
          value: "http://akeyless-gateway.infra-security.svc.cluster.local:8000"
```

## Security

### Authentication

Currently **disabled for MVP**. For production:
- Add API key authentication
- Validate Akeyless auth tokens
- Restrict allowed target URLs
- Enable TLS verification

### Network Access

Required connections:
- **Inbound**: User browser → Cred Server (HTTPS)
- **Outbound**: Cred Server → Target applications (HTTPS)
- **Outbound**: Cred Server → Akeyless Gateway (HTTP/HTTPS)
- **Inbound**: Akeyless Gateway → Cred Server sync endpoints (HTTP)

### Sensitive Data

- Credentials are **not stored** long-term
- Session data is in-memory only
- Request/response bodies captured during recording
- Cleared when session stops

## Development

### Build

```bash
go build -o cred-server ./cmd/main.go
```

### Run Locally

```bash
export AKEYLESS_GATEWAY_URL=https://akeyless.fklab.local
export PORT=8080
./cred-server
```

### Test

```bash
# Unit tests
go test ./...

# Integration test
curl http://localhost:8080/health
```

## Architecture Decisions

### Why Proxy Recording?

**Alternatives considered:**
1. ❌ HAR file upload - Manual, error-prone
2. ❌ OpenAPI spec parsing - Not all apps have specs
3. ✅ **Live proxy recording** - Automatic, captures real traffic

**Benefits:**
- Zero configuration
- Works with any application
- Captures actual request/response
- User performs operations naturally

### Why Score-Based Detection?

Instead of hardcoded endpoint names, we use ML-style scoring:

```go
score := 0.0
if POST && returns_token {
    score += 0.4  // High confidence
}
if path_contains("token") {
    score += 0.3  // Pattern match
}
if no_id_in_path {
    score += 0.2  // Creating new
}
// Total: 0.9 → High confidence CREATE
```

**Benefits:**
- Works with any naming convention
- Handles edge cases
- Confidence levels for validation
- Easy to tune/improve

## Roadmap

- [ ] WebSocket support for real-time apps
- [ ] SAML/OAuth flow detection
- [ ] Multi-factor auth handling
- [ ] GraphQL endpoint detection
- [ ] gRPC support
- [ ] Credential validation testing
- [ ] Historical session replay
- [ ] Export configuration as code

## Contributing

See [TESTING.md](TESTING.md) for testing procedures.

## License

MIT

## Support

- Issues: GitHub Issues
- Docs: [TESTING.md](TESTING.md)
- Logs: `kubectl logs -n cred-server deployment/cred-server -f`

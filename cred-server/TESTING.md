# Cred Server Testing Guide

## Overview

This guide helps you test the credential discovery and producer creation workflow.

## Quick Start

### 1. Deploy the Server

```bash
cd /home/fahmy/code/akeyless/cred-server
./deploy.sh
```

### 2. Access the Web UI

Open your browser:
```
https://cred-server.fklab.local
```

### 3. Discover Credentials

**Step 1: Enter Application URL**
```
URL: https://argo.fklab.local
```

**Step 2: Click "🔍 Discover Application"**
- Button turns **🔴 RED** → Recording started
- You'll get a proxy URL like: `https://cred-server.fklab.local/proxy/abc123/`

**Step 3: Access App Through Proxy**
- Click the proxy link
- Login with admin credentials
- Perform credential operations:
  - **CREATE**: Generate a new API token/key
  - **REVOKE**: Delete/revoke an existing token
  - **ROTATE**: Update/reset credentials (if available)

**Step 4: Stop Recording**
- Click the **🔴 RED** button
- Button turns **🟢 GREEN** → Analysis complete
- Review discovered endpoints

**Step 5: Create Producer**
- Click "➕ Create Producer"
- Review the configuration
- Click "Deploy to Akeyless"

## Troubleshooting Proxy Issues

### Empty/Blank Screen

**Check 1: Is the session active?**
```bash
SESSION_ID="your-session-id"
curl -s "https://cred-server.fklab.local/api/v1/sessions/${SESSION_ID}/debug" -k | jq
```

Look for:
- `"active": true` - Session is recording
- `"total_requests": 5` - Requests are being captured
- `"request_groups"` - Shows HTML/JS/CSS/API requests

**Check 2: Browser console (F12)**
```javascript
// Look for:
// - 404 errors on JS/CSS files
// - CORS errors (should be fixed)
// - CSP violations (should be removed)
```

**Check 3: View raw HTML**
```bash
curl -s "https://cred-server.fklab.local/proxy/${SESSION_ID}/" -k | head -100
```

URLs should be rewritten:
- ❌ `https://argo.fklab.local/api/`
- ✅ `/proxy/abc123/api/`

**Check 4: Response headers**
```bash
curl -I "https://cred-server.fklab.local/proxy/${SESSION_ID}/" -k
```

Should see:
```
X-Proxy-Target: https://argo.fklab.local
X-Proxy-Path: /
```

Should NOT see:
```
Content-Security-Policy: ...  # Blocked by proxy
X-Frame-Options: ...          # Blocked by proxy
```

### Common Issues

**Issue 1: 404 on static files**
```
GET /proxy/abc123/assets/main.js → 404
```

**Fix**: Add trailing slash to proxy URL:
```
https://cred-server.fklab.local/proxy/abc123/
```

**Issue 2: App uses absolute URLs**
```javascript
// App code:
const API_URL = 'https://argo.fklab.local/api'
```

**Fix**: Proxy rewrites these, but check debug endpoint:
```bash
curl -s "https://cred-server.fklab.local/api/v1/sessions/${SESSION_ID}/debug" -k | jq '.request_groups.js'
```

Verify JS files are being loaded and rewritten.

**Issue 3: App has base path**

Some apps use `/argo` or `/app` as base path:
```bash
# Test different paths:
curl -I "https://argo.fklab.local/" -k           # Returns 404?
curl -I "https://argo.fklab.local/argo/" -k      # Returns 200?
```

If app uses base path `/argo`:
```
https://cred-server.fklab.local/proxy/abc123/argo/
```

**Issue 4: WebSocket connections**

WebSockets need special handling:
```bash
# Check logs for WS upgrade requests
kubectl logs -n cred-server deployment/cred-server -f | grep -i websocket
```

## Testing Different Applications

### ArgoCD

```bash
# Target URL
https://argo.fklab.local

# Credential Operations
CREATE:  Settings → Repositories → Connect Repo (uses token)
REVOKE:  Settings → Accounts → Revoke Token
ROTATE:  User Info → Update Password
```

### GitLab

```bash
# Target URL
https://gitlab.fklab.local

# Credential Operations
CREATE:  User Settings → Access Tokens → Create
REVOKE:  User Settings → Access Tokens → Revoke
ROTATE:  User Settings → Password → Change
```

### Jenkins

```bash
# Target URL
https://jenkins.fklab.local

# Credential Operations
CREATE:  User → Configure → Add API Token
REVOKE:  User → Configure → Revoke Token
ROTATE:  User → Configure → Change Password
```

### Generic REST API

```bash
# Target URL
https://api.example.com

# Watch for patterns:
POST   /api/tokens        → CREATE
DELETE /api/tokens/{id}   → REVOKE
PUT    /api/tokens/{id}   → ROTATE
```

## Debugging Endpoint Discovery

### Check Score Thresholds

The system uses confidence scoring (0.0 - 1.0):
- **High confidence**: > 0.7
- **Medium confidence**: 0.5 - 0.7
- **Low confidence**: < 0.5 (not shown)

### View Detailed Logs

```bash
# Watch discovery in real-time
kubectl logs -n cred-server deployment/cred-server -f

# Filter for specific session
kubectl logs -n cred-server deployment/cred-server | grep "session_id=abc123"

# See scoring details
kubectl logs -n cred-server deployment/cred-server | grep "score:"
```

### Manual Analysis

If auto-detection fails, check captured requests:
```bash
SESSION_ID="abc123"

# Get all requests
curl -s "https://cred-server.fklab.local/api/v1/sessions/${SESSION_ID}" -k | \
  jq '.requests[] | {method, path, status, content_type}'

# Filter POST requests (potential CREATE)
curl -s "https://cred-server.fklab.local/api/v1/sessions/${SESSION_ID}" -k | \
  jq '.requests[] | select(.method == "POST") | {path, status}'

# Filter DELETE requests (potential REVOKE)
curl -s "https://cred-server.fklab.local/api/v1/sessions/${SESSION_ID}" -k | \
  jq '.requests[] | select(.method == "DELETE") | {path, status}'
```

## Deployment to Akeyless

### Success Criteria

After clicking "Deploy to Akeyless":
```json
{
  "success": true,
  "secret_path": "/Dynamic/abc123",
  "producer_id": "abc123",
  "message": "Dynamic secret created at /Dynamic/abc123"
}
```

### Testing the Dynamic Secret

```bash
# Get a credential from Akeyless
akeyless dynamic-secret get-value --name /Dynamic/abc123

# Should return:
{
  "id": "...",
  "username": "...",
  "token": "...",
  "expires_at": "..."
}
```

### Common Deployment Issues

**Error: producer_id required**
```
Fixed in latest version - uses fallback logic
```

**Error: 400 from Akeyless**
```bash
# Check Akeyless Gateway logs
kubectl logs -n infra-security deployment/akeyless-gateway -f

# Verify sync URLs are reachable FROM gateway
kubectl exec -n infra-security deployment/akeyless-gateway -- \
  curl -k http://cred-server.cred-server.svc.cluster.local:8080/sync/create
```

**Error: No producer found**
```bash
# List all producers
curl -s "https://cred-server.fklab.local/api/v1/producers" -k | jq
```

## Advanced Testing

### Test with curl

**Start Recording**
```bash
curl -X POST "https://cred-server.fklab.local/api/v1/sessions" \
  -H "Content-Type: application/json" \
  -d '{"target_url": "https://argo.fklab.local"}' \
  -k | jq

# Save session_id
SESSION_ID="..."
```

**Access through proxy**
```bash
# Make requests through proxy
curl "https://cred-server.fklab.local/proxy/${SESSION_ID}/" -k
curl "https://cred-server.fklab.local/proxy/${SESSION_ID}/api/version" -k
```

**Stop and analyze**
```bash
curl -X POST "https://cred-server.fklab.local/api/v1/sessions/${SESSION_ID}/stop" \
  -k | jq
```

### Load Testing

```bash
# Create multiple sessions
for i in {1..10}; do
  curl -X POST "https://cred-server.fklab.local/api/v1/sessions" \
    -H "Content-Type: application/json" \
    -d '{"target_url": "https://argo.fklab.local"}' \
    -k &
done
```

## Success Checklist

- [ ] Server deployed and healthy
- [ ] Web UI accessible
- [ ] Can start recording session
- [ ] Proxy URL loads application (not blank)
- [ ] Can perform credential operations through proxy
- [ ] Stop recording shows discovered endpoints
- [ ] CREATE endpoint detected (required)
- [ ] REVOKE endpoint detected (optional but recommended)
- [ ] Can create producer from discovery
- [ ] Can deploy producer to Akeyless
- [ ] Can get credentials from Akeyless dynamic secret
- [ ] Credentials work with target application

## Getting Help

**View Logs**
```bash
kubectl logs -n cred-server deployment/cred-server -f --tail=100
```

**Check Pod Status**
```bash
kubectl get pods -n cred-server
kubectl describe pod -n cred-server <pod-name>
```

**Debug Network**
```bash
# From inside cluster
kubectl run -it --rm debug --image=nicolaka/netshoot --restart=Never -- bash
curl http://cred-server.cred-server.svc.cluster.local:8080/health
```

**Check Ingress**
```bash
kubectl get ingress -n cred-server
kubectl describe ingress -n cred-server cred-server
```

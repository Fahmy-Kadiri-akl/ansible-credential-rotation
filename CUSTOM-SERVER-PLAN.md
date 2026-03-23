# Akeyless Custom Server Platform - Comprehensive Design Plan

## Executive Summary

This document outlines the architecture for a modern, secure, enterprise-ready custom credential server platform that enables:

1. **Automatic API Discovery** - Browser extension captures authentication flows
2. **Intelligent Server Generation** - Backend automatically generates custom producers
3. **Seamless Akeyless Integration** - Direct connection to existing gateway
4. **Production-Ready Security** - Enterprise-grade security from day one

---

## Part 1: Current State Analysis

### 1.1 Existing Components

| Component | Purpose | Maturity | Issues |
|-----------|---------|----------|--------|
| `custom-producer/` | Reference implementations (Go/Bash) | 55% | Auth disabled, logging secrets, no rate limiting |
| `akeyless-ui-webhook-builder/` | CLI to convert Chrome recordings to scripts | 70% | CLI-only, manual process, limited field support |
| Gateway (titan) | Akeyless Gateway at akeyless.fklab.local | Running | Needs custom producer registration |

### 1.2 Key Gaps Identified

1. **Discovery is Manual** - Users must manually record interactions in Chrome DevTools
2. **No Real-time Analysis** - Can't detect API patterns while browsing
3. **Limited to Web UIs** - No support for REST API credential rotation
4. **No Code Generation** - Only bash scripts, no Go/Python/TypeScript
5. **No Deployment Pipeline** - Manual deployment required
6. **Security Gaps** - Authentication commented out, secrets logged

---

## Part 2: Target Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           USER BROWSER                                       │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    Credential Discovery Extension                     │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐   │    │
│  │  │ Request      │  │ Form Field   │  │ Auth Pattern             │   │    │
│  │  │ Interceptor  │  │ Detector     │  │ Analyzer                 │   │    │
│  │  └──────┬───────┘  └──────┬───────┘  └───────────┬──────────────┘   │    │
│  │         │                 │                       │                  │    │
│  │         └─────────────────┴───────────────────────┘                  │    │
│  │                           │                                          │    │
│  │                    ┌──────▼──────┐                                   │    │
│  │                    │ Discovery   │                                   │    │
│  │                    │ Aggregator  │                                   │    │
│  │                    └──────┬──────┘                                   │    │
│  └───────────────────────────┼──────────────────────────────────────────┘    │
└──────────────────────────────┼──────────────────────────────────────────────┘
                               │ WebSocket + REST
                               ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        CREDENTIAL BUILDER BACKEND                            │
│                                                                              │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────────────────┐     │
│  │ Discovery API  │  │ Analysis       │  │ Code Generator             │     │
│  │ (WebSocket)    │  │ Engine         │  │ (Go/Python/TypeScript)     │     │
│  └───────┬────────┘  └───────┬────────┘  └───────────┬────────────────┘     │
│          │                   │                       │                       │
│          └───────────────────┴───────────────────────┘                       │
│                              │                                               │
│                       ┌──────▼──────┐                                        │
│                       │ Producer    │                                        │
│                       │ Orchestrator│                                        │
│                       └──────┬──────┘                                        │
│                              │                                               │
│  ┌───────────────────────────┼───────────────────────────────────────┐      │
│  │                    ┌──────▼──────┐                                 │      │
│  │                    │ Producer    │                                 │      │
│  │                    │ Registry    │                                 │      │
│  │                    └──────┬──────┘                                 │      │
│  │                           │                                        │      │
│  │  ┌────────────┐  ┌────────┴───────┐  ┌────────────┐               │      │
│  │  │ Database   │  │ Template       │  │ Secret     │               │      │
│  │  │ Producers  │  │ Producers      │  │ Producers  │               │      │
│  │  └────────────┘  └────────────────┘  └────────────┘               │      │
│  │                                                                    │      │
│  │                    PRODUCER RUNTIME                                │      │
│  └────────────────────────────────────────────────────────────────────┘      │
└──────────────────────────────────────────────────────────────────────────────┘
                               │
                               │ mTLS + JWT
                               ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        AKEYLESS GATEWAY (titan)                              │
│                        https://akeyless.fklab.local                          │
│                                                                              │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐      │
│  │ Dynamic Secrets │  │ Rotated Secrets │  │ Producer Management     │      │
│  │ /sync/create    │  │ /sync/rotate    │  │ Gateway API             │      │
│  │ /sync/revoke    │  │                 │  │                         │      │
│  └─────────────────┘  └─────────────────┘  └─────────────────────────┘      │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Part 3: Browser Extension Design

### 3.1 Extension Capabilities

#### Request Interceptor
```typescript
// Intercepts all requests to detect authentication patterns
interface RequestCapture {
  url: string;
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  headers: Record<string, string>;
  body?: string;
  response?: {
    status: number;
    headers: Record<string, string>;
    body?: string;
  };
  timing: {
    started: number;
    completed: number;
  };
}

// Detected authentication patterns
type AuthPattern =
  | 'bearer_token'      // Authorization: Bearer <token>
  | 'basic_auth'        // Authorization: Basic <base64>
  | 'api_key_header'    // X-API-Key, X-Auth-Token, etc.
  | 'api_key_query'     // ?api_key=xxx
  | 'cookie_session'    // Session cookies
  | 'oauth2_flow'       // OAuth2 authorization code/implicit
  | 'saml_sso'          // SAML assertions
  | 'form_login'        // Traditional form POST
  | 'mfa_totp'          // Multi-factor TOTP
  | 'custom';           // Unknown pattern
```

#### Form Field Detector
```typescript
interface FormField {
  id: string;
  name: string;
  type: 'text' | 'password' | 'email' | 'hidden' | 'submit';
  selector: string;
  xpath: string;
  label?: string;
  placeholder?: string;
  autocomplete?: string;
  isPasswordField: boolean;
  isUsernameField: boolean;
  isNewPasswordField: boolean;
  confidence: number;  // 0-1 confidence score
}

// ML-based field classification
interface FieldClassification {
  field: FormField;
  predictedType: 'username' | 'current_password' | 'new_password' |
                 'confirm_password' | 'mfa_code' | 'api_key' | 'unknown';
  confidence: number;
  reasoning: string[];
}
```

#### Auth Pattern Analyzer
```typescript
interface AuthFlowAnalysis {
  appName: string;
  appDomain: string;
  detectedPatterns: AuthPattern[];

  // Login flow
  loginEndpoint?: {
    url: string;
    method: string;
    requestFormat: 'json' | 'form' | 'query';
    fields: FieldClassification[];
  };

  // Password change flow
  passwordChangeEndpoint?: {
    url: string;
    method: string;
    requestFormat: 'json' | 'form';
    fields: FieldClassification[];
    requiresCurrentPassword: boolean;
  };

  // API key management
  apiKeyEndpoint?: {
    url: string;
    method: string;
    rotationSupported: boolean;
    revocationSupported: boolean;
  };

  // Session management
  sessionInfo?: {
    tokenLocation: 'header' | 'cookie' | 'body';
    tokenName: string;
    expirationDetected: boolean;
    refreshSupported: boolean;
  };
}
```

### 3.2 Extension UI

```
┌──────────────────────────────────────────────────────────────┐
│ 🔐 Akeyless Credential Discovery                        [−][×]│
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Current Site: gitlab.fklab.local                            │
│  ────────────────────────────────────────────────────────    │
│                                                              │
│  ✅ DETECTED AUTH PATTERNS                                   │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ • Form Login (confidence: 95%)                         │  │
│  │ • Cookie Session (confidence: 90%)                     │  │
│  │ • API Token Management (confidence: 85%)               │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  📝 DISCOVERED FIELDS                                        │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Username: #user_login (confidence: 98%)                │  │
│  │ Password: #user_password (confidence: 99%)             │  │
│  │ Remember: #user_remember_me (optional)                 │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  📡 CAPTURED REQUESTS: 23                                    │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ POST /users/sign_in (auth endpoint)                    │  │
│  │ GET /api/v4/user (authenticated)                       │  │
│  │ POST /api/v4/personal_access_tokens (token create)     │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─────────────────┐  ┌─────────────────┐                    │
│  │ 📤 Send to      │  │ 💾 Export       │                    │
│  │    Builder      │  │    Recording    │                    │
│  └─────────────────┘  └─────────────────┘                    │
│                                                              │
│  🔗 Connected to: https://credential-builder.fklab.local     │
│  Gateway: https://akeyless.fklab.local ✓                     │
└──────────────────────────────────────────────────────────────┘
```

### 3.3 Extension Manifest

```json
{
  "manifest_version": 3,
  "name": "Akeyless Credential Discovery",
  "version": "1.0.0",
  "description": "Automatically discover credential requirements for custom Akeyless producers",
  "permissions": [
    "webRequest",
    "webRequestBlocking",
    "storage",
    "activeTab",
    "scripting"
  ],
  "host_permissions": ["<all_urls>"],
  "background": {
    "service_worker": "background.js"
  },
  "content_scripts": [{
    "matches": ["<all_urls>"],
    "js": ["content.js"],
    "css": ["content.css"]
  }],
  "action": {
    "default_popup": "popup.html",
    "default_icon": {
      "16": "icons/icon16.png",
      "48": "icons/icon48.png",
      "128": "icons/icon128.png"
    }
  }
}
```

---

## Part 4: Backend Service Design

### 4.1 Service Architecture

```go
// Core service structure
package main

type CredentialBuilderService struct {
    // Configuration
    config           *Config
    akeylessClient   *AkeylessClient

    // Core components
    discoveryServer  *DiscoveryWebSocketServer
    analysisEngine   *AnalysisEngine
    codeGenerator    *CodeGenerator
    producerRuntime  *ProducerRuntime

    // Data stores
    discoveryStore   DiscoveryStore      // Redis for real-time data
    producerStore    ProducerStore       // PostgreSQL for producer configs
    secretStore      SecretStore         // Akeyless for secrets

    // Observability
    metrics          *prometheus.Registry
    tracer           trace.Tracer
    logger           *slog.Logger
}

// API endpoints
type API interface {
    // Discovery endpoints (WebSocket)
    HandleDiscoveryConnection(ws *websocket.Conn) error

    // Analysis endpoints
    AnalyzeAuthFlow(ctx context.Context, req *AnalyzeRequest) (*AuthFlowAnalysis, error)
    SuggestProducerType(ctx context.Context, analysis *AuthFlowAnalysis) (*ProducerSuggestion, error)

    // Code generation endpoints
    GenerateProducer(ctx context.Context, req *GenerateRequest) (*GeneratedProducer, error)
    PreviewProducer(ctx context.Context, req *GenerateRequest) (*ProducerPreview, error)

    // Producer management endpoints
    DeployProducer(ctx context.Context, producer *GeneratedProducer) (*DeploymentResult, error)
    TestProducer(ctx context.Context, producerID string) (*TestResult, error)
    ListProducers(ctx context.Context) ([]*Producer, error)

    // Akeyless integration endpoints
    RegisterWithGateway(ctx context.Context, producer *Producer) error
    CreateDynamicSecret(ctx context.Context, req *DynamicSecretRequest) error
    CreateRotatedSecret(ctx context.Context, req *RotatedSecretRequest) error
}
```

### 4.2 Analysis Engine

```go
package analysis

// AuthFlowAnalyzer uses pattern matching and ML to identify auth flows
type AuthFlowAnalyzer struct {
    patternMatchers  []PatternMatcher
    fieldClassifier  *FieldClassifier
    endpointAnalyzer *EndpointAnalyzer
}

// Pattern matchers for different auth types
type PatternMatcher interface {
    Name() string
    Match(requests []RequestCapture) (MatchResult, error)
    Confidence() float64
}

// Built-in pattern matchers
var DefaultPatternMatchers = []PatternMatcher{
    &BearerTokenMatcher{},      // Detects Bearer token auth
    &BasicAuthMatcher{},        // Detects Basic auth
    &APIKeyMatcher{},           // Detects API key patterns
    &OAuth2Matcher{},           // Detects OAuth2 flows
    &SAMLMatcher{},             // Detects SAML SSO
    &FormLoginMatcher{},        // Detects form-based login
    &SessionCookieMatcher{},    // Detects session cookies
    &JWTMatcher{},              // Detects JWT tokens
}

// Field classifier using heuristics + optional ML
type FieldClassifier struct {
    // Heuristic rules
    usernamePatterns    []string  // "user", "email", "login", "account"
    passwordPatterns    []string  // "pass", "pwd", "secret", "credential"
    newPasswordPatterns []string  // "new", "confirm", "change"

    // Optional ML model
    mlModel            *tflite.Model  // TensorFlow Lite for edge inference
    useML              bool
}

func (c *FieldClassifier) ClassifyField(field FormField) FieldClassification {
    // 1. Check autocomplete attribute (highest confidence)
    if field.Autocomplete == "username" || field.Autocomplete == "email" {
        return FieldClassification{Type: "username", Confidence: 0.99}
    }
    if field.Autocomplete == "current-password" {
        return FieldClassification{Type: "current_password", Confidence: 0.99}
    }
    if field.Autocomplete == "new-password" {
        return FieldClassification{Type: "new_password", Confidence: 0.99}
    }

    // 2. Check field type
    if field.Type == "password" {
        // Use context to determine if new or current
        return c.classifyPasswordField(field)
    }

    // 3. Check name/id/label patterns
    score := c.scoreFieldPatterns(field)

    // 4. Optional ML classification
    if c.useML {
        mlScore := c.mlClassify(field)
        score = c.combineScores(score, mlScore)
    }

    return score
}
```

### 4.3 Code Generator

```go
package generator

// ProducerGenerator creates custom producer code
type ProducerGenerator struct {
    templates     *template.Template
    outputFormats []OutputFormat
}

type OutputFormat string

const (
    FormatGo         OutputFormat = "go"
    FormatPython     OutputFormat = "python"
    FormatTypeScript OutputFormat = "typescript"
    FormatBash       OutputFormat = "bash"
    FormatHelm       OutputFormat = "helm"
    FormatDocker     OutputFormat = "docker"
)

// GeneratedProducer contains all generated artifacts
type GeneratedProducer struct {
    ID          string
    Name        string
    Type        ProducerType
    TargetApp   string

    // Generated code
    SourceCode  map[OutputFormat][]byte  // Code in multiple languages

    // Deployment artifacts
    Dockerfile  []byte
    HelmChart   *HelmChart
    K8sManifests []byte

    // Configuration
    Config      *ProducerConfig
    Secrets     []SecretRef

    // Testing
    TestCases   []TestCase
    MockServer  *MockServerConfig
}

// Template for Go producer
const GoProducerTemplate = `
package producer

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/akeyless/custom-producer/pkg/auth"
)

// {{.Name}}Producer handles credential management for {{.TargetApp}}
type {{.Name}}Producer struct {
    config     *Config
    httpClient *http.Client
    logger     *slog.Logger
}

// Config for the producer
type Config struct {
    // Akeyless configuration
    AllowedAccessID   string   ` + "`" + `env:"AKEYLESS_ACCESS_ID" required:"true"` + "`" + `
    AllowedItemName   string   ` + "`" + `env:"AKEYLESS_ITEM_NAME"` + "`" + `

    // Target application configuration
    {{range .ConfigFields}}
    {{.Name}} {{.Type}} ` + "`" + `env:"{{.EnvVar}}" {{if .Required}}required:"true"{{end}}` + "`" + `
    {{end}}
}

// Create generates new credentials
func (p *{{.Name}}Producer) Create(ctx context.Context, req *CreateRequest) (*CreateResponse, error) {
    // 1. Validate authentication
    if err := auth.Authenticate(ctx, req.Creds, p.config.AllowedAccessID,
        auth.WithAllowedItemName(p.config.AllowedItemName)); err != nil {
        return nil, fmt.Errorf("authentication failed: %w", err)
    }

    // 2. Decode payload
    var payload Payload
    if err := json.Unmarshal(req.Payload, &payload); err != nil {
        return nil, fmt.Errorf("invalid payload: %w", err)
    }

    // 3. Generate credentials
    {{.CreateLogic}}

    // 4. Return response
    return &CreateResponse{
        ID:       credentialID,
        Response: response,
    }, nil
}

// Revoke removes credentials
func (p *{{.Name}}Producer) Revoke(ctx context.Context, req *RevokeRequest) (*RevokeResponse, error) {
    // 1. Validate authentication
    if err := auth.Authenticate(ctx, req.Creds, p.config.AllowedAccessID); err != nil {
        return nil, fmt.Errorf("authentication failed: %w", err)
    }

    // 2. Revoke each credential
    var revoked []string
    var errors []string

    for _, id := range req.IDs {
        {{.RevokeLogic}}
    }

    return &RevokeResponse{
        Revoked: revoked,
        Message: strings.Join(errors, "; "),
    }, nil
}

// Rotate rotates admin credentials
func (p *{{.Name}}Producer) Rotate(ctx context.Context, req *RotateRequest) (*RotateResponse, error) {
    // 1. Validate authentication
    if err := auth.Authenticate(ctx, req.Creds, p.config.AllowedAccessID); err != nil {
        return nil, fmt.Errorf("authentication failed: %w", err)
    }

    // 2. Rotate credentials
    {{.RotateLogic}}

    return &RotateResponse{
        Payload: newPayload,
    }, nil
}
`
```

### 4.4 Producer Registry

```go
package registry

// ProducerRegistry manages producer instances
type ProducerRegistry struct {
    db       *sql.DB
    cache    *redis.Client
    runtime  *ProducerRuntime
    k8s      *kubernetes.Clientset
}

// RegisteredProducer represents a deployed producer
type RegisteredProducer struct {
    ID              string
    Name            string
    Type            ProducerType
    Status          ProducerStatus

    // Deployment info
    DeploymentType  DeploymentType  // kubernetes, docker, lambda
    Endpoint        string
    HealthEndpoint  string

    // Akeyless integration
    AkeylessItemID  string
    GatewayURL      string

    // Metrics
    CreateCount     int64
    RevokeCount     int64
    ErrorCount      int64
    LastActivity    time.Time

    // Configuration
    Config          json.RawMessage
    Secrets         []SecretRef
}

type ProducerStatus string

const (
    StatusPending   ProducerStatus = "pending"
    StatusDeploying ProducerStatus = "deploying"
    StatusRunning   ProducerStatus = "running"
    StatusError     ProducerStatus = "error"
    StatusStopped   ProducerStatus = "stopped"
)

// ProducerRuntime manages producer execution
type ProducerRuntime struct {
    // For Kubernetes deployment
    k8s        *kubernetes.Clientset
    namespace  string

    // For Docker deployment
    docker     *client.Client

    // Built-in producers (run in-process)
    builtIn    map[string]Producer
}

func (r *ProducerRuntime) Deploy(ctx context.Context, producer *GeneratedProducer) error {
    switch r.deploymentType {
    case DeploymentKubernetes:
        return r.deployToKubernetes(ctx, producer)
    case DeploymentDocker:
        return r.deployToDocker(ctx, producer)
    case DeploymentBuiltIn:
        return r.registerBuiltIn(ctx, producer)
    default:
        return fmt.Errorf("unsupported deployment type: %s", r.deploymentType)
    }
}
```

---

## Part 5: Security Architecture

### 5.1 Authentication & Authorization

```go
// Multi-layer authentication
type AuthStack struct {
    // Layer 1: Extension authentication
    extensionAuth  *ExtensionAuthenticator   // API key per extension instance

    // Layer 2: User authentication
    userAuth       *UserAuthenticator        // OIDC/SAML via Akeyless

    // Layer 3: Gateway authentication
    gatewayAuth    *GatewayAuthenticator     // Akeyless JWT validation

    // Layer 4: Service-to-service
    serviceAuth    *mTLSAuthenticator        // mTLS between components
}

// RBAC for producer operations
type Permission string

const (
    PermissionDiscoveryRead   Permission = "discovery:read"
    PermissionDiscoveryWrite  Permission = "discovery:write"
    PermissionProducerCreate  Permission = "producer:create"
    PermissionProducerDeploy  Permission = "producer:deploy"
    PermissionProducerDelete  Permission = "producer:delete"
    PermissionSecretsRead     Permission = "secrets:read"
    PermissionSecretsWrite    Permission = "secrets:write"
    PermissionAdminAll        Permission = "admin:*"
)

type Role struct {
    Name        string
    Permissions []Permission
}

var DefaultRoles = []Role{
    {Name: "viewer", Permissions: []Permission{PermissionDiscoveryRead}},
    {Name: "developer", Permissions: []Permission{
        PermissionDiscoveryRead, PermissionDiscoveryWrite,
        PermissionProducerCreate,
    }},
    {Name: "operator", Permissions: []Permission{
        PermissionDiscoveryRead, PermissionDiscoveryWrite,
        PermissionProducerCreate, PermissionProducerDeploy,
    }},
    {Name: "admin", Permissions: []Permission{PermissionAdminAll}},
}
```

### 5.2 Secrets Management

```go
// All secrets stored in Akeyless, never in application
type SecretManager struct {
    akeyless  *AkeylessClient
    cache     *EncryptedCache  // Optional short-term cache with encryption
}

// Secret references (never store actual values)
type SecretRef struct {
    Name        string  // Human-readable name
    AkeylessPath string // Path in Akeyless
    Version     int     // Secret version
    ExpiresAt   time.Time
}

// Payload encryption for custom producers
type PayloadEncryption struct {
    // Use Akeyless DFC for payload encryption
    akeyless     *AkeylessClient
    encryptionKey string  // Akeyless encryption key path
}

func (p *PayloadEncryption) EncryptPayload(ctx context.Context, payload []byte) ([]byte, error) {
    return p.akeyless.Encrypt(ctx, p.encryptionKey, payload)
}

func (p *PayloadEncryption) DecryptPayload(ctx context.Context, encrypted []byte) ([]byte, error) {
    return p.akeyless.Decrypt(ctx, p.encryptionKey, encrypted)
}
```

### 5.3 Audit Logging

```go
// Comprehensive audit logging without secrets
type AuditLogger struct {
    output    io.Writer
    redactor  *SecretRedactor
}

type AuditEvent struct {
    Timestamp   time.Time         `json:"timestamp"`
    EventType   string            `json:"event_type"`
    Actor       *Actor            `json:"actor"`
    Resource    *Resource         `json:"resource"`
    Action      string            `json:"action"`
    Result      string            `json:"result"`
    Metadata    map[string]string `json:"metadata"`
    // Never include: passwords, tokens, secrets, payloads
}

// Redact sensitive data from logs
type SecretRedactor struct {
    patterns []regexp.Regexp
}

var DefaultRedactionPatterns = []string{
    `(?i)(password|passwd|pwd|secret|token|key|credential)["\s:=]+[^\s"]+`,
    `(?i)bearer\s+[a-zA-Z0-9._-]+`,
    `(?i)basic\s+[a-zA-Z0-9+/=]+`,
    `-----BEGIN.*PRIVATE KEY-----`,
}
```

---

## Part 6: Producer Templates

### 6.1 Built-in Producer Types

| Type | Description | Create | Revoke | Rotate |
|------|-------------|--------|--------|--------|
| `web-form` | Web application form-based auth | ✓ | ✓ | ✓ |
| `rest-api` | REST API with token auth | ✓ | ✓ | ✓ |
| `database` | Database user credentials | ✓ | ✓ | ✓ |
| `oauth2` | OAuth2 client credentials | ✓ | ✓ | ✓ |
| `ssh-key` | SSH key pairs | ✓ | ✓ | ✓ |
| `certificate` | X.509 certificates | ✓ | ✓ | ✓ |
| `cloud-iam` | Cloud IAM credentials | ✓ | ✓ | ✓ |
| `custom` | Fully custom logic | ✓ | ✓ | ✓ |

### 6.2 Web Form Producer Template

```go
// WebFormProducer for form-based web authentication
type WebFormProducer struct {
    config      *WebFormConfig
    browser     *rod.Browser      // Headless browser for automation
    playwright  *playwright.Playwright  // Alternative: Playwright
}

type WebFormConfig struct {
    // Target application
    LoginURL        string
    PasswordChangeURL string

    // Field selectors
    UsernameSelector  string
    PasswordSelector  string
    NewPasswordSelector string
    ConfirmPasswordSelector string
    SubmitSelector    string

    // Success detection
    SuccessIndicator  string  // Selector or text that indicates success
    ErrorIndicator    string  // Selector or text that indicates error

    // Optional MFA
    MFAEnabled        bool
    MFASelector       string
    MFATOTPSecret     string  // Stored in Akeyless
}

func (p *WebFormProducer) Create(ctx context.Context, req *CreateRequest) (*CreateResponse, error) {
    // Use headless browser to automate login
    page := p.browser.MustPage(p.config.LoginURL)
    defer page.Close()

    // Fill form
    page.MustElement(p.config.UsernameSelector).MustInput(req.Username)
    page.MustElement(p.config.PasswordSelector).MustInput(req.Password)
    page.MustElement(p.config.SubmitSelector).MustClick()

    // Handle MFA if enabled
    if p.config.MFAEnabled {
        totp := generateTOTP(p.config.MFATOTPSecret)
        page.MustElement(p.config.MFASelector).MustInput(totp)
    }

    // Wait for success/error
    // Extract session token
    // Return credentials
}
```

### 6.3 REST API Producer Template

```go
// RESTAPIProducer for REST API credential management
type RESTAPIProducer struct {
    config     *RESTAPIConfig
    httpClient *http.Client
}

type RESTAPIConfig struct {
    // Target API
    BaseURL     string

    // Endpoints
    CreateEndpoint struct {
        Path   string
        Method string
        Body   string  // Template with {{.Variables}}
    }
    RevokeEndpoint struct {
        Path   string
        Method string
    }
    RotateEndpoint struct {
        Path   string
        Method string
        Body   string
    }

    // Authentication
    AuthType    string  // bearer, basic, api_key, oauth2
    AuthHeader  string  // Header name for auth
    AuthPrefix  string  // e.g., "Bearer "
}
```

---

## Part 7: Integration with Akeyless Gateway

### 7.1 Gateway Client

```go
package akeyless

type GatewayClient struct {
    baseURL     string
    accessID    string
    credentials CredentialProvider
    httpClient  *http.Client
}

// Register custom producer with gateway
func (c *GatewayClient) RegisterProducer(ctx context.Context, producer *Producer) error {
    // 1. Get auth token
    token, err := c.credentials.GetToken(ctx)
    if err != nil {
        return err
    }

    // 2. Create dynamic secret in Akeyless
    req := &CreateDynamicSecretRequest{
        Name:          producer.Name,
        CreateSyncURL: producer.Endpoint + "/sync/create",
        RevokeSyncURL: producer.Endpoint + "/sync/revoke",
        RotateSyncURL: producer.Endpoint + "/sync/rotate",
        Tags:          producer.Tags,
        UserTTL:       producer.DefaultTTL,
    }

    // 3. Register with gateway
    return c.post(ctx, "/gateway-create-producer-custom", req, token)
}

// Create dynamic secret for the producer
func (c *GatewayClient) CreateDynamicSecret(ctx context.Context, req *DynamicSecretRequest) error {
    token, err := c.credentials.GetToken(ctx)
    if err != nil {
        return err
    }

    return c.post(ctx, "/dynamic-secret-create-custom", req, token)
}
```

### 7.2 Gateway Configuration

```yaml
# Configuration for connecting to existing gateway
gateway:
  url: https://akeyless.fklab.local
  accessID: p-qzj686col15oum

  # Authentication options
  auth:
    # Option 1: Universal Identity (recommended)
    type: uid
    credentialsSecret: akeyless-gateway-credentials

    # Option 2: API Key
    # type: api_key
    # accessKey: ${AKEYLESS_ACCESS_KEY}

    # Option 3: SAML SSO
    # type: saml
    # samlAccessID: p-4hr352lth7m5sm

  # TLS configuration
  tls:
    enabled: true
    caSecret: mkcert-ca
    insecureSkipVerify: false  # Set to true only for development

  # Producer defaults
  producerDefaults:
    defaultTTL: 1h
    maxTTL: 24h
    autoRotate: true
    rotationInterval: 7d
```

---

## Part 8: Deployment Architecture

### 8.1 Kubernetes Deployment

```yaml
# Namespace for credential builder platform
apiVersion: v1
kind: Namespace
metadata:
  name: credential-builder
  labels:
    app.kubernetes.io/name: credential-builder
    app.kubernetes.io/part-of: akeyless
---
# Main backend service
apiVersion: apps/v1
kind: Deployment
metadata:
  name: credential-builder
  namespace: credential-builder
spec:
  replicas: 2
  selector:
    matchLabels:
      app: credential-builder
  template:
    metadata:
      labels:
        app: credential-builder
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
    spec:
      serviceAccountName: credential-builder
      containers:
      - name: backend
        image: ghcr.io/akeyless/credential-builder:latest
        ports:
        - containerPort: 8080  # REST API
        - containerPort: 8081  # WebSocket
        - containerPort: 9090  # Metrics
        env:
        - name: AKEYLESS_GATEWAY_URL
          value: https://akeyless.fklab.local
        - name: AKEYLESS_ACCESS_ID
          valueFrom:
            secretKeyRef:
              name: akeyless-credentials
              key: access-id
        envFrom:
        - secretRef:
            name: credential-builder-config
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 2000m
            memory: 2Gi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
# Producer runtime (for running custom producers)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: producer-runtime
  namespace: credential-builder
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: runtime
        image: ghcr.io/akeyless/producer-runtime:latest
        ports:
        - containerPort: 8000  # Producer endpoints
        securityContext:
          runAsNonRoot: true
          readOnlyRootFilesystem: true
          capabilities:
            drop: ["ALL"]
```

### 8.2 Service Mesh Integration

```yaml
# Istio configuration for mTLS between services
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: credential-builder-mtls
  namespace: credential-builder
spec:
  mtls:
    mode: STRICT
---
# Authorization policy
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: credential-builder-authz
  namespace: credential-builder
spec:
  selector:
    matchLabels:
      app: credential-builder
  rules:
  - from:
    - source:
        principals: ["cluster.local/ns/infra-security/sa/akeyless-gateway-sa"]
    to:
    - operation:
        methods: ["POST"]
        paths: ["/sync/*"]
```

---

## Part 9: Implementation Phases

### Phase 1: Foundation (Weeks 1-2)
- [ ] Set up project structure and CI/CD
- [ ] Implement core backend service with REST API
- [ ] Create authentication middleware (Akeyless JWT validation)
- [ ] Implement basic producer registry (PostgreSQL)
- [ ] Deploy to infra-security namespace on titan

### Phase 2: Analysis Engine (Weeks 3-4)
- [ ] Implement request capture data structures
- [ ] Build pattern matchers for common auth types
- [ ] Create field classifier (heuristic-based)
- [ ] Implement auth flow analyzer
- [ ] Add unit tests for all analyzers

### Phase 3: Code Generator (Weeks 5-6)
- [ ] Create Go producer template
- [ ] Create Python producer template
- [ ] Implement Dockerfile generator
- [ ] Implement Helm chart generator
- [ ] Add integration tests

### Phase 4: Browser Extension (Weeks 7-8)
- [ ] Build extension manifest v3 structure
- [ ] Implement request interceptor
- [ ] Implement form field detector
- [ ] Create WebSocket connection to backend
- [ ] Build popup UI with React

### Phase 5: Producer Runtime (Weeks 9-10)
- [ ] Implement Kubernetes deployment logic
- [ ] Create producer health monitoring
- [ ] Implement automatic scaling
- [ ] Add producer lifecycle management
- [ ] Integration with Akeyless gateway

### Phase 6: Security Hardening (Weeks 11-12)
- [ ] Implement mTLS between components
- [ ] Add comprehensive audit logging
- [ ] Implement rate limiting
- [ ] Security scan and penetration testing
- [ ] Documentation and runbooks

---

## Part 10: Success Criteria

### Functional Requirements
- [ ] Browser extension captures auth flows with 95%+ accuracy
- [ ] System generates working producers for 10+ common applications
- [ ] Producers successfully create/revoke/rotate credentials
- [ ] Integration with Akeyless gateway works seamlessly
- [ ] UI provides clear feedback at every step

### Security Requirements
- [ ] No secrets ever logged or stored in plaintext
- [ ] All inter-service communication uses mTLS
- [ ] Authentication required for all operations
- [ ] Audit trail for all credential operations
- [ ] Rate limiting prevents abuse

### Performance Requirements
- [ ] Extension adds < 50ms latency to page loads
- [ ] Producer generation completes in < 30 seconds
- [ ] Producer deployment completes in < 2 minutes
- [ ] Credential operations complete in < 5 seconds
- [ ] System handles 100+ concurrent producers

---

## Appendix A: API Reference

### Discovery API (WebSocket)

```typescript
// WebSocket messages
interface DiscoveryMessage {
  type: 'request' | 'form_field' | 'auth_detected' | 'session_start' | 'session_end';
  timestamp: number;
  data: RequestCapture | FormField | AuthPattern | SessionInfo;
}

// Session management
interface SessionInfo {
  sessionId: string;
  domain: string;
  startTime: number;
  endTime?: number;
}
```

### Producer API (REST)

```yaml
openapi: 3.0.0
info:
  title: Credential Builder API
  version: 1.0.0

paths:
  /api/v1/analyze:
    post:
      summary: Analyze captured auth flow
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/AnalyzeRequest'
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AuthFlowAnalysis'

  /api/v1/generate:
    post:
      summary: Generate producer code
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/GenerateRequest'
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/GeneratedProducer'

  /api/v1/deploy:
    post:
      summary: Deploy producer to runtime
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/DeployRequest'
      responses:
        '200':
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/DeploymentResult'
```

---

## Appendix B: Configuration Reference

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `AKEYLESS_GATEWAY_URL` | Gateway URL | Yes | - |
| `AKEYLESS_ACCESS_ID` | Access ID for authentication | Yes | - |
| `AKEYLESS_UID_TOKEN` | Universal Identity token | Yes* | - |
| `DATABASE_URL` | PostgreSQL connection string | Yes | - |
| `REDIS_URL` | Redis connection string | Yes | - |
| `LOG_LEVEL` | Logging level | No | `info` |
| `METRICS_PORT` | Prometheus metrics port | No | `9090` |
| `ENABLE_TRACING` | Enable OpenTelemetry tracing | No | `false` |

---

*Document Version: 1.0.0*
*Last Updated: 2026-01-14*
*Author: Fahmy Kadiri*

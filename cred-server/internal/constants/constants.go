// Package constants provides centralized configuration values following
// hierarchical SSOT naming: DOMAIN__CONTEXT__THING__UNIT__QUALIFIER
package constants

import "time"

// =============================================================================
// HTTP Configuration
// =============================================================================

const (
	HTTP__TIMEOUT__CLIENT__SECONDS__DEFAULT    = 30 * time.Second
	HTTP__TIMEOUT__DISCOVERY__SECONDS__DEFAULT = 10 * time.Second
	HTTP__TIMEOUT__REQUEST__SECONDS__DEFAULT   = 60 * time.Second
	HTTP__TIMEOUT__SHUTDOWN__SECONDS__GRACEFUL = 30 * time.Second
)

// =============================================================================
// Size Limits
// =============================================================================

const (
	LIMIT__FORM__SIZE__BYTES__MAX       = 10 << 20 // 10MB
	LIMIT__BODY__TRUNCATE__CHARS__MAX   = 10000
	LIMIT__REQUESTS__RECENT__COUNT__MAX = 10
)

// =============================================================================
// Credential Generation
// =============================================================================

const (
	CREDENTIAL__ID__LENGTH__BYTES__DEFAULT       = 8
	CREDENTIAL__TOKEN__LENGTH__CHARS__DEFAULT    = 40
	CREDENTIAL__PASSWORD__LENGTH__CHARS__DEFAULT = 32
	CREDENTIAL__TTL__DURATION__HOURS__DEFAULT    = 24 * time.Hour
)

// CREDENTIAL__PASSWORD__CHARSET defines allowed characters for generated passwords
const CREDENTIAL__PASSWORD__CHARSET = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"

// =============================================================================
// Confidence Scoring
// =============================================================================

const (
	CONFIDENCE__THRESHOLD__SCORE__MIN  = 0.3
	CONFIDENCE__THRESHOLD__SCORE__LOW  = 0.5
	CONFIDENCE__THRESHOLD__SCORE__HIGH = 0.7
)

// Confidence labels
const (
	CONFIDENCE__LABEL__HIGH   = "high"
	CONFIDENCE__LABEL__MEDIUM = "medium"
	CONFIDENCE__LABEL__LOW    = "low"
)

// =============================================================================
// Authentication Types
// =============================================================================

const (
	AUTH__TYPE__BEARER     = "bearer"
	AUTH__TYPE__BASIC      = "basic"
	AUTH__TYPE__API_KEY    = "api_key"
	AUTH__TYPE__FORM_LOGIN = "form_login"
	AUTH__TYPE__UNKNOWN    = "unknown"
)

// =============================================================================
// Source Labels
// =============================================================================

const (
	SOURCE__LABEL__RECORDING = "recording"
	SOURCE__LABEL__SPEC      = "spec"
)

// =============================================================================
// Scoring Weights (for endpoint detection)
// =============================================================================

const (
	SCORE__WEIGHT__CREDENTIAL_IN_RESPONSE = 0.4
	SCORE__WEIGHT__PATH_MATCH__HIGH       = 0.35
	SCORE__WEIGHT__PATH_MATCH__MEDIUM     = 0.3
	SCORE__WEIGHT__PATH_MATCH__LOW        = 0.25
	SCORE__WEIGHT__NO_ID_IN_PATH          = 0.2
	SCORE__WEIGHT__JSON_RESPONSE          = 0.1
	SCORE__WEIGHT__USER_CREATION          = 0.3
	SCORE__WEIGHT__DELETE_METHOD          = 0.5
	SCORE__WEIGHT__PATH_INDICATOR         = 0.3
	SCORE__WEIGHT__HAS_ID_IN_PATH         = 0.2
	SCORE__WEIGHT__REVOKE_ACTION          = 0.4
	SCORE__WEIGHT__PUT_PATCH_METHOD       = 0.3
	SCORE__WEIGHT__POST_METHOD            = 0.1
	SCORE__WEIGHT__ROTATE_ACTION          = 0.4
	SCORE__WEIGHT__PASSWORD_FIELD         = 0.3
	SCORE__WEIGHT__NEW_CREDENTIAL         = 0.2
	SCORE__PENALTY__LOGIN_PATH            = -0.3
	SCORE__PENALTY__USER_UPDATE           = -0.2
)

// =============================================================================
// Static File Extensions
// =============================================================================

// STATIC__FILE__EXTENSIONS lists file extensions for static assets
var STATIC__FILE__EXTENSIONS = []string{
	".js", ".css", ".png", ".jpg", ".jpeg", ".gif",
	".ico", ".svg", ".woff", ".woff2", ".ttf", ".eot", ".map",
}

// =============================================================================
// Credential Indicators
// =============================================================================

// INDICATOR__CREDENTIAL__KEYWORDS are terms that indicate credential-related fields
var INDICATOR__CREDENTIAL__KEYWORDS = []string{
	"token", "api_key", "apikey", "access_token", "token_alias",
	"secret", "password", "credential", "key", "bearer",
}

// INDICATOR__SENSITIVE__FIELDS are field names that should be sanitized
var INDICATOR__SENSITIVE__FIELDS = []string{
	"password", "passwd", "secret", "token",
	"key", "credential", "api_key", "apikey",
}

// =============================================================================
// Path Indicators for Endpoint Detection
// =============================================================================

// INDICATOR__PATH__CREATE maps URL patterns to create-endpoint confidence scores
var INDICATOR__PATH__CREATE = map[string]float64{
	"token":                 SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"tokens":                SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"api_key":               SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"api-key":               SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"api-keys":              SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"apikey":                SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"apikeys":               SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"personal_access_token": SCORE__WEIGHT__PATH_MATCH__HIGH,
	"personal-access-token": SCORE__WEIGHT__PATH_MATCH__HIGH,
	"access_token":          SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"access-token":          SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"access-tokens":         SCORE__WEIGHT__PATH_MATCH__HIGH, // Common in management APIs (Supabase, etc.)
	"credential":            SCORE__WEIGHT__PATH_MATCH__LOW,
	"credentials":           SCORE__WEIGHT__PATH_MATCH__LOW,
	"service_account":       SCORE__WEIGHT__PATH_MATCH__LOW,
	"service-account":       SCORE__WEIGHT__PATH_MATCH__LOW,
	"application_key":       SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"application-key":       SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"/users":                SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"/user":                 SCORE__WEIGHT__PATH_MATCH__MEDIUM,
	"/profile/":             SCORE__WEIGHT__PATH_MATCH__LOW, // Management APIs often use /profile/ paths
}

// INDICATOR__PATH__REVOKE are URL patterns that suggest credential revocation
var INDICATOR__PATH__REVOKE = []string{
	"token", "tokens", "api_key", "api-key", "api-keys", "apikey", "apikeys", "key", "keys",
	"credential", "credentials", "access", "access-token", "access-tokens",
	"user", "users", "account", "accounts",
}

// INDICATOR__PATH__LOGIN are URL patterns that suggest login endpoints
var INDICATOR__PATH__LOGIN = []string{
	"login", "signin", "sign_in", "authenticate", "auth", "session",
}

// INDICATOR__PATH__API are URL patterns that indicate API endpoints
var INDICATOR__PATH__API = []string{
	"/api/", "/v1/", "/v2/", "/v3/",
	"/auth/", "/oauth/", "/token", "/tokens", "/login", "/session",
	"/platform/", "/profile/", "/management/", "/admin/",
}

// =============================================================================
// Action Indicators
// =============================================================================

// INDICATOR__ACTION__ROTATE are keywords that suggest credential rotation
var INDICATOR__ACTION__ROTATE = []string{
	"rotate", "renew", "refresh", "reset", "change", "update",
}

// INDICATOR__ACTION__REVOKE are keywords that suggest credential revocation
var INDICATOR__ACTION__REVOKE = []string{
	"revoke", "delete", "disable", "deactivate", "remove",
}

// =============================================================================
// Password Field Indicators
// =============================================================================

// INDICATOR__FIELD__PASSWORD are field names related to password rotation
var INDICATOR__FIELD__PASSWORD = []string{
	"new_password", "newpassword", "current_password", "old_password",
}

// =============================================================================
// Response Parsing Keys
// =============================================================================

// RESPONSE__KEY__TOKEN are JSON keys that typically contain tokens
var RESPONSE__KEY__TOKEN = []string{
	"token", "access_token", "api_key", "apiKey", "key", "secret", "password",
	"token_alias", "accessToken", "bearer_token", "refresh_token",
}

// RESPONSE__KEY__ID are JSON keys that typically contain IDs
var RESPONSE__KEY__ID = []string{
	"id", "ID", "Id", "token_id", "key_id",
}

// =============================================================================
// Auth Header Names
// =============================================================================

// AUTH__HEADER__NAMES are header names related to authentication
var AUTH__HEADER__NAMES = []string{
	"authorization", "x-api-key", "x-auth-token", "private-token",
}

// =============================================================================
// API Spec Discovery
// =============================================================================

// SPEC__DISCOVERY__PATHS are common paths where API specs are served
var SPEC__DISCOVERY__PATHS = []string{
	"/swagger/",
	"/swagger/index.html",
	"/swagger/v1/swagger.json",
	"/swagger.json",
	"/swagger.yaml",
	"/openapi.json",
	"/openapi.yaml",
	"/api-docs",
	"/api-docs/",
	"/v2/api-docs",
	"/v3/api-docs",
	"/api/swagger.json",
	"/api/openapi.json",
	"/docs/openapi.json",
	"/.well-known/openapi.json",
}

// =============================================================================
// HTTP Methods
// =============================================================================

// HTTP__METHOD__VALID are valid HTTP methods for API endpoints
var HTTP__METHOD__VALID = []string{
	"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS",
}

// =============================================================================
// Helper Functions
// =============================================================================

// IsHTTPMethod checks if a string is a valid HTTP method
func IsHTTPMethod(method string) bool {
	for _, m := range HTTP__METHOD__VALID {
		if m == method {
			return true
		}
	}
	return false
}

// IsStaticExtension checks if a path ends with a static file extension
func IsStaticExtension(path string) bool {
	for _, ext := range STATIC__FILE__EXTENSIONS {
		if len(path) >= len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}
	return false
}

// ContainsSensitiveField checks if a field name contains sensitive keywords
func ContainsSensitiveField(fieldName string) bool {
	for _, sensitive := range INDICATOR__SENSITIVE__FIELDS {
		if len(fieldName) >= len(sensitive) {
			// Case-insensitive contains check
			for i := 0; i <= len(fieldName)-len(sensitive); i++ {
				match := true
				for j := 0; j < len(sensitive); j++ {
					fc := fieldName[i+j]
					sc := sensitive[j]
					// Simple lowercase comparison for ASCII
					if fc >= 'A' && fc <= 'Z' {
						fc += 32
					}
					if fc != sc {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}
	return false
}

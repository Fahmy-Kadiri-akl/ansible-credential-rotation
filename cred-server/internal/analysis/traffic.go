package analysis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/akeyless-community/cred-server/internal/constants"
	"github.com/rs/zerolog/log"
)

// AnalyzeTraffic analyzes captured traffic and returns analysis results
func AnalyzeTraffic(targetURL string, requests []CapturedRequest) *TrafficAnalysis {
	analysis := &TrafficAnalysis{
		TargetURL:     targetURL,
		TotalRequests: len(requests),
		AllEndpoints:  []DetectedEndpoint{},
		AuthHeaders:   make(map[string]string),
		SpecEndpoints: []DetectedEndpoint{},
	}

	// First, try to discover API spec
	specURL, specEndpoints := DiscoverAPISpec(targetURL)
	if specURL != "" {
		analysis.SpecFound = true
		analysis.SpecURL = specURL
		analysis.SpecEndpoints = specEndpoints
		log.Info().Str("spec_url", specURL).Int("endpoints", len(specEndpoints)).Msg("Discovered API spec")
	}

	var loginEndpoint *DetectedEndpoint
	var createEndpoint *DetectedEndpoint
	var revokeEndpoint *DetectedEndpoint
	var rotateEndpoint *DetectedEndpoint
	authType := ""

	// Check spec endpoints for create/revoke/rotate
	for i := range analysis.SpecEndpoints {
		ep := &analysis.SpecEndpoints[i]
		descLower := strings.ToLower(ep.Description)
		pathLower := strings.ToLower(ep.Path)
		opLower := strings.ToLower(ep.Operation)

		// Look for create endpoints
		if createEndpoint == nil && (ep.Method == "POST" || ep.Method == "PUT") {
			if strings.Contains(descLower, "create") || strings.Contains(opLower, "create") ||
				strings.Contains(descLower, "add user") || strings.Contains(opLower, "adduser") ||
				(strings.Contains(pathLower, "user") && !strings.Contains(pathLower, "{")) ||
				strings.Contains(pathLower, "token") || strings.Contains(pathLower, "apikey") ||
				strings.Contains(pathLower, "credential") {
				createEndpoint = &DetectedEndpoint{
					Path:        ep.Path,
					Method:      ep.Method,
					Description: ep.Description,
					Source:      constants.SOURCE__LABEL__SPEC,
					Operation:   ep.Operation,
					Confidence:  constants.CONFIDENCE__LABEL__HIGH,
				}
			}
		}

		// Look for revoke/delete endpoints
		if revokeEndpoint == nil && ep.Method == "DELETE" {
			if strings.Contains(pathLower, "user") || strings.Contains(pathLower, "token") ||
				strings.Contains(pathLower, "key") || strings.Contains(pathLower, "credential") ||
				strings.Contains(descLower, "delete") || strings.Contains(descLower, "remove") ||
				strings.Contains(descLower, "revoke") {
				revokeEndpoint = &DetectedEndpoint{
					Path:        ep.Path,
					Method:      ep.Method,
					Description: ep.Description,
					Source:      constants.SOURCE__LABEL__SPEC,
					Operation:   ep.Operation,
					Confidence:  constants.CONFIDENCE__LABEL__HIGH,
				}
			}
		}

		// Look for rotate/update password endpoints
		if rotateEndpoint == nil && (ep.Method == "PUT" || ep.Method == "PATCH" || ep.Method == "POST") {
			if strings.Contains(descLower, "password") || strings.Contains(descLower, "rotate") ||
				strings.Contains(opLower, "password") || strings.Contains(opLower, "rotate") ||
				strings.Contains(pathLower, "password") || strings.Contains(pathLower, "rotate") {
				rotateEndpoint = &DetectedEndpoint{
					Path:        ep.Path,
					Method:      ep.Method,
					Description: ep.Description,
					Source:      constants.SOURCE__LABEL__SPEC,
					Operation:   ep.Operation,
					Confidence:  constants.CONFIDENCE__LABEL__MEDIUM,
				}
			}
		}
	}

	// Now analyze recorded traffic with smarter pattern detection
	seenEndpoints := make(map[string]bool)
	credentialScores := make(map[string]float64) // Track confidence scores for each endpoint

	for _, req := range requests {
		// Skip static assets
		if IsStaticAsset(req.Path) {
			continue
		}

		endpointKey := req.Method + " " + req.Path
		if !seenEndpoints[endpointKey] {
			seenEndpoints[endpointKey] = true
			analysis.AllEndpoints = append(analysis.AllEndpoints, DetectedEndpoint{
				Path:        req.Path,
				Method:      req.Method,
				ContentType: req.ContentType,
				Source:      constants.SOURCE__LABEL__RECORDING,
			})
		}

		// Detect auth type from headers
		if auth := req.RequestHeaders["Authorization"]; auth != "" {
			if strings.HasPrefix(auth, "Bearer ") {
				authType = constants.AUTH__TYPE__BEARER
				analysis.AuthHeaders["Authorization"] = "Bearer {{token}}"
			} else if strings.HasPrefix(auth, "Basic ") {
				authType = constants.AUTH__TYPE__BASIC
				analysis.AuthHeaders["Authorization"] = "Basic {{base64(username:password)}}"
			}
		}
		if token := req.RequestHeaders["X-Api-Key"]; token != "" {
			authType = constants.AUTH__TYPE__API_KEY
			analysis.AuthHeaders["X-Api-Key"] = "{{api_key}}"
		}
		if token := req.RequestHeaders["Private-Token"]; token != "" {
			authType = constants.AUTH__TYPE__API_KEY
			analysis.AuthHeaders["Private-Token"] = "{{token}}"
		}

		// Detect login endpoint (POST with password in body) - from recording
		// BUT exclude user creation endpoints (which also have password + other user fields)
		if req.Method == http.MethodPost && loginEndpoint == nil {
			bodyLower := strings.ToLower(req.RequestBody)
			pathLower := strings.ToLower(req.Path)

			// Check if this looks like user creation (password + user profile fields)
			isUserCreation := (strings.Contains(bodyLower, "email") && strings.Contains(bodyLower, "password")) &&
				(strings.Contains(bodyLower, "first_name") || strings.Contains(bodyLower, "last_name") ||
					strings.Contains(bodyLower, "firstname") || strings.Contains(bodyLower, "role"))

			// Only classify as login if it's not user creation
			if !isUserCreation && (strings.Contains(bodyLower, "password") ||
				strings.Contains(bodyLower, "passwd") ||
				strings.Contains(pathLower, "login") ||
				strings.Contains(pathLower, "signin") ||
				strings.Contains(pathLower, "auth") ||
				strings.Contains(pathLower, "session")) {
				if req.StatusCode >= 200 && req.StatusCode < 300 {
					loginEndpoint = &DetectedEndpoint{
						Path:         req.Path,
						Method:       req.Method,
						ContentType:  req.ContentType,
						BodyTemplate: SanitizeBody(req.RequestBody),
						Description:  "Login/Authentication endpoint",
						Confidence:   constants.CONFIDENCE__LABEL__HIGH,
						Source:       constants.SOURCE__LABEL__RECORDING,
					}
				}
			}
		}

		// Detect CREATE equivalent - POST that returns credentials/tokens
		if req.Method == http.MethodPost && req.StatusCode >= 200 && req.StatusCode < 300 {
			score := ScoreAsCreateEndpoint(req)
			if score > constants.CONFIDENCE__THRESHOLD__SCORE__LOW {
				// Only update if this is better than what we have
				if createEndpoint == nil || score > credentialScores["create"] {
					credentialScores["create"] = score
					confidence := constants.CONFIDENCE__LABEL__HIGH
					if score < constants.CONFIDENCE__THRESHOLD__SCORE__HIGH {
						confidence = constants.CONFIDENCE__LABEL__MEDIUM
					}
					createEndpoint = &DetectedEndpoint{
						Path:         req.Path,
						Method:       req.Method,
						ContentType:  req.ContentType,
						BodyTemplate: SanitizeBody(req.RequestBody),
						Description:  fmt.Sprintf("CREATE equivalent (score: %.2f)", score),
						Confidence:   confidence,
						Source:       constants.SOURCE__LABEL__RECORDING,
					}
				}
			}
		}

		// Detect REVOKE equivalent - DELETE or specific POST patterns
		if req.StatusCode >= 200 && req.StatusCode < 300 {
			score := ScoreAsRevokeEndpoint(req)
			if score > constants.CONFIDENCE__THRESHOLD__SCORE__LOW {
				if revokeEndpoint == nil || score > credentialScores["revoke"] {
					credentialScores["revoke"] = score
					confidence := constants.CONFIDENCE__LABEL__HIGH
					if score < constants.CONFIDENCE__THRESHOLD__SCORE__HIGH {
						confidence = constants.CONFIDENCE__LABEL__MEDIUM
					}
					revokeEndpoint = &DetectedEndpoint{
						Path:        req.Path,
						Method:      req.Method,
						Description: fmt.Sprintf("REVOKE equivalent (score: %.2f)", score),
						Confidence:  confidence,
						Source:      constants.SOURCE__LABEL__RECORDING,
					}
				}
			}
		}

		// Detect ROTATE equivalent - PUT/PATCH to credential endpoints
		if (req.Method == http.MethodPut || req.Method == http.MethodPatch || req.Method == http.MethodPost) && req.StatusCode >= 200 && req.StatusCode < 300 {
			score := ScoreAsRotateEndpoint(req)
			if score > constants.CONFIDENCE__THRESHOLD__SCORE__LOW {
				if rotateEndpoint == nil || score > credentialScores["rotate"] {
					credentialScores["rotate"] = score
					confidence := constants.CONFIDENCE__LABEL__HIGH
					if score < constants.CONFIDENCE__THRESHOLD__SCORE__HIGH {
						confidence = constants.CONFIDENCE__LABEL__MEDIUM
					}
					rotateEndpoint = &DetectedEndpoint{
						Path:        req.Path,
						Method:      req.Method,
						Description: fmt.Sprintf("ROTATE equivalent (score: %.2f)", score),
						Confidence:  confidence,
						Source:      constants.SOURCE__LABEL__RECORDING,
					}
				}
			}
		}
	}

	analysis.DetectedAuthType = authType
	analysis.LoginEndpoint = loginEndpoint
	analysis.CreateEndpoint = createEndpoint
	analysis.RevokeEndpoint = revokeEndpoint
	analysis.RotateEndpoint = rotateEndpoint

	// Determine success
	if analysis.SpecFound {
		analysis.Success = true
		if createEndpoint != nil || revokeEndpoint != nil {
			analysis.Message = fmt.Sprintf("Found API spec at %s with credential endpoints. Recording captured %d requests.", specURL, len(requests))
		} else {
			analysis.Message = fmt.Sprintf("Found API spec at %s. Review spec endpoints below to identify credential operations.", specURL)
		}
	} else if loginEndpoint != nil || createEndpoint != nil {
		analysis.Success = true
		analysis.Message = "Successfully detected API patterns from recording. Review the endpoints below."
	} else if len(analysis.AllEndpoints) > 0 {
		analysis.Success = true
		analysis.Message = "Captured endpoints but could not auto-detect auth patterns. No API spec found."
	} else {
		analysis.Message = "No requests captured and no API spec found. Please interact with the application."
	}

	return analysis
}

// IsStaticAsset checks if a path is a static asset
func IsStaticAsset(path string) bool {
	pathLower := strings.ToLower(path)
	return constants.IsStaticExtension(pathLower)
}

// SanitizeBody replaces actual credentials with placeholders
func SanitizeBody(body string) string {
	if body == "" {
		return ""
	}

	// Try to parse as JSON and sanitize
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body), &data); err == nil {
		sanitizeMap(data)
		if sanitized, err := json.Marshal(data); err == nil {
			return string(sanitized)
		}
	}

	return "{{request_body}}"
}

func sanitizeMap(data map[string]interface{}) {
	for key, value := range data {
		keyLower := strings.ToLower(key)
		for _, sensitive := range constants.INDICATOR__SENSITIVE__FIELDS {
			if strings.Contains(keyLower, sensitive) {
				data[key] = "{{" + key + "}}"
				break
			}
		}
		// Recurse into nested objects
		if nested, ok := value.(map[string]interface{}); ok {
			sanitizeMap(nested)
		}
	}
}

// ContainsTokenResponse checks if a response body contains token indicators
func ContainsTokenResponse(body string) bool {
	bodyLower := strings.ToLower(body)
	for _, indicator := range constants.INDICATOR__CREDENTIAL__KEYWORDS {
		if strings.Contains(bodyLower, indicator) {
			return true
		}
	}
	return false
}

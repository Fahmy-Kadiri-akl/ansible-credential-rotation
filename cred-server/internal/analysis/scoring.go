package analysis

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/akeyless-community/cred-server/internal/constants"
)

// ScoreAsCreateEndpoint scores how likely a request is a CREATE credential equivalent
func ScoreAsCreateEndpoint(req CapturedRequest) float64 {
	score := 0.0
	pathLower := strings.ToLower(req.Path)
	respLower := strings.ToLower(req.ResponseBody)

	// Must be POST to be create
	if req.Method != http.MethodPost {
		return 0.0
	}

	// High confidence: Response contains credential indicators
	for _, indicator := range constants.INDICATOR__CREDENTIAL__KEYWORDS {
		if strings.Contains(respLower, `"`+indicator+`"`) || strings.Contains(respLower, `'`+indicator+`'`) {
			score += constants.SCORE__WEIGHT__CREDENTIAL_IN_RESPONSE
			break
		}
	}

	// Path analysis - common credential creation paths
	for path, pathScore := range constants.INDICATOR__PATH__CREATE {
		if strings.Contains(pathLower, path) {
			score += pathScore
			break
		}
	}

	// Path ends without ID (creating new resource, not updating)
	// e.g., /api/tokens (create) vs /api/tokens/123 (update)
	hasIDPattern := regexp.MustCompile(`/\d+$|/[a-f0-9]{8,}$|/\{[^}]+\}$`).MatchString(req.Path)
	if !hasIDPattern && (strings.Contains(pathLower, "token") || strings.Contains(pathLower, "key") || strings.Contains(pathLower, "credential")) {
		score += constants.SCORE__WEIGHT__NO_ID_IN_PATH
	}

	// Response analysis - structured data returned
	if len(req.ResponseBody) > 10 && strings.HasPrefix(strings.TrimSpace(req.ResponseBody), "{") {
		score += constants.SCORE__WEIGHT__JSON_RESPONSE
	}

	// User creation detection: POST with email + password in body (not login)
	bodyLower := strings.ToLower(req.RequestBody)
	if strings.Contains(bodyLower, "email") && strings.Contains(bodyLower, "password") {
		// This could be user creation (email + password + other fields)
		// Differentiate from login by checking for additional user fields
		if strings.Contains(bodyLower, "first_name") || strings.Contains(bodyLower, "last_name") ||
			strings.Contains(bodyLower, "firstname") || strings.Contains(bodyLower, "lastname") ||
			strings.Contains(bodyLower, "role") || strings.Contains(bodyLower, "name") {
			score += constants.SCORE__WEIGHT__USER_CREATION
		}
	}

	// Negative indicators
	if strings.Contains(pathLower, "login") || strings.Contains(pathLower, "signin") || strings.Contains(pathLower, "session") {
		score += constants.SCORE__PENALTY__LOGIN_PATH
	}

	return score
}

// ScoreAsRevokeEndpoint scores how likely a request is a REVOKE credential equivalent
func ScoreAsRevokeEndpoint(req CapturedRequest) float64 {
	score := 0.0
	pathLower := strings.ToLower(req.Path)
	bodyLower := strings.ToLower(req.RequestBody)

	// DELETE is the most obvious revoke
	if req.Method == http.MethodDelete {
		score += constants.SCORE__WEIGHT__DELETE_METHOD

		// Path contains credential indicators
		for _, indicator := range constants.INDICATOR__PATH__REVOKE {
			if strings.Contains(pathLower, indicator) {
				score += constants.SCORE__WEIGHT__PATH_INDICATOR
				break
			}
		}

		// Has ID in path (revoking specific resource)
		// Matches: numbers, hex IDs, UUIDs, emails, usernames
		hasIDPattern := regexp.MustCompile(`/\d+$|/[a-f0-9]{8,}$|/[^/]+@[^/]+$|/[^/]+$`).MatchString(req.Path)
		if hasIDPattern && !strings.HasSuffix(req.Path, "/users") && !strings.HasSuffix(req.Path, "/tokens") {
			score += constants.SCORE__WEIGHT__HAS_ID_IN_PATH
		}

		return score
	}

	// POST/PUT with revoke/delete/disable in path or body
	if req.Method == http.MethodPost || req.Method == http.MethodPut {
		for _, action := range constants.INDICATOR__ACTION__REVOKE {
			if strings.Contains(pathLower, action) || strings.Contains(bodyLower, action) {
				score += constants.SCORE__WEIGHT__REVOKE_ACTION
				break
			}
		}

		// Path contains credential indicators
		if strings.Contains(pathLower, "token") || strings.Contains(pathLower, "key") || strings.Contains(pathLower, "credential") {
			score += constants.SCORE__WEIGHT__NO_ID_IN_PATH
		}
	}

	return score
}

// ScoreAsRotateEndpoint scores how likely a request is a ROTATE credential equivalent
func ScoreAsRotateEndpoint(req CapturedRequest) float64 {
	score := 0.0
	pathLower := strings.ToLower(req.Path)
	bodyLower := strings.ToLower(req.RequestBody)
	respLower := strings.ToLower(req.ResponseBody)

	// PUT/PATCH are common for rotation
	if req.Method == http.MethodPut || req.Method == http.MethodPatch {
		score += constants.SCORE__WEIGHT__PUT_PATCH_METHOD
	} else if req.Method == http.MethodPost {
		score += constants.SCORE__WEIGHT__POST_METHOD
	} else {
		return 0.0
	}

	// Rotation action indicators
	for _, action := range constants.INDICATOR__ACTION__ROTATE {
		if strings.Contains(pathLower, action) || strings.Contains(bodyLower, action) {
			score += constants.SCORE__WEIGHT__ROTATE_ACTION
			break
		}
	}

	// Password/credential indicators in path
	credentialPaths := []string{"password", "credential", "secret", "key"}
	for _, path := range credentialPaths {
		if strings.Contains(pathLower, path) {
			score += constants.SCORE__WEIGHT__NO_ID_IN_PATH
			break
		}
	}

	// Body contains password-related fields
	for _, field := range constants.INDICATOR__FIELD__PASSWORD {
		if strings.Contains(bodyLower, field) {
			score += constants.SCORE__WEIGHT__PASSWORD_FIELD
			break
		}
	}

	// Response returns new credential
	if strings.Contains(respLower, "token") || strings.Contains(respLower, "password") || strings.Contains(respLower, "secret") {
		score += constants.SCORE__WEIGHT__NEW_CREDENTIAL
	}

	// Negative indicators - user creation/update is not rotation UNLESS it has password fields
	// Only penalize if path has /user but body doesn't have password rotation indicators
	hasPasswordRotation := strings.Contains(bodyLower, "old_password") || strings.Contains(bodyLower, "new_password") ||
		strings.Contains(bodyLower, "current_password") || strings.Contains(bodyLower, "newpassword")
	if strings.Contains(pathLower, "/user") && !strings.Contains(pathLower, "password") && !hasPasswordRotation {
		score += constants.SCORE__PENALTY__USER_UPDATE
	}

	return score
}

package producer

// RotateRequest is sent by Akeyless to rotate the stored payload credentials.
type RotateRequest struct {
	Payload string `json:"payload"`
}

// RotateResponse returns the new payload to be stored in Akeyless.
type RotateResponse struct {
	Payload string `json:"payload"`
}

// CreateRequest is sent by Akeyless to create temporary credentials.
type CreateRequest struct {
	Payload    string     `json:"payload"`
	ClientInfo ClientInfo `json:"client_info"`
}

// ClientInfo wraps the requesting user's identity.
type ClientInfo struct {
	AccessID  string              `json:"access_id"`
	SubClaims map[string][]string `json:"sub_claims"`
}

// CreateResponse returns the temporary credential.
type CreateResponse struct {
	ID       string `json:"id"`
	Response string `json:"response"`
}

// RevokeRequest is sent by Akeyless when a dynamic secret TTL expires.
type RevokeRequest struct {
	Payload string   `json:"payload"`
	IDs     []string `json:"ids"`
}

// RevokeResponse confirms which credentials were revoked.
type RevokeResponse struct {
	Revoked []string `json:"revoked"`
	Message string   `json:"message,omitempty"`
}

// ---- Payload types stored in Akeyless rotated secrets ----

// PasswordPayload is the encrypted payload for Ansible user password rotation.
type PasswordPayload struct {
	Type           string `json:"type"`            // "password"
	AnsibleURL     string `json:"ansible_url"`     // AAP/AWX controller URL
	AdminUser      string `json:"admin_user"`      // Admin username for API auth
	AdminPassword  string `json:"admin_password"`  // Admin password for API auth
	TargetUsername string `json:"target_username"` // User whose password to rotate
	TargetUserID   int    `json:"target_user_id"`  // User ID (0 = auto-lookup)
	Password       string `json:"password"`        // Current password
	SkipTLSVerify  bool   `json:"skip_tls_verify"`
}

// APIKeyPayload is the encrypted payload for Ansible API token rotation.
type APIKeyPayload struct {
	Type          string `json:"type"`            // "api_key"
	AnsibleURL    string `json:"ansible_url"`     // AAP/AWX controller URL
	AdminUser     string `json:"admin_user"`      // Admin username for API auth
	AdminPassword string `json:"admin_password"`  // Admin password for API auth
	TargetUserID  int    `json:"target_user_id"`  // User whose token to rotate
	TokenID       int    `json:"token_id"`        // Current token ID (for revocation)
	Token         string `json:"token"`           // Current token value
	TokenScope    string `json:"token_scope"`     // "write" or "read"
	Description   string `json:"description"`     // Token description
	SkipTLSVerify bool   `json:"skip_tls_verify"`
}

package handler

import "encoding/json"

// AkeylessRequest represents the request from Akeyless gateway
type AkeylessRequest struct {
	Creds    *AkeylessCreds  `json:"creds"`
	Payload  json.RawMessage `json:"payload"`
	IDs      []string        `json:"ids,omitempty"`       // For revoke
	ItemName string          `json:"item_name,omitempty"` // Dynamic secret name
}

// AkeylessCreds represents the credentials sent by Akeyless
type AkeylessCreds struct {
	AccessID  string `json:"access_id"`
	AccessKey string `json:"access_key,omitempty"`
	Token     string `json:"token,omitempty"`
}

// CreateResponse is returned for /sync/create
type CreateResponse struct {
	ID       string `json:"id"`
	Response string `json:"response"`
}

// RevokeResponse is returned for /sync/revoke
// Akeyless expects "revoked" to be an array of the IDs that were revoked
type RevokeResponse struct {
	Revoked []string `json:"revoked"`
	Message string   `json:"message,omitempty"`
}

// RotateResponse is returned for /sync/rotate
type RotateResponse struct {
	Payload string `json:"payload"`
}

// Package producer implements the Akeyless custom producer protocol for
// Ansible AAP/AWX credential rotation. It supports two rotation types:
//   - Password rotation: generates a new password, updates the Ansible user
//   - API key rotation: creates a new personal access token, revokes the old one
package producer

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/akeyless-community/ansible-credential-rotation/internal/ansible"
	"github.com/rs/zerolog/log"
)

const (
	passwordCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	defaultPwLen    = 24
)

// Producer handles Ansible credential lifecycle operations.
type Producer struct{}

// New creates a new Ansible credential producer.
func New() *Producer {
	return &Producer{}
}

// Rotate generates new credentials on the Ansible target and returns
// the updated payload for Akeyless to store.
func (p *Producer) Rotate(ctx context.Context, req *RotateRequest) (*RotateResponse, error) {
	// Determine payload type
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(req.Payload), &peek); err != nil {
		return nil, fmt.Errorf("parse payload type: %w", err)
	}

	switch peek.Type {
	case "password":
		return p.rotatePassword(ctx, req.Payload)
	case "api_key":
		return p.rotateAPIKey(ctx, req.Payload)
	default:
		return nil, fmt.Errorf("unknown payload type: %q", peek.Type)
	}
}

// Create returns the current credentials from the payload. For rotated secrets
// this is called when a consumer requests the secret value.
func (p *Producer) Create(ctx context.Context, req *CreateRequest) (*CreateResponse, error) {
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(req.Payload), &peek); err != nil {
		return nil, fmt.Errorf("parse payload: %w", err)
	}

	switch peek.Type {
	case "password":
		var payload PasswordPayload
		if err := json.Unmarshal([]byte(req.Payload), &payload); err != nil {
			return nil, fmt.Errorf("parse password payload: %w", err)
		}
		resp, _ := json.Marshal(map[string]string{
			"username": payload.TargetUsername,
			"password": payload.Password,
		})
		return &CreateResponse{
			ID:       payload.TargetUsername,
			Response: string(resp),
		}, nil

	case "api_key":
		var payload APIKeyPayload
		if err := json.Unmarshal([]byte(req.Payload), &payload); err != nil {
			return nil, fmt.Errorf("parse api_key payload: %w", err)
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"token":    payload.Token,
			"token_id": payload.TokenID,
		})
		return &CreateResponse{
			ID:       fmt.Sprintf("%d", payload.TokenID),
			Response: string(resp),
		}, nil

	default:
		return nil, fmt.Errorf("unknown payload type: %q", peek.Type)
	}
}

// Revoke is a no-op for rotated secrets (credentials are replaced, not revoked).
func (p *Producer) Revoke(_ context.Context, req *RevokeRequest) (*RevokeResponse, error) {
	return &RevokeResponse{
		Revoked: req.IDs,
		Message: "rotated secret revoke acknowledged",
	}, nil
}

// rotatePassword generates a new password, updates the Ansible user, and
// returns the updated payload.
func (p *Producer) rotatePassword(ctx context.Context, rawPayload string) (*RotateResponse, error) {
	var payload PasswordPayload
	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		return nil, fmt.Errorf("parse password payload: %w", err)
	}

	client := ansible.NewClient(payload.AnsibleURL, payload.SkipTLSVerify)
	adminAuth := ansible.AuthMethod{
		Username: payload.AdminUser,
		Password: payload.AdminPassword,
	}

	// Auto-lookup user ID if not provided
	userID := payload.TargetUserID
	if userID == 0 {
		var err error
		userID, err = client.LookupUserByUsername(ctx, adminAuth, payload.TargetUsername)
		if err != nil {
			return nil, fmt.Errorf("lookup user %q: %w", payload.TargetUsername, err)
		}
		log.Info().Int("user_id", userID).Str("username", payload.TargetUsername).Msg("resolved user ID")
	}

	// Generate new password
	newPassword, err := generatePassword(defaultPwLen)
	if err != nil {
		return nil, fmt.Errorf("generate password: %w", err)
	}

	// Update password on Ansible
	if err := client.UpdateUserPassword(ctx, adminAuth, userID, newPassword); err != nil {
		return nil, fmt.Errorf("update ansible user password: %w", err)
	}

	log.Info().Str("username", payload.TargetUsername).Msg("password rotated successfully on Ansible")

	// Update payload with new password and resolved user ID
	payload.Password = newPassword
	payload.TargetUserID = userID
	newPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal updated payload: %w", err)
	}

	return &RotateResponse{Payload: string(newPayload)}, nil
}

// rotateAPIKey creates a new personal access token, then revokes the old one.
func (p *Producer) rotateAPIKey(ctx context.Context, rawPayload string) (*RotateResponse, error) {
	var payload APIKeyPayload
	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		return nil, fmt.Errorf("parse api_key payload: %w", err)
	}

	client := ansible.NewClient(payload.AnsibleURL, payload.SkipTLSVerify)
	adminAuth := ansible.AuthMethod{
		Username: payload.AdminUser,
		Password: payload.AdminPassword,
	}

	scope := payload.TokenScope
	if scope == "" {
		scope = "write"
	}
	description := payload.Description
	if description == "" {
		description = "akeyless-rotated-token"
	}

	// Create new token first (create-before-revoke pattern)
	newToken, err := client.CreatePersonalToken(ctx, adminAuth, payload.TargetUserID, description, scope)
	if err != nil {
		return nil, fmt.Errorf("create new token: %w", err)
	}

	log.Info().Int("new_token_id", newToken.ID).Int("user_id", payload.TargetUserID).Msg("new API token created")

	// Revoke old token (best-effort - the old token may already be expired)
	if payload.TokenID > 0 {
		if err := client.RevokeToken(ctx, adminAuth, payload.TokenID); err != nil {
			log.Warn().Err(err).Int("old_token_id", payload.TokenID).Msg("failed to revoke old token (may already be expired)")
		} else {
			log.Info().Int("old_token_id", payload.TokenID).Msg("old API token revoked")
		}
	}

	// Update payload with new token
	payload.TokenID = newToken.ID
	payload.Token = newToken.Token
	newPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal updated payload: %w", err)
	}

	return &RotateResponse{Payload: string(newPayload)}, nil
}

// generatePassword creates a cryptographically random password.
func generatePassword(length int) (string, error) {
	pw := make([]byte, length)
	for i := range pw {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordCharset))))
		if err != nil {
			return "", err
		}
		pw[i] = passwordCharset[idx.Int64()]
	}
	return string(pw), nil
}

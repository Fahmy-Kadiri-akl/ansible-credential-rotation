package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/akeyless-community/cred-server/internal/constants"
	"github.com/akeyless-community/cred-server/internal/producer"
	"github.com/rs/zerolog/log"
)

// SyncHandler handles Akeyless custom producer sync endpoints
type SyncHandler struct {
	registry         *producer.Registry
	allowedAccessIDs []string
}

// NewSyncHandler creates a new sync handler
func NewSyncHandler(registry *producer.Registry, allowedAccessIDs string) *SyncHandler {
	ids := []string{}
	if allowedAccessIDs != "" {
		ids = strings.Split(allowedAccessIDs, ",")
		for i, id := range ids {
			ids[i] = strings.TrimSpace(id)
		}
	}

	return &SyncHandler{
		registry:         registry,
		allowedAccessIDs: ids,
	}
}

// Create handles /sync/create - generates new credentials
func (h *SyncHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Parse request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		respondBadRequest(w, "Failed to read request")
		return
	}

	var req AkeylessRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to parse request")
		respondBadRequest(w, "Invalid request format")
		return
	}

	// Handle nil creds (can happen during dry run)
	accessID := ""
	if req.Creds != nil {
		accessID = req.Creds.AccessID
	}

	log.Info().
		Str("access_id", accessID).
		Str("item_name", req.ItemName).
		RawJSON("payload", req.Payload).
		Msg("Received create request")

	// Authenticate (skip if no creds provided - dry run mode)
	if req.Creds != nil {
		if err := h.authenticate(req.Creds); err != nil {
			log.Warn().Err(err).Str("access_id", accessID).Msg("Authentication failed")
			respondUnauthorized(w, "Authentication failed")
			return
		}
	}

	// Parse payload to determine which producer to use
	// Akeyless may send payload as a JSON string (double-encoded) or as a JSON object
	var payload map[string]interface{}
	if len(req.Payload) > 0 {
		// First try to unmarshal directly as an object
		if err := json.Unmarshal(req.Payload, &payload); err != nil {
			// If that fails, try to unmarshal as a string first (double-encoded JSON)
			var payloadStr string
			if strErr := json.Unmarshal(req.Payload, &payloadStr); strErr == nil && payloadStr != "" {
				// Now unmarshal the string content as JSON object
				if innerErr := json.Unmarshal([]byte(payloadStr), &payload); innerErr != nil {
					log.Warn().Err(innerErr).Str("payload_str", payloadStr).Msg("Failed to parse inner payload JSON")
					payload = make(map[string]interface{})
				}
			} else {
				log.Warn().Err(err).Str("payload", string(req.Payload)).Msg("Failed to parse payload, using empty")
				payload = make(map[string]interface{})
			}
		}
	} else {
		payload = make(map[string]interface{})
	}

	// Get producer ID from payload
	producerID, _ := payload["producer_id"].(string)
	if producerID == "" {
		// For dry run or if no producer_id, try to get from item name or use first producer
		log.Warn().Str("item_name", req.ItemName).Msg("No producer_id in payload, attempting fallback")

		// Try to extract from item name (e.g., /Dynamic/producer-123 -> producer-123)
		if req.ItemName != "" {
			parts := strings.Split(req.ItemName, "/")
			if len(parts) > 0 {
				producerID = parts[len(parts)-1]
			}
		}

		// If still no producer ID, try to use the first available producer (for demo/testing)
		if producerID == "" {
			producers := h.registry.ListProducers()
			if len(producers) > 0 {
				producerID = producers[0].ID
				log.Info().Str("producer_id", producerID).Msg("Using first available producer")
			} else {
				respondBadRequest(w, "producer_id is required in payload and no producers available")
				return
			}
		}
	}

	// Get producer
	prod, err := h.registry.GetProducer(producerID)
	if err != nil {
		log.Error().Err(err).Str("producer_id", producerID).Msg("Producer not found")
		respondNotFound(w, fmt.Sprintf("Producer not found: %s", producerID))
		return
	}

	// Generate credential based on producer type
	var cred *producer.Credential
	var response map[string]interface{}

	switch prod.Type {
	case producer.TypeRESTAPI:
		cred, response, err = h.createRESTAPICredential(prod, payload)
	default:
		// For demo, generate a random credential
		cred, response, err = h.createDemoCredential(prod, payload)
	}

	if err != nil {
		log.Error().Err(err).Str("producer_id", producerID).Msg("Failed to create credential")
		respondInternalError(w, fmt.Sprintf("Failed to create credential: %v", err))
		return
	}

	// Store credential for tracking
	h.registry.StoreCredential(cred)

	// Build response
	responseJSON, _ := json.Marshal(response)
	resp := &CreateResponse{
		ID:       cred.ID,
		Response: string(responseJSON),
	}

	log.Info().
		Str("credential_id", cred.ID).
		Str("producer_id", producerID).
		Msg("Credential created successfully")

	respondOK(w, resp)
}

// Revoke handles /sync/revoke - removes credentials
func (h *SyncHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	// Parse request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		respondBadRequest(w, "Failed to read request")
		return
	}

	var req AkeylessRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to parse request")
		respondBadRequest(w, "Invalid request format")
		return
	}

	// Handle nil creds (can happen during dry run)
	accessID := ""
	if req.Creds != nil {
		accessID = req.Creds.AccessID
	}

	log.Info().
		Str("access_id", accessID).
		Strs("ids", req.IDs).
		Msg("Received revoke request")

	// Authenticate (skip if no creds provided - dry run mode)
	if req.Creds != nil {
		if err := h.authenticate(req.Creds); err != nil {
			log.Warn().Err(err).Str("access_id", accessID).Msg("Authentication failed")
			respondUnauthorized(w, "Authentication failed")
			return
		}
	}

	// Handle dry run (no IDs provided)
	if len(req.IDs) == 0 {
		resp := &RevokeResponse{
			Revoked: []string{},
			Message: "dry run - no credentials to revoke",
		}
		respondOK(w, resp)
		return
	}

	// Revoke each credential
	var errors []string
	var revokedIDs []string

	for _, id := range req.IDs {
		cred, exists := h.registry.GetCredential(id)
		if !exists {
			log.Warn().Str("id", id).Msg("Credential not found for revocation")
			errors = append(errors, fmt.Sprintf("credential %s not found", id))
			// Still mark as revoked for Akeyless
			revokedIDs = append(revokedIDs, id)
			continue
		}

		// Get producer to know how to revoke
		prod, err := h.registry.GetProducer(cred.ProducerID)
		if err != nil {
			log.Warn().Err(err).Str("id", id).Msg("Producer not found for revocation")
			errors = append(errors, fmt.Sprintf("producer for %s not found", id))
			// Still mark as revoked for Akeyless
			revokedIDs = append(revokedIDs, id)
			h.registry.DeleteCredential(id)
			continue
		}

		// Check if producer has revoke endpoint configured
		if prod.Config == nil || prod.Config.RevokeEndpoint == nil {
			log.Info().Str("id", id).Str("producer", prod.Name).Msg("Producer has no revoke endpoint, removing from registry only")
			h.registry.DeleteCredential(id)
			revokedIDs = append(revokedIDs, id)
			continue
		}

		// Revoke based on producer type
		if err := h.revokeCredential(prod, cred); err != nil {
			log.Error().Err(err).Str("id", id).Msg("Failed to revoke credential")
			errors = append(errors, fmt.Sprintf("failed to revoke %s: %v", id, err))
			// Still remove from registry even if revoke fails
			h.registry.DeleteCredential(id)
			revokedIDs = append(revokedIDs, id)
			continue
		}

		// Remove from registry
		h.registry.DeleteCredential(id)
		revokedIDs = append(revokedIDs, id)
		log.Info().Str("id", id).Msg("Credential revoked")
	}

	message := fmt.Sprintf("Revoked %d credentials", len(revokedIDs))
	if len(errors) > 0 {
		message += fmt.Sprintf(". Warnings: %s", strings.Join(errors, "; "))
	}

	resp := &RevokeResponse{
		Revoked: revokedIDs,
		Message: message,
	}

	respondOK(w, resp)
}

// Rotate handles /sync/rotate - rotates admin credentials
func (h *SyncHandler) Rotate(w http.ResponseWriter, r *http.Request) {
	// Parse request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		respondBadRequest(w, "Failed to read request")
		return
	}

	var req AkeylessRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to parse request")
		respondBadRequest(w, "Invalid request format")
		return
	}

	// Handle nil creds (can happen during dry run)
	accessID := ""
	if req.Creds != nil {
		accessID = req.Creds.AccessID
	}

	log.Info().
		Str("access_id", accessID).
		RawJSON("payload", req.Payload).
		Msg("Received rotate request")

	// Authenticate (skip if no creds provided - dry run mode)
	if req.Creds != nil {
		if err := h.authenticate(req.Creds); err != nil {
			log.Warn().Err(err).Str("access_id", accessID).Msg("Authentication failed")
			respondUnauthorized(w, "Authentication failed")
			return
		}
	} else {
		// If no creds and trying to rotate, just return success for dry run
		resp := RotateResponse{
			Payload: "{}",
		}
		respondOK(w, resp)
		return
	}

	// Parse current payload
	var currentPayload map[string]interface{}
	if err := json.Unmarshal(req.Payload, &currentPayload); err != nil {
		log.Error().Err(err).Msg("Failed to parse rotate payload")
		respondBadRequest(w, "Invalid payload")
		return
	}

	// Generate new password
	newPassword := generateSecurePassword(constants.CREDENTIAL__PASSWORD__LENGTH__CHARS__DEFAULT)
	currentPayload["password"] = newPassword
	currentPayload["rotated_at"] = time.Now().Format(time.RFC3339)

	// In a real implementation, this would update the target system
	log.Info().Msg("Admin credentials rotated (demo mode)")

	newPayloadJSON, _ := json.Marshal(currentPayload)
	resp := &RotateResponse{
		Payload: string(newPayloadJSON),
	}

	respondOK(w, resp)
}

// authenticate validates the Akeyless credentials
func (h *SyncHandler) authenticate(creds *AkeylessCreds) error {
	if creds == nil || creds.AccessID == "" {
		return fmt.Errorf("missing credentials")
	}

	// Skip authentication if no allowed IDs configured (demo mode)
	if len(h.allowedAccessIDs) == 0 {
		log.Debug().Msg("No allowed access IDs configured, skipping authentication")
		return nil
	}

	// Check if access ID is allowed
	for _, allowed := range h.allowedAccessIDs {
		if creds.AccessID == allowed {
			return nil
		}
	}

	return fmt.Errorf("access ID not allowed: %s", creds.AccessID)
}

// createRESTAPICredential creates credential via REST API
func (h *SyncHandler) createRESTAPICredential(prod *producer.Producer, payload map[string]interface{}) (*producer.Credential, map[string]interface{}, error) {
	cfg := prod.Config
	if cfg == nil || cfg.CreateEndpoint == nil {
		return h.createDemoCredential(prod, payload)
	}

	// Build URL
	urlTmpl, err := template.New("url").Parse(prod.TargetURL + cfg.CreateEndpoint.Path)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid URL template: %w", err)
	}

	var urlBuf bytes.Buffer
	if err := urlTmpl.Execute(&urlBuf, payload); err != nil {
		return nil, nil, fmt.Errorf("failed to execute URL template: %w", err)
	}

	// Build body
	var bodyBuf bytes.Buffer
	if cfg.CreateEndpoint.BodyTemplate != "" {
		bodyTmpl, err := template.New("body").Parse(cfg.CreateEndpoint.BodyTemplate)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid body template: %w", err)
		}
		if err := bodyTmpl.Execute(&bodyBuf, payload); err != nil {
			return nil, nil, fmt.Errorf("failed to execute body template: %w", err)
		}
	}

	// Create HTTP request
	req, err := http.NewRequest(cfg.CreateEndpoint.Method, urlBuf.String(), &bodyBuf)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add authentication
	if cfg.AdminPassword != "" {
		switch cfg.AuthType {
		case constants.AUTH__TYPE__BEARER:
			req.Header.Set(cfg.AuthHeader, cfg.AdminPassword)
		case constants.AUTH__TYPE__BASIC:
			auth := base64.StdEncoding.EncodeToString([]byte(cfg.AdminUsername + ":" + cfg.AdminPassword))
			req.Header.Set("Authorization", "Basic "+auth)
		case constants.AUTH__TYPE__API_KEY:
			req.Header.Set(cfg.AuthHeader, cfg.AdminPassword)
		}
	}

	// Add custom headers
	for k, v := range cfg.CreateEndpoint.Headers {
		req.Header.Set(k, v)
	}

	log.Debug().
		Str("url", urlBuf.String()).
		Str("method", cfg.CreateEndpoint.Method).
		Msg("Making API request")

	// Execute request (skip for demo without real target)
	// For demo, return generated credentials
	return h.createDemoCredential(prod, payload)
}

// createDemoCredential generates demo credentials
func (h *SyncHandler) createDemoCredential(prod *producer.Producer, payload map[string]interface{}) (*producer.Credential, map[string]interface{}, error) {
	credID := generateID()
	token := generateSecureToken(constants.CREDENTIAL__TOKEN__LENGTH__CHARS__DEFAULT)

	username, _ := payload["username"].(string)
	if username == "" {
		username = "demo-user"
	}

	expiresAt := time.Now().Add(constants.CREDENTIAL__TTL__DURATION__HOURS__DEFAULT)

	cred := &producer.Credential{
		ID:         credID,
		ProducerID: prod.ID,
		ExternalID: credID,
		Username:   username,
		Token:      token,
		CreatedAt:  time.Now(),
		ExpiresAt:  &expiresAt,
	}

	response := map[string]interface{}{
		"id":         credID,
		"username":   username,
		"token":      token,
		"expires_at": expiresAt.Format(time.RFC3339),
		"producer":   prod.Name,
		"type":       string(prod.Type),
	}

	return cred, response, nil
}

// revokeCredential revokes a credential from the target system
func (h *SyncHandler) revokeCredential(prod *producer.Producer, cred *producer.Credential) error {
	// For demo, just log the revocation
	log.Info().
		Str("credential_id", cred.ID).
		Str("producer", prod.Name).
		Msg("Revoking credential (demo mode)")

	return nil
}

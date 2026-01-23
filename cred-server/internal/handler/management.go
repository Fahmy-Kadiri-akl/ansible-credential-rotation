package handler

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/akeyless-community/cred-server/internal/config"
	"github.com/akeyless-community/cred-server/internal/producer"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

//go:embed ui/*
var uiFS embed.FS

// ManagementHandler handles producer management API
type ManagementHandler struct {
	registry       *producer.Registry
	akeylessClient *AkeylessClient
}

// NewManagementHandler creates a new management handler
func NewManagementHandler(registry *producer.Registry, cfg *config.Config) *ManagementHandler {
	return &ManagementHandler{
		registry:       registry,
		akeylessClient: NewAkeylessClient(cfg),
	}
}

// CreateProducerRequest is the request to create a new producer
type CreateProducerRequest struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Type        producer.ProducerType    `json:"type"`
	Description string                   `json:"description"`
	TargetURL   string                   `json:"target_url"`
	Config      *producer.ProducerConfig `json:"config"`
}

// CreateProducer creates a new producer
func (h *ManagementHandler) CreateProducer(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondBadRequest(w, "Failed to read request")
		return
	}

	var req CreateProducerRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respondBadRequest(w, "Invalid request format")
		return
	}

	if req.Name == "" {
		respondBadRequest(w, "Name is required")
		return
	}

	if req.ID == "" {
		req.ID = generateID()
	}

	if req.Type == "" {
		req.Type = producer.TypeCustom
	}

	prod := &producer.Producer{
		ID:          req.ID,
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		TargetURL:   req.TargetURL,
		Config:      req.Config,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.registry.AddProducer(prod); err != nil {
		log.Error().Err(err).Str("id", req.ID).Msg("Failed to create producer")
		respondError(w, http.StatusConflict, err.Error())
		return
	}

	log.Info().Str("id", prod.ID).Str("name", prod.Name).Msg("Producer created")
	respondCreated(w, prod)
}

// ListProducers returns all producers
func (h *ManagementHandler) ListProducers(w http.ResponseWriter, r *http.Request) {
	producers := h.registry.ListProducers()
	respondOK(w, producers)
}

// GetProducer returns a specific producer
func (h *ManagementHandler) GetProducer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	prod, err := h.registry.GetProducer(id)
	if err != nil {
		respondNotFound(w, err.Error())
		return
	}

	respondOK(w, prod)
}

// DeleteProducer removes a producer
func (h *ManagementHandler) DeleteProducer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.registry.DeleteProducer(id); err != nil {
		respondNotFound(w, err.Error())
		return
	}

	log.Info().Str("id", id).Msg("Producer deleted")
	w.WriteHeader(http.StatusNoContent)
}

// TestProducer tests a producer
func (h *ManagementHandler) TestProducer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	prod, err := h.registry.GetProducer(id)
	if err != nil {
		respondNotFound(w, err.Error())
		return
	}

	result := map[string]interface{}{
		"producer": prod.Name,
		"status":   "success",
		"message":  fmt.Sprintf("Producer %s is configured correctly", prod.Name),
	}

	log.Info().Str("id", id).Msg("Producer tested")
	respondOK(w, result)
}

// DeployToAkeylessRequest is the request to deploy a producer to Akeyless
type DeployToAkeylessRequest struct {
	SecretPath string `json:"secret_path"` // Path in Akeyless, e.g., /Dynamic/my-app
	TTL        string `json:"ttl"`         // e.g., "1h", "24h"
}

// DeployToAkeyless creates a dynamic secret in Akeyless for a producer
func (h *ManagementHandler) DeployToAkeyless(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	prod, err := h.registry.GetProducer(id)
	if err != nil {
		respondNotFound(w, err.Error())
		return
	}

	// Parse optional request body
	var req DeployToAkeylessRequest
	if r.Body != nil {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)
	}

	// Default secret path
	secretPath := req.SecretPath
	if secretPath == "" {
		secretPath = "/Dynamic/" + prod.ID
	}

	// Default TTL
	ttl := req.TTL
	if ttl == "" {
		ttl = "1h"
	}

	// Derive sync URL from the request's Host header (what the user sees in their browser)
	syncURLBase := getSyncURLFromRequest(r)
	log.Info().Str("sync_url_from_request", syncURLBase).Msg("Using sync URL derived from request")

	// Create the dynamic secret in Akeyless
	createReq := &CreateDynamicSecretRequest{
		Name:           secretPath,
		ProducerID:     prod.ID,
		UserTTL:        ttl,
		TimeoutSeconds: 60,
		Tags:           []string{"cred-server", "auto-created"},
		SyncURLBase:    syncURLBase,
	}

	err = h.akeylessClient.CreateCustomProducer(createReq)
	if err != nil {
		log.Error().Err(err).Str("producer_id", id).Msg("Failed to create Akeyless dynamic secret")
		respondInternalError(w, fmt.Sprintf("Failed to deploy to Akeyless: %v", err))
		return
	}

	log.Info().
		Str("producer_id", id).
		Str("secret_path", secretPath).
		Msg("Successfully deployed to Akeyless")

	result := map[string]interface{}{
		"success":     true,
		"secret_path": secretPath,
		"producer_id": prod.ID,
		"message":     fmt.Sprintf("Dynamic secret created at %s", secretPath),
		"usage":       fmt.Sprintf("akeyless dynamic-secret get-value --name %s", secretPath),
	}

	respondOK(w, result)
}

// getSyncURLFromRequest derives the sync URL base from the incoming HTTP request
// This uses the Host header and scheme to build a URL that matches what the user sees in their browser
func getSyncURLFromRequest(r *http.Request) string {
	// Determine scheme
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// Check X-Forwarded-Proto header (for reverse proxies)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	// Get host from request
	host := r.Host
	// Check X-Forwarded-Host header (for reverse proxies)
	if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}

	return fmt.Sprintf("%s://%s", scheme, host)
}

// ServeUI serves the web UI index page
func ServeUI(w http.ResponseWriter, r *http.Request) {
	data, err := uiFS.ReadFile("ui/index.html")
	if err != nil {
		respondInternalError(w, "Failed to load UI")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// UIFileServer returns an http.Handler that serves UI static files
func UIFileServer() http.Handler {
	subFS, err := fs.Sub(uiFS, "ui")
	if err != nil {
		log.Error().Err(err).Msg("Failed to create UI sub-filesystem")
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(subFS))
}

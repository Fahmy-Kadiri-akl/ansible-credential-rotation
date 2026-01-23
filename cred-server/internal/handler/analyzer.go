package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/akeyless-community/cred-server/internal/constants"
	"github.com/akeyless-community/cred-server/internal/har"
	"github.com/akeyless-community/cred-server/internal/producer"
	"github.com/rs/zerolog/log"
)

// AnalyzerHandler handles HAR file analysis and producer generation
type AnalyzerHandler struct {
	registry *producer.Registry
}

// NewAnalyzerHandler creates a new analyzer handler
func NewAnalyzerHandler(registry *producer.Registry) *AnalyzerHandler {
	return &AnalyzerHandler{registry: registry}
}

// AnalysisResult contains the detected auth patterns
type AnalysisResult struct {
	AppName         string             `json:"app_name"`
	BaseURL         string             `json:"base_url"`
	AuthType        string             `json:"auth_type"`
	DetectedFlows   []DetectedFlow     `json:"detected_flows"`
	SuggestedConfig *producer.Producer `json:"suggested_config"`
	RawEntries      int                `json:"raw_entries"`
	DetectedHosts   []HostAnalysis     `json:"detected_hosts,omitempty"` // All hosts found in HAR
	APIHost         string             `json:"api_host,omitempty"`       // Primary API host (for credential operations)
}

// HostAnalysis summarizes API activity for a single host
type HostAnalysis struct {
	Host            string  `json:"host"`
	BaseURL         string  `json:"base_url"`
	RequestCount    int     `json:"request_count"`
	APIRequestPct   float64 `json:"api_request_pct"` // Percentage that are API calls (not static)
	HasCredFlows    bool    `json:"has_cred_flows"`  // Has credential create/revoke flows
	CredFlowCount   int     `json:"cred_flow_count"` // Number of credential flows
	IsLikelyAPIHost bool    `json:"is_likely_api_host"`
}

type DetectedFlow struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"` // login, create_token, revoke_token, api_call
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Host       string            `json:"host"` // The host this flow came from
	Path       string            `json:"path"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body,omitempty"`
	BodyFormat string            `json:"body_format"` // json, form, none
	Response   *FlowResponse     `json:"response,omitempty"`
	Confidence float64           `json:"confidence"`
}

type FlowResponse struct {
	Status    int               `json:"status"`
	Headers   map[string]string `json:"headers"`
	TokenPath string            `json:"token_path,omitempty"`
	IDPath    string            `json:"id_path,omitempty"`
}

// AnalyzeHAR analyzes an uploaded HAR file
func (h *AnalyzerHandler) AnalyzeHAR(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	if err := r.ParseMultipartForm(constants.LIMIT__FORM__SIZE__BYTES__MAX); err != nil {
		respondBadRequest(w, "Failed to parse form")
		return
	}

	file, header, err := r.FormFile("har")
	if err != nil {
		respondBadRequest(w, "No HAR file provided")
		return
	}
	defer file.Close()

	log.Info().Str("filename", header.Filename).Int64("size", header.Size).Msg("Analyzing HAR file")

	// Read HAR content
	data, err := io.ReadAll(file)
	if err != nil {
		respondBadRequest(w, "Failed to read HAR file")
		return
	}

	// Parse HAR
	var harFile har.File
	if err := json.Unmarshal(data, &harFile); err != nil {
		respondBadRequest(w, "Invalid HAR format: "+err.Error())
		return
	}

	// Analyze
	result := h.analyzeEntries(harFile.Log.Entries)

	log.Info().
		Str("app", result.AppName).
		Str("auth_type", result.AuthType).
		Int("flows", len(result.DetectedFlows)).
		Msg("HAR analysis complete")

	respondOK(w, result)
}

// CreateFromAnalysis creates a producer from analysis result
func (h *AnalyzerHandler) CreateFromAnalysis(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondBadRequest(w, "Failed to read request")
		return
	}

	var result AnalysisResult
	if err := json.Unmarshal(body, &result); err != nil {
		respondBadRequest(w, "Invalid request format")
		return
	}

	if result.SuggestedConfig == nil {
		respondBadRequest(w, "No suggested config in analysis")
		return
	}

	prod := result.SuggestedConfig
	prod.ID = generateID()
	prod.CreatedAt = time.Now()
	prod.UpdatedAt = time.Now()

	if err := h.registry.AddProducer(prod); err != nil {
		respondError(w, http.StatusConflict, err.Error())
		return
	}

	log.Info().Str("id", prod.ID).Str("name", prod.Name).Msg("Producer created from analysis")

	respondCreated(w, prod)
}

func (h *AnalyzerHandler) analyzeEntries(entries []har.Entry) *AnalysisResult {
	result := &AnalysisResult{
		DetectedFlows: []DetectedFlow{},
		RawEntries:    len(entries),
	}

	if len(entries) == 0 {
		return result
	}

	// Group entries by host and track statistics
	hostEntries := make(map[string][]har.Entry)
	hostStats := make(map[string]*HostAnalysis)

	for _, entry := range entries {
		u, err := url.Parse(entry.Request.URL)
		if err != nil {
			continue
		}
		host := u.Host
		hostEntries[host] = append(hostEntries[host], entry)

		if hostStats[host] == nil {
			hostStats[host] = &HostAnalysis{
				Host:    host,
				BaseURL: fmt.Sprintf("%s://%s", u.Scheme, u.Host),
			}
		}
		hostStats[host].RequestCount++
	}

	// Analyze each entry and track which host has credential flows
	flowsByHost := make(map[string][]DetectedFlow)

	for _, entry := range entries {
		flow := h.analyzeEntry(entry)
		if flow != nil {
			result.DetectedFlows = append(result.DetectedFlows, *flow)

			// Track which host this flow belongs to
			u, _ := url.Parse(entry.Request.URL)
			if u != nil {
				flowsByHost[u.Host] = append(flowsByHost[u.Host], *flow)

				// Update host stats for credential flows
				if flow.Type == "create_token" || flow.Type == "revoke_token" {
					hostStats[u.Host].HasCredFlows = true
					hostStats[u.Host].CredFlowCount++
				}
			}
		}
	}

	// Calculate API request percentage for each host
	for host, stats := range hostStats {
		apiCount := 0
		for _, entry := range hostEntries[host] {
			if h.isAPIRequest(entry.Request) {
				apiCount++
			}
		}
		if stats.RequestCount > 0 {
			stats.APIRequestPct = float64(apiCount) / float64(stats.RequestCount) * 100
		}
		// A host is likely an API host if it has high API percentage or credential flows
		stats.IsLikelyAPIHost = stats.APIRequestPct > 50 || stats.HasCredFlows
	}

	// Determine the primary API host (prefer host with credential flows, then highest API%)
	var apiHost *HostAnalysis
	for _, stats := range hostStats {
		if apiHost == nil {
			apiHost = stats
			continue
		}
		// Prefer host with credential flows
		if stats.HasCredFlows && !apiHost.HasCredFlows {
			apiHost = stats
		} else if stats.HasCredFlows == apiHost.HasCredFlows {
			// If both have (or don't have) cred flows, prefer higher API%
			if stats.APIRequestPct > apiHost.APIRequestPct {
				apiHost = stats
			}
		}
	}

	// Set the base URL and app name from the API host
	if apiHost != nil {
		result.BaseURL = apiHost.BaseURL
		result.AppName = apiHost.Host
		result.APIHost = apiHost.Host
	} else if len(entries) > 0 {
		// Fallback to first entry
		if u, err := url.Parse(entries[0].Request.URL); err == nil {
			result.BaseURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
			result.AppName = u.Host
		}
	}

	// Build detected hosts list
	for _, stats := range hostStats {
		result.DetectedHosts = append(result.DetectedHosts, *stats)
	}

	// Determine auth type based on detected flows
	result.AuthType = h.detectAuthType(result.DetectedFlows)

	// Generate suggested producer config
	result.SuggestedConfig = h.generateProducerConfig(result)

	return result
}

func (h *AnalyzerHandler) analyzeEntry(entry har.Entry) *DetectedFlow {
	req := entry.Request
	resp := entry.Response

	// Skip non-API requests
	if !h.isAPIRequest(req) {
		return nil
	}

	// Parse URL
	u, err := url.Parse(req.URL)
	if err != nil {
		return nil
	}

	flow := &DetectedFlow{
		Method:     req.Method,
		URL:        req.URL,
		Host:       u.Host,
		Path:       u.Path,
		Headers:    make(map[string]string),
		BodyFormat: "none",
		Confidence: 0.5,
	}

	// Extract relevant headers
	for _, header := range req.Headers {
		lowerName := strings.ToLower(header.Name)
		if lowerName == "authorization" ||
			lowerName == "x-api-key" ||
			lowerName == "x-auth-token" ||
			lowerName == "private-token" ||
			strings.Contains(lowerName, "auth") ||
			strings.Contains(lowerName, "token") ||
			strings.Contains(lowerName, "api-key") {
			flow.Headers[header.Name] = header.Value
		}
	}

	// Analyze body
	if req.PostData != nil && req.PostData.Text != "" {
		flow.Body = req.PostData.Text
		if strings.Contains(req.PostData.MimeType, "json") {
			flow.BodyFormat = "json"
		} else if strings.Contains(req.PostData.MimeType, "form") {
			flow.BodyFormat = "form"
		}
	}

	// Analyze response
	if resp.Status >= 200 && resp.Status < 300 {
		flow.Response = &FlowResponse{
			Status:  resp.Status,
			Headers: make(map[string]string),
		}

		// Look for tokens in response
		if resp.Content.Text != "" {
			flow.Response.TokenPath = h.findTokenInJSON(resp.Content.Text)
			flow.Response.IDPath = h.findIDInJSON(resp.Content.Text)
		}
	}

	// Classify the flow
	flow.Type, flow.Name, flow.Confidence = h.classifyFlow(flow, u.Path)

	// Only return if it looks like an auth-related flow
	if flow.Confidence < constants.CONFIDENCE__THRESHOLD__SCORE__MIN {
		return nil
	}

	return flow
}

func (h *AnalyzerHandler) isAPIRequest(req har.Request) bool {
	// Check content type
	for _, header := range req.Headers {
		if strings.ToLower(header.Name) == "content-type" {
			if strings.Contains(header.Value, "json") ||
				strings.Contains(header.Value, "form") {
				return true
			}
		}
		if strings.ToLower(header.Name) == "accept" {
			if strings.Contains(header.Value, "json") {
				return true
			}
		}
	}

	// Check URL patterns using centralized constants
	lowerURL := strings.ToLower(req.URL)
	for _, pattern := range constants.INDICATOR__PATH__API {
		if strings.Contains(lowerURL, pattern) {
			return true
		}
	}

	// POST/PUT/DELETE to non-static resources
	if req.Method == http.MethodPost || req.Method == http.MethodPut || req.Method == http.MethodDelete || req.Method == http.MethodPatch {
		// Exclude static resources using centralized constants
		if constants.IsStaticExtension(lowerURL) {
			return false
		}
		return true
	}

	return false
}

func (h *AnalyzerHandler) classifyFlow(flow *DetectedFlow, path string) (string, string, float64) {
	lowerPath := strings.ToLower(path)
	lowerBody := strings.ToLower(flow.Body)

	// Login patterns using centralized constants
	for _, p := range constants.INDICATOR__PATH__LOGIN {
		if strings.Contains(lowerPath, p) {
			if flow.Method == http.MethodPost {
				return "login", "User Login", 0.9
			}
		}
	}

	// Password in body suggests login
	if strings.Contains(lowerBody, "password") || strings.Contains(lowerBody, "passwd") {
		return "login", "User Login", 0.85
	}

	// Token creation patterns - include both underscore and hyphen variants
	tokenPatterns := []string{
		"token", "tokens", "api_key", "api-key", "api-keys", "apikey", "apikeys",
		"access_token", "access-token", "access-tokens",
		"personal_access", "personal-access",
		"credential", "credentials",
	}
	for _, p := range tokenPatterns {
		if strings.Contains(lowerPath, p) {
			if flow.Method == http.MethodPost {
				return "create_token", "Create API Token", 0.9
			}
			if flow.Method == http.MethodDelete {
				return "revoke_token", "Revoke API Token", 0.9
			}
		}
	}

	// User/account patterns with POST might be creating credentials
	if flow.Method == http.MethodPost {
		userPatterns := []string{"user", "account", "credential", "key"}
		for _, p := range userPatterns {
			if strings.Contains(lowerPath, p) {
				return "create_token", "Create Credential", 0.7
			}
		}
	}

	// DELETE requests are likely revocation
	if flow.Method == http.MethodDelete {
		return "revoke_token", "Revoke/Delete", 0.6
	}

	// Has auth headers - likely an authenticated API call
	if len(flow.Headers) > 0 {
		return "api_call", "Authenticated API Call", 0.5
	}

	return "unknown", "Unknown", 0.3
}

func (h *AnalyzerHandler) findTokenInJSON(jsonStr string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return ""
	}

	// Use centralized token key constants
	for _, key := range constants.RESPONSE__KEY__TOKEN {
		if _, ok := data[key]; ok {
			return key
		}
	}
	return ""
}

func (h *AnalyzerHandler) findIDInJSON(jsonStr string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return ""
	}

	// Use centralized ID key constants
	for _, key := range constants.RESPONSE__KEY__ID {
		if _, ok := data[key]; ok {
			return key
		}
	}
	return ""
}

func (h *AnalyzerHandler) detectAuthType(flows []DetectedFlow) string {
	for _, flow := range flows {
		// Check headers
		for name := range flow.Headers {
			lowerName := strings.ToLower(name)
			if lowerName == "authorization" {
				if strings.HasPrefix(strings.ToLower(flow.Headers[name]), "bearer") {
					return constants.AUTH__TYPE__BEARER
				}
				if strings.HasPrefix(strings.ToLower(flow.Headers[name]), "basic") {
					return constants.AUTH__TYPE__BASIC
				}
			}
			if strings.Contains(lowerName, "api-key") || strings.Contains(lowerName, "apikey") {
				return constants.AUTH__TYPE__API_KEY
			}
			if strings.Contains(lowerName, "token") {
				return constants.AUTH__TYPE__API_KEY
			}
		}
	}

	// Check for form login
	for _, flow := range flows {
		if flow.Type == "login" && flow.BodyFormat == "form" {
			return constants.AUTH__TYPE__FORM_LOGIN
		}
	}

	return constants.AUTH__TYPE__UNKNOWN
}

func (h *AnalyzerHandler) generateProducerConfig(result *AnalysisResult) *producer.Producer {
	// Find relevant flows
	var createFlow, revokeFlow *DetectedFlow
	var authHeaders map[string]string

	for i, flow := range result.DetectedFlows {
		switch flow.Type {
		case "login", "create_token":
			if createFlow == nil || flow.Confidence > createFlow.Confidence {
				createFlow = &result.DetectedFlows[i]
			}
		case "revoke_token":
			if revokeFlow == nil || flow.Confidence > revokeFlow.Confidence {
				revokeFlow = &result.DetectedFlows[i]
			}
		case "api_call":
			if len(flow.Headers) > 0 && authHeaders == nil {
				authHeaders = flow.Headers
			}
		}
	}

	// Build producer config
	config := &producer.ProducerConfig{
		AuthType: result.AuthType,
	}

	// Determine auth header
	for name, value := range authHeaders {
		config.AuthHeader = name
		// Mask the actual value
		if len(value) > 10 {
			config.AdminPassword = "{{ADMIN_TOKEN}}"
		}
		break
	}

	// Create endpoint
	if createFlow != nil {
		u, _ := url.Parse(createFlow.URL)
		config.CreateEndpoint = &producer.EndpointConfig{
			Path:   u.Path,
			Method: createFlow.Method,
		}
		if createFlow.Body != "" {
			// Template-ize the body
			config.CreateEndpoint.BodyTemplate = h.templatizeBody(createFlow.Body)
		}
		if createFlow.Response != nil {
			config.TokenPath = createFlow.Response.TokenPath
			config.CredentialIDPath = createFlow.Response.IDPath
		}
	}

	// Revoke endpoint
	if revokeFlow != nil {
		u, _ := url.Parse(revokeFlow.URL)
		config.RevokeEndpoint = &producer.EndpointConfig{
			Path:   h.templatizePath(u.Path),
			Method: revokeFlow.Method,
		}
	}

	return &producer.Producer{
		Name:        fmt.Sprintf("%s Credentials", result.AppName),
		Type:        producer.TypeRESTAPI,
		Description: fmt.Sprintf("Auto-generated producer for %s", result.AppName),
		TargetURL:   result.BaseURL,
		Config:      config,
	}
}

func (h *AnalyzerHandler) templatizeBody(body string) string {
	// Replace common field values with template variables
	replacements := map[string]string{
		// Mask any hardcoded passwords/secrets
	}

	result := body
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	return result
}

func (h *AnalyzerHandler) templatizePath(path string) string {
	// Replace IDs in path with template variables
	// e.g., /api/tokens/123 -> /api/tokens/{{.token_id}}

	// Match numeric IDs
	idPattern := regexp.MustCompile(`/(\d+)(/|$)`)
	result := idPattern.ReplaceAllString(path, "/{{.id}}$2")

	return result
}

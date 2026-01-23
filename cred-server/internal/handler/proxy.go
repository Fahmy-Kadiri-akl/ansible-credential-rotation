package handler

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/akeyless-community/cred-server/internal/analysis"
	"github.com/akeyless-community/cred-server/internal/constants"
	"github.com/akeyless-community/cred-server/internal/rewrite"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// ProxyHandler handles reverse proxy recording for API discovery
type ProxyHandler struct {
	client   *http.Client
	sessions map[string]*RecordingSession
	mu       sync.RWMutex
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler() *ProxyHandler {
	return &ProxyHandler{
		client: &http.Client{
			Timeout: constants.HTTP__TIMEOUT__CLIENT__SECONDS__DEFAULT,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		},
		sessions: make(map[string]*RecordingSession),
	}
}

// StartSession starts a new recording session
func (h *ProxyHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetURL       string   `json:"target_url"`
		AdditionalHosts []string `json:"additional_hosts,omitempty"` // Additional hosts to proxy (e.g., api.supabase.com)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondBadRequest(w, "Invalid request")
		return
	}

	// Validate and normalize URL
	if req.TargetURL == "" {
		respondBadRequest(w, "target_url is required")
		return
	}
	if !strings.HasPrefix(req.TargetURL, "http") {
		req.TargetURL = "https://" + req.TargetURL
	}

	// Parse to validate
	u, err := url.Parse(req.TargetURL)
	if err != nil {
		respondBadRequest(w, "Invalid URL")
		return
	}

	sessionID := generateID()
	primaryTarget := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	// Build target hosts map
	targetHosts := make(map[string]string)
	targetHosts[u.Host] = primaryTarget

	// Add additional hosts
	for _, host := range req.AdditionalHosts {
		if host == "" {
			continue
		}
		// Normalize: if no scheme, assume https
		if !strings.HasPrefix(host, "http") {
			host = "https://" + host
		}
		hostURL, err := url.Parse(host)
		if err != nil {
			log.Warn().Str("host", host).Err(err).Msg("Invalid additional host, skipping")
			continue
		}
		targetHosts[hostURL.Host] = fmt.Sprintf("%s://%s", hostURL.Scheme, hostURL.Host)
	}

	session := &RecordingSession{
		ID:          sessionID,
		TargetURL:   primaryTarget,
		TargetHosts: targetHosts,
		StartedAt:   time.Now(),
		Active:      true,
		Requests:    []analysis.CapturedRequest{},
	}

	h.mu.Lock()
	h.sessions[sessionID] = session
	h.mu.Unlock()

	log.Info().
		Str("session_id", sessionID).
		Str("primary_target", session.TargetURL).
		Int("total_hosts", len(targetHosts)).
		Msg("Started recording session")

	respondOK(w, map[string]interface{}{
		"session_id":   sessionID,
		"target_url":   session.TargetURL,
		"target_hosts": targetHosts,
		"proxy_url":    fmt.Sprintf("/proxy/%s", sessionID),
		"message":      "Recording session started. Use proxy_url to access the target application. Additional hosts will be proxied via /_h/{hostname}/path.",
	})
}

// StopSession stops a recording session and returns analysis
func (h *ProxyHandler) StopSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	h.mu.RLock()
	session, ok := h.sessions[sessionID]
	h.mu.RUnlock()

	if !ok {
		respondNotFound(w, "Session not found")
		return
	}

	requestCount := session.Stop()

	log.Info().Str("session_id", sessionID).Int("requests", requestCount).Msg("Stopped recording session")

	// Analyze the captured traffic using the analysis package
	result := analysis.AnalyzeTraffic(session.TargetURL, session.GetRequests())

	respondOK(w, result)
}

// DebugSession returns detailed session info for debugging
func (h *ProxyHandler) DebugSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	h.mu.RLock()
	session, ok := h.sessions[sessionID]
	h.mu.RUnlock()

	if !ok {
		respondNotFound(w, "Session not found")
		return
	}

	requests := session.GetRequests()

	// Group requests by type
	htmlRequests := []string{}
	jsRequests := []string{}
	cssRequests := []string{}
	apiRequests := []string{}
	otherRequests := []string{}

	for _, req := range requests {
		path := req.Method + " " + req.Path
		contentType := strings.ToLower(req.ContentType)

		if strings.Contains(contentType, "html") {
			htmlRequests = append(htmlRequests, path)
		} else if strings.Contains(contentType, "javascript") {
			jsRequests = append(jsRequests, path)
		} else if strings.Contains(contentType, "css") {
			cssRequests = append(cssRequests, path)
		} else if strings.Contains(contentType, "json") {
			apiRequests = append(apiRequests, path)
		} else {
			otherRequests = append(otherRequests, path)
		}
	}

	// Get recent requests
	recent := []map[string]interface{}{}
	start := len(requests) - constants.LIMIT__REQUESTS__RECENT__COUNT__MAX
	if start < 0 {
		start = 0
	}
	for i := start; i < len(requests); i++ {
		req := requests[i]
		recent = append(recent, map[string]interface{}{
			"method":       req.Method,
			"path":         req.Path,
			"status":       req.StatusCode,
			"content_type": req.ContentType,
			"timestamp":    req.Timestamp,
		})
	}

	debug := map[string]interface{}{
		"session_id":     sessionID,
		"target_url":     session.TargetURL,
		"active":         session.IsActive(),
		"total_requests": len(requests),
		"request_groups": map[string]interface{}{
			"html":  htmlRequests,
			"js":    jsRequests,
			"css":   cssRequests,
			"api":   apiRequests,
			"other": otherRequests,
		},
		"recent_requests": recent,
	}

	respondOK(w, debug)
}

// GetSession returns session details
func (h *ProxyHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	h.mu.RLock()
	session, ok := h.sessions[sessionID]
	h.mu.RUnlock()

	if !ok {
		respondNotFound(w, "Session not found")
		return
	}

	respondOK(w, session)
}

// ProxyRequest handles proxied requests
func (h *ProxyHandler) ProxyRequest(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	h.mu.RLock()
	session, ok := h.sessions[sessionID]
	h.mu.RUnlock()

	if !ok {
		respondNotFound(w, "Session not found")
		return
	}

	// Get the path after /proxy/{sessionID}
	path := chi.URLParam(r, "*")
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Check for multi-host routing: /_h/{hostname}/path
	// This allows proxying to additional hosts specified in the session
	var targetBaseURL string
	var targetHost string

	if strings.HasPrefix(path, "/_h/") {
		// Extract hostname from path: /_h/{hostname}/rest/of/path
		parts := strings.SplitN(path[4:], "/", 2) // Remove "/_h/" prefix
		if len(parts) >= 1 && parts[0] != "" {
			targetHost = parts[0]
			if len(parts) == 2 {
				path = "/" + parts[1]
			} else {
				path = "/"
			}
			// Look up the target URL for this host
			targetBaseURL = session.GetTargetForHost(targetHost)
			if targetBaseURL == session.TargetURL && targetHost != "" {
				// Host not in our list - check if it's a known additional host
				// If not, reject to prevent open proxy
				if _, exists := session.TargetHosts[targetHost]; !exists {
					log.Warn().Str("host", targetHost).Msg("Attempted to proxy to unknown host")
					respondBadRequest(w, "Host not configured for this proxy session")
					return
				}
			}
		} else {
			targetBaseURL = session.TargetURL
		}
	} else {
		targetBaseURL = session.TargetURL
	}

	// Build target URL
	targetURL := targetBaseURL + path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	// Read request body
	var requestBody []byte
	if r.Body != nil {
		requestBody, _ = io.ReadAll(r.Body)
		r.Body.Close()
	}

	// Create proxied request
	proxyReq, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		respondInternalError(w, "Failed to create request")
		return
	}

	// Copy headers (except host-specific ones)
	for key, values := range r.Header {
		lowerKey := strings.ToLower(key)
		if lowerKey == "host" || lowerKey == "connection" || lowerKey == "accept-encoding" {
			continue
		}
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Execute request
	resp, err := h.client.Do(proxyReq)
	if err != nil {
		log.Error().Err(err).Str("url", targetURL).Msg("Proxy request failed")
		respondBadGateway(w, "Failed to reach target: "+err.Error())
		return
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, _ := io.ReadAll(resp.Body)

	// Decompress if gzipped
	if resp.Header.Get("Content-Encoding") == "gzip" {
		if gr, err := gzip.NewReader(bytes.NewReader(responseBody)); err == nil {
			responseBody, _ = io.ReadAll(gr)
			gr.Close()
		}
	}

	// Record the request/response if session is active
	if session.IsActive() {
		captured := analysis.CapturedRequest{
			Timestamp:       time.Now(),
			Method:          r.Method,
			Path:            path,
			FullURL:         targetURL,
			RequestHeaders:  flattenHeaders(r.Header),
			RequestBody:     string(requestBody),
			ContentType:     r.Header.Get("Content-Type"),
			StatusCode:      resp.StatusCode,
			ResponseHeaders: flattenHeaders(resp.Header),
			ResponseBody:    truncateBody(string(responseBody), constants.LIMIT__BODY__TRUNCATE__CHARS__MAX),
		}

		session.AddRequest(captured)

		log.Debug().
			Str("method", r.Method).
			Str("path", path).
			Int("status", resp.StatusCode).
			Msg("Captured request")
	}

	// Build proxy base path for URL rewriting
	proxyBasePath := fmt.Sprintf("/proxy/%s", sessionID)

	// Parse primary host from target URL
	primaryParsed, _ := url.Parse(session.TargetURL)
	primaryHost := ""
	if primaryParsed != nil {
		primaryHost = primaryParsed.Host
	}

	// Build multi-host config for rewriting
	multiHostConfig := &rewrite.MultiHostConfig{
		PrimaryHost:   primaryHost,
		TargetHosts:   session.TargetHosts,
		ProxyBasePath: proxyBasePath,
	}

	// Copy response headers, rewriting redirects to go through proxy
	for key, values := range resp.Header {
		lowerKey := strings.ToLower(key)
		// Skip encoding headers since we decompressed
		// Also skip CSP and security headers that might block the proxy
		if lowerKey == "content-encoding" || lowerKey == "content-length" ||
			lowerKey == "content-security-policy" || lowerKey == "x-frame-options" {
			continue
		}
		for _, value := range values {
			// Rewrite Location header for redirects (use multi-host aware version)
			if lowerKey == "location" {
				if len(session.TargetHosts) > 1 {
					value = rewrite.RedirectURLMultiHost(value, multiHostConfig)
				} else {
					value = rewrite.RedirectURL(value, session.TargetURL, proxyBasePath)
				}
			}
			w.Header().Add(key, value)
		}
	}

	// Rewrite URLs in HTML/JS/CSS responses for SPA apps
	contentType := resp.Header.Get("Content-Type")
	shouldRewrite := strings.Contains(contentType, "text/html") ||
		strings.Contains(contentType, "application/javascript") ||
		strings.Contains(contentType, "text/javascript") ||
		strings.Contains(contentType, "application/x-javascript") ||
		strings.Contains(contentType, "text/css")

	if shouldRewrite && len(responseBody) > 0 {
		// Use multi-host rewriting if multiple hosts configured
		if len(session.TargetHosts) > 1 {
			responseBody = rewrite.ResponseBodyMultiHost(responseBody, multiHostConfig)
		} else {
			responseBody = rewrite.ResponseBody(responseBody, session.TargetURL, proxyBasePath)
		}
		// Update content length after rewrite
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(responseBody)))
	}

	// Explicitly delete security headers that might block the proxy (BEFORE WriteHeader!)
	w.Header().Del("Content-Security-Policy")
	w.Header().Del("X-Frame-Options")
	w.Header().Del("X-Content-Type-Options")

	// Add debug header to help troubleshoot
	w.Header().Set("X-Proxy-Target", session.TargetURL)
	w.Header().Set("X-Proxy-Path", path)

	// Write response (headers are sent here)
	w.WriteHeader(resp.StatusCode)
	w.Write(responseBody)
}

// Helper functions

func flattenHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

func truncateBody(body string, maxLen int) string {
	if len(body) > maxLen {
		return body[:maxLen] + "...[truncated]"
	}
	return body
}

package analysis

import "time"

// CapturedRequest represents a single captured HTTP request/response
type CapturedRequest struct {
	Timestamp       time.Time         `json:"timestamp"`
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	FullURL         string            `json:"full_url"`
	RequestHeaders  map[string]string `json:"request_headers"`
	RequestBody     string            `json:"request_body,omitempty"`
	ContentType     string            `json:"content_type,omitempty"`
	StatusCode      int               `json:"status_code"`
	ResponseHeaders map[string]string `json:"response_headers"`
	ResponseBody    string            `json:"response_body,omitempty"`
}

// TrafficAnalysis contains the analysis results
type TrafficAnalysis struct {
	Success          bool                   `json:"success"`
	Message          string                 `json:"message"`
	TargetURL        string                 `json:"target_url"`
	TotalRequests    int                    `json:"total_requests"`
	DetectedAuthType string                 `json:"detected_auth_type"`
	LoginEndpoint    *DetectedEndpoint      `json:"login_endpoint,omitempty"`
	CreateEndpoint   *DetectedEndpoint      `json:"create_endpoint,omitempty"`
	RevokeEndpoint   *DetectedEndpoint      `json:"revoke_endpoint,omitempty"`
	RotateEndpoint   *DetectedEndpoint      `json:"rotate_endpoint,omitempty"`
	AuthHeaders      map[string]string      `json:"auth_headers,omitempty"`
	SuggestedConfig  map[string]interface{} `json:"suggested_config,omitempty"`
	AllEndpoints     []DetectedEndpoint     `json:"all_endpoints"`
	// API Spec discovery
	SpecFound     bool               `json:"spec_found"`
	SpecURL       string             `json:"spec_url,omitempty"`
	SpecEndpoints []DetectedEndpoint `json:"spec_endpoints,omitempty"`
}

// DetectedEndpoint represents a discovered API endpoint
type DetectedEndpoint struct {
	Path         string `json:"path"`
	Method       string `json:"method"`
	ContentType  string `json:"content_type,omitempty"`
	BodyTemplate string `json:"body_template,omitempty"`
	Description  string `json:"description,omitempty"`
	Confidence   string `json:"confidence,omitempty"`
	Source       string `json:"source,omitempty"`    // "recording" or "spec"
	Operation    string `json:"operation,omitempty"` // For spec: operationId
}

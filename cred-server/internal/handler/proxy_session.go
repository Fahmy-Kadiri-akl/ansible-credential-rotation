package handler

import (
	"sync"
	"time"

	"github.com/akeyless-community/cred-server/internal/analysis"
)

// RecordingSession stores captured traffic for a discovery session
type RecordingSession struct {
	ID          string                     `json:"id"`
	TargetURL   string                     `json:"target_url"`             // Primary target (for backwards compatibility)
	TargetHosts map[string]string          `json:"target_hosts,omitempty"` // hostname -> base URL mapping
	StartedAt   time.Time                  `json:"started_at"`
	Active      bool                       `json:"active"`
	Requests    []analysis.CapturedRequest `json:"requests"`
	mu          sync.Mutex
}

// GetTargetForHost returns the base URL for a given hostname
// Falls back to primary TargetURL if host not found
func (s *RecordingSession) GetTargetForHost(hostname string) string {
	if s.TargetHosts != nil {
		if target, ok := s.TargetHosts[hostname]; ok {
			return target
		}
	}
	return s.TargetURL
}

// GetAllHosts returns all hostnames that are being proxied
func (s *RecordingSession) GetAllHosts() []string {
	hosts := make([]string, 0, len(s.TargetHosts)+1)
	// Add hosts from TargetHosts map
	for host := range s.TargetHosts {
		hosts = append(hosts, host)
	}
	return hosts
}

// GetTargetURL implements the analysis.Session interface
func (s *RecordingSession) GetTargetURL() string {
	return s.TargetURL
}

// GetRequests implements the analysis.Session interface
func (s *RecordingSession) GetRequests() []analysis.CapturedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Return a copy to avoid race conditions
	requests := make([]analysis.CapturedRequest, len(s.Requests))
	copy(requests, s.Requests)
	return requests
}

// AddRequest adds a captured request to the session
func (s *RecordingSession) AddRequest(req analysis.CapturedRequest) {
	s.mu.Lock()
	s.Requests = append(s.Requests, req)
	s.mu.Unlock()
}

// Stop marks the session as inactive and returns the request count
func (s *RecordingSession) Stop() int {
	s.mu.Lock()
	s.Active = false
	count := len(s.Requests)
	s.mu.Unlock()
	return count
}

// IsActive returns whether the session is still recording
func (s *RecordingSession) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Active
}

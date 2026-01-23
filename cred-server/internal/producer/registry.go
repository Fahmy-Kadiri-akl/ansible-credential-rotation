package producer

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ProducerType defines the type of credential producer
type ProducerType string

const (
	TypeRESTAPI  ProducerType = "rest-api"
	TypeWebForm  ProducerType = "web-form"
	TypeDatabase ProducerType = "database"
	TypeCustom   ProducerType = "custom"
)

// Producer represents a credential producer configuration
type Producer struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        ProducerType    `json:"type"`
	Description string          `json:"description"`
	TargetURL   string          `json:"target_url"`
	Config      *ProducerConfig `json:"config"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ProducerConfig holds the producer-specific configuration
type ProducerConfig struct {
	// REST API configuration
	AuthType      string `json:"auth_type,omitempty"`      // bearer, basic, api_key
	AuthHeader    string `json:"auth_header,omitempty"`    // Authorization, X-API-Key
	AuthPrefix    string `json:"auth_prefix,omitempty"`    // "Bearer ", "Basic "
	AdminUsername string `json:"admin_username,omitempty"` // Admin username (stored reference)
	AdminPassword string `json:"admin_password,omitempty"` // Admin password (stored reference)

	// Endpoints
	CreateEndpoint *EndpointConfig `json:"create_endpoint,omitempty"`
	RevokeEndpoint *EndpointConfig `json:"revoke_endpoint,omitempty"`
	RotateEndpoint *EndpointConfig `json:"rotate_endpoint,omitempty"`

	// Response parsing
	CredentialIDPath string `json:"credential_id_path,omitempty"` // JSON path to extract credential ID
	TokenPath        string `json:"token_path,omitempty"`         // JSON path to extract token/password

	// Custom fields
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

// EndpointConfig holds endpoint-specific configuration
type EndpointConfig struct {
	Path         string            `json:"path"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers,omitempty"`
	BodyTemplate string            `json:"body_template,omitempty"`
}

// Credential represents a created credential
type Credential struct {
	ID          string          `json:"id"`
	ProducerID  string          `json:"producer_id"`
	ExternalID  string          `json:"external_id"` // ID in the target system
	Username    string          `json:"username,omitempty"`
	Token       string          `json:"token,omitempty"`
	RawResponse json.RawMessage `json:"raw_response,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"`
}

// Registry manages producers and credentials in memory
type Registry struct {
	mu          sync.RWMutex
	producers   map[string]*Producer
	credentials map[string]*Credential
}

// NewRegistry creates a new producer registry
func NewRegistry() *Registry {
	return &Registry{
		producers:   make(map[string]*Producer),
		credentials: make(map[string]*Credential),
	}
}

// GetProducer returns a producer by ID
func (r *Registry) GetProducer(id string) (*Producer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.producers[id]
	if !ok {
		return nil, fmt.Errorf("producer not found: %s", id)
	}
	return p, nil
}

// ListProducers returns all producers
func (r *Registry) ListProducers() []*Producer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	producers := make([]*Producer, 0, len(r.producers))
	for _, p := range r.producers {
		producers = append(producers, p)
	}
	return producers
}

// AddProducer adds a new producer
func (r *Registry) AddProducer(p *Producer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.producers[p.ID]; exists {
		return fmt.Errorf("producer already exists: %s", p.ID)
	}

	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	r.producers[p.ID] = p

	log.Info().Str("id", p.ID).Str("name", p.Name).Msg("Producer added")
	return nil
}

// DeleteProducer removes a producer
func (r *Registry) DeleteProducer(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.producers[id]; !exists {
		return fmt.Errorf("producer not found: %s", id)
	}

	delete(r.producers, id)
	log.Info().Str("id", id).Msg("Producer deleted")
	return nil
}

// StoreCredential stores a created credential
func (r *Registry) StoreCredential(c *Credential) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.credentials[c.ID] = c
	log.Debug().Str("id", c.ID).Str("producer", c.ProducerID).Msg("Credential stored")
}

// GetCredential returns a credential by ID
func (r *Registry) GetCredential(id string) (*Credential, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.credentials[id]
	return c, ok
}

// DeleteCredential removes a credential
func (r *Registry) DeleteCredential(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.credentials, id)
	log.Debug().Str("id", id).Msg("Credential deleted")
}

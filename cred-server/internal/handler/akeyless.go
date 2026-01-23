package handler

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/akeyless-community/cred-server/internal/config"
	akeyless "github.com/akeylesslabs/akeyless-go/v3"
	"github.com/rs/zerolog/log"
)

// AkeylessClient handles communication with Akeyless Gateway/API using the official SDK
type AkeylessClient struct {
	client      *akeyless.V2ApiService
	gatewayURL  string
	accessID    string
	accessKey   string
	syncURLBase string // Base URL for sync endpoints (for gateway to reach cred-server)
	token       string
	ctx         context.Context
}

// NewAkeylessClient creates a new Akeyless API client using the SDK
func NewAkeylessClient(cfg *config.Config) *AkeylessClient {
	gatewayURL := cfg.AkeylessGateway

	// Create SDK configuration
	sdkCfg := akeyless.NewConfiguration()

	// Configure for Gateway mode (uses /api/v2/ prefix)
	// If URL is not the SaaS endpoint, add /api/v2 to the base path
	if gatewayURL != "https://api.akeyless.io" {
		sdkCfg.Servers = akeyless.ServerConfigurations{
			{
				URL: gatewayURL + "/api/v2",
			},
		}
	} else {
		sdkCfg.Servers = akeyless.ServerConfigurations{
			{
				URL: gatewayURL,
			},
		}
	}

	// Configure HTTP client with TLS skip verify
	sdkCfg.HTTPClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	return &AkeylessClient{
		client:      akeyless.NewAPIClient(sdkCfg).V2Api,
		gatewayURL:  gatewayURL,
		accessID:    cfg.AkeylessAccessID,
		accessKey:   cfg.AkeylessAccessKey,
		syncURLBase: cfg.SyncURLBase,
		ctx:         context.Background(),
	}
}

// Authenticate gets a token from Akeyless using the SDK
func (c *AkeylessClient) Authenticate() error {
	if c.accessID == "" {
		return fmt.Errorf("AKEYLESS_ACCESS_ID not configured")
	}

	authBody := akeyless.NewAuth()
	authBody.SetAccessType("api_key")
	authBody.SetAccessId(c.accessID)
	authBody.SetAccessKey(c.accessKey)

	authResp, _, err := c.client.Auth(c.ctx).Body(*authBody).Execute()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	token := authResp.GetToken()
	if token == "" {
		return fmt.Errorf("authentication succeeded but no token received")
	}

	c.token = token
	log.Info().Msg("Successfully authenticated with Akeyless")
	return nil
}

// ValidateToken checks if the current token is still valid
func (c *AkeylessClient) ValidateToken() bool {
	if c.token == "" {
		return false
	}

	validateBody := akeyless.NewGetAccountSettingsWithDefaults()
	validateBody.SetToken(c.token)

	_, _, err := c.client.GetAccountSettings(c.ctx).Body(*validateBody).Execute()
	if err != nil {
		log.Debug().Err(err).Msg("Token validation failed")
		return false
	}

	return true
}

// EnsureValidToken validates the current token and re-authenticates if needed
func (c *AkeylessClient) EnsureValidToken() error {
	if c.ValidateToken() {
		log.Debug().Msg("Token is valid")
		return nil
	}

	log.Info().Msg("Token invalid or expired, re-authenticating...")
	return c.Authenticate()
}

// CreateDynamicSecretRequest represents the request to create a custom producer
type CreateDynamicSecretRequest struct {
	Name                      string   `json:"name"`
	ProducerID                string   `json:"producer_id"`
	TargetName                string   `json:"target_name,omitempty"`
	UserTTL                   string   `json:"user_ttl,omitempty"`
	TimeoutSeconds            int64    `json:"timeout_sec,omitempty"`
	Tags                      []string `json:"tags,omitempty"`
	AdminRotationIntervalDays int      `json:"admin_rotation_interval_days,omitempty"`
	EnableAdminRotation       bool     `json:"enable_admin_rotation,omitempty"`
	SyncURLBase               string   `json:"sync_url_base,omitempty"` // Override sync URL (from request Host header)
}

// CreateCustomProducer creates a new custom producer dynamic secret in Akeyless using the SDK
func (c *AkeylessClient) CreateCustomProducer(req *CreateDynamicSecretRequest) error {
	if err := c.EnsureValidToken(); err != nil {
		return err
	}

	// Use request's sync URL if provided, otherwise fall back to client default
	syncURLBase := req.SyncURLBase
	if syncURLBase == "" {
		syncURLBase = c.syncURLBase
	}

	// Build the sync URLs pointing to our cred-server
	createURL := fmt.Sprintf("%s/sync/create", syncURLBase)
	revokeURL := fmt.Sprintf("%s/sync/revoke", syncURLBase)
	rotateURL := fmt.Sprintf("%s/sync/rotate", syncURLBase)

	// Build the payload that will be sent to cred-server
	payload := fmt.Sprintf(`{"producer_id":"%s"}`, req.ProducerID)

	// Create the dynamic secret using the SDK with required parameters
	// NewDynamicSecretCreateCustom requires: createSyncUrl, name, revokeSyncUrl
	body := akeyless.NewDynamicSecretCreateCustom(createURL, req.Name, revokeURL)
	body.SetPayload(payload)
	body.SetToken(c.token)

	// Add optional rotate URL
	if rotateURL != "" {
		body.SetRotateSyncUrl(rotateURL)
	}
	if req.TimeoutSeconds > 0 {
		body.SetTimeoutSec(req.TimeoutSeconds)
	}
	if req.UserTTL != "" {
		body.SetUserTtl(req.UserTTL)
	}
	if len(req.Tags) > 0 {
		body.SetTags(req.Tags)
	}
	if req.AdminRotationIntervalDays > 0 {
		body.SetAdminRotationIntervalDays(int64(req.AdminRotationIntervalDays))
	}
	if req.EnableAdminRotation {
		body.SetEnableAdminRotation(req.EnableAdminRotation)
	}

	log.Info().
		Str("name", req.Name).
		Str("producer_id", req.ProducerID).
		Str("create_url", createURL).
		Str("sync_url_base", syncURLBase).
		Msg("Creating custom producer dynamic secret in Akeyless")

	_, _, err := c.client.DynamicSecretCreateCustom(c.ctx).Body(*body).Execute()
	if err != nil {
		return fmt.Errorf("failed to create custom producer: %w", err)
	}

	log.Info().Str("name", req.Name).Msg("Custom producer dynamic secret created successfully")
	return nil
}

// GetDynamicSecretValue gets a dynamic secret value (for testing)
func (c *AkeylessClient) GetDynamicSecretValue(name string) (map[string]interface{}, error) {
	if err := c.EnsureValidToken(); err != nil {
		return nil, err
	}

	body := akeyless.NewGetDynamicSecretValueWithDefaults()
	body.SetName(name)
	body.SetToken(c.token)

	resp, _, err := c.client.GetDynamicSecretValue(c.ctx).Body(*body).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get dynamic secret value: %w", err)
	}

	// Convert the response to a map[string]interface{}
	result := make(map[string]interface{})
	for k, v := range resp {
		result[k] = v
	}

	return result, nil
}

// DeleteDynamicSecret deletes a dynamic secret
func (c *AkeylessClient) DeleteDynamicSecret(name string) error {
	if err := c.EnsureValidToken(); err != nil {
		return err
	}

	body := akeyless.NewDeleteItemWithDefaults()
	body.SetName(name)
	body.SetToken(c.token)

	_, _, err := c.client.DeleteItem(c.ctx).Body(*body).Execute()
	if err != nil {
		return fmt.Errorf("failed to delete dynamic secret: %w", err)
	}

	log.Info().Str("name", name).Msg("Dynamic secret deleted")
	return nil
}

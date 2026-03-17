// Package ansible provides an HTTP client for the Ansible Automation Platform
// (AAP/AWX) API. It handles user password updates, API token lifecycle, and
// credential object management.
package ansible

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client communicates with the Ansible AAP/AWX API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Ansible API client.
// baseURL should be the AAP/AWX controller URL, e.g. "https://ansible.example.com".
func NewClient(baseURL string, skipTLSVerify bool) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerify},
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// AuthMethod specifies how to authenticate to the Ansible API.
type AuthMethod struct {
	// Username + Password (basic auth)
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// OAuth2 token (bearer auth) - used when managing API tokens
	Token string `json:"token,omitempty"`
}

func (c *Client) authHeader(auth AuthMethod) (string, string) {
	if auth.Token != "" {
		return "Authorization", "Bearer " + auth.Token
	}
	return "", ""
}

func (c *Client) do(ctx context.Context, method, path string, auth AuthMethod, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		bs, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(bs)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if auth.Token != "" {
		req.Header.Set("Authorization", "Bearer "+auth.Token)
	} else if auth.Username != "" {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// UpdateUserPassword changes the password for an Ansible user.
// Requires admin credentials or the user's own current password.
func (c *Client) UpdateUserPassword(ctx context.Context, auth AuthMethod, userID int, newPassword string) error {
	path := fmt.Sprintf("/api/v2/users/%d/", userID)
	body := map[string]string{"password": newPassword}

	respBody, status, err := c.do(ctx, http.MethodPatch, path, auth, body)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("update user password: HTTP %d: %s", status, string(respBody))
	}
	return nil
}

// LookupUserByUsername finds a user by username and returns their ID.
func (c *Client) LookupUserByUsername(ctx context.Context, auth AuthMethod, username string) (int, error) {
	path := fmt.Sprintf("/api/v2/users/?username=%s", username)

	respBody, status, err := c.do(ctx, http.MethodGet, path, auth, nil)
	if err != nil {
		return 0, fmt.Errorf("lookup user: %w", err)
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("lookup user: HTTP %d: %s", status, string(respBody))
	}

	var result struct {
		Count   int `json:"count"`
		Results []struct {
			ID int `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("parse user lookup response: %w", err)
	}
	if result.Count == 0 {
		return 0, fmt.Errorf("user %q not found", username)
	}
	return result.Results[0].ID, nil
}

// TokenResponse represents an Ansible personal access token.
type TokenResponse struct {
	ID      int    `json:"id"`
	Token   string `json:"token"`
	Expires string `json:"expires,omitempty"`
}

// CreatePersonalToken creates a new personal access token for the given user.
func (c *Client) CreatePersonalToken(ctx context.Context, auth AuthMethod, userID int, description string, scope string) (*TokenResponse, error) {
	path := fmt.Sprintf("/api/v2/users/%d/personal_tokens/", userID)
	body := map[string]string{
		"description": description,
		"scope":       scope,
	}

	respBody, status, err := c.do(ctx, http.MethodPost, path, auth, body)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}
	if status != http.StatusCreated {
		return nil, fmt.Errorf("create token: HTTP %d: %s", status, string(respBody))
	}

	var token TokenResponse
	if err := json.Unmarshal(respBody, &token); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	return &token, nil
}

// RevokeToken deletes a personal access token by ID.
func (c *Client) RevokeToken(ctx context.Context, auth AuthMethod, tokenID int) error {
	path := fmt.Sprintf("/api/v2/tokens/%d/", tokenID)

	_, status, err := c.do(ctx, http.MethodDelete, path, auth, nil)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	if status != http.StatusNoContent && status != http.StatusOK {
		return fmt.Errorf("revoke token: unexpected HTTP %d", status)
	}
	return nil
}

// UpdateCredential updates a credential object in Ansible by ID.
// Fields map should contain the credential input fields to update.
func (c *Client) UpdateCredential(ctx context.Context, auth AuthMethod, credentialID int, inputs map[string]string) error {
	path := fmt.Sprintf("/api/v2/credentials/%d/", credentialID)
	body := map[string]interface{}{"inputs": inputs}

	respBody, status, err := c.do(ctx, http.MethodPatch, path, auth, body)
	if err != nil {
		return fmt.Errorf("update credential: %w", err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("update credential: HTTP %d: %s", status, string(respBody))
	}
	return nil
}

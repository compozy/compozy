package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// client implements the AuthClient interface
type client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewAuthClient creates a new auth client that implements AuthClient interface
func NewAuthClient(cfg *config.Config, apiKey string) (AuthClient, error) {
	baseURL, err := getBaseURL(cfg)
	if err != nil {
		return nil, err
	}

	return &client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
		apiKey:  apiKey,
	}, nil
}

// getBaseURL determines the base URL from configuration with proper precedence
func getBaseURL(cfg *config.Config) (string, error) {
	// Highest precedence: CLI base_url
	if cfg.CLI.BaseURL != "" {
		if _, err := url.Parse(cfg.CLI.BaseURL); err != nil {
			return "", fmt.Errorf("invalid base URL from CLI config: %w", err)
		}
		return cfg.CLI.BaseURL, nil
	}

	// Fallback: construct from server host and port
	scheme := "https"
	if cfg.Server.Host == "localhost" || cfg.Server.Host == "127.0.0.1" || cfg.Server.Host == "0.0.0.0" ||
		cfg.Server.Host == "[::1]" ||
		cfg.Server.Host == "::1" {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s:%d/api/v0", scheme, cfg.Server.Host, cfg.Server.Port)

	// Final validation
	if _, err := url.Parse(baseURL); err != nil {
		return "", fmt.Errorf("invalid base URL constructed from server config: %w", err)
	}

	return baseURL, nil
}

// GetBaseURL returns the base URL for the client
func (c *client) GetBaseURL() string {
	return c.baseURL
}

// GetAPIKey returns the API key for the client
func (c *client) GetAPIKey() string {
	return c.apiKey
}

// prepareRequestBody marshals the body to JSON if provided
func (c *client) prepareRequestBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	return bytes.NewReader(jsonBody), nil
}

// createRequest creates an HTTP request with appropriate headers
func (c *client) createRequest(ctx context.Context, method, url string, bodyReader io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return req, nil
}

// shouldRetry determines if a response should trigger a retry
func (c *client) shouldRetry(resp *http.Response, attempt, maxRetries int) bool {
	// Don't retry on client errors (4xx)
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return false
	}
	// Retry on server errors (5xx) if not at max attempts
	return resp.StatusCode >= 500 && attempt < maxRetries
}

// doRequest performs an HTTP request with retry logic
func (c *client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	log := logger.FromContext(ctx)
	url := c.baseURL + path
	maxRetries := 3
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Debug("Retrying request", "attempt", attempt, "backoff", backoff)
			timer := time.NewTimer(backoff)
			select {
			case <-timer.C:
				backoff *= 2 // Exponential backoff
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			}
		}

		// Create a fresh body reader for each attempt to avoid issues with consumed streams
		bodyReader, err := c.prepareRequestBody(body)
		if err != nil {
			return nil, err
		}

		req, err := c.createRequest(ctx, method, url, bodyReader)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if attempt == maxRetries {
				return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, err)
			}
			continue
		}

		if c.shouldRetry(resp, attempt, maxRetries) {
			if closeErr := resp.Body.Close(); closeErr != nil {
				log.Debug("Failed to close response body", "error", closeErr)
			}
			if attempt == maxRetries {
				return nil, fmt.Errorf("server error (status %d) after %d attempts", resp.StatusCode, maxRetries+1)
			}
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after all retry attempts")
}

// parseResponse parses the API response
func (c *client) parseResponse(resp *http.Response, result any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp models.APIResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
		}
		if errResp.Error != "" {
			return fmt.Errorf("API error: %s", errResp.Error)
		}
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// GenerateKey generates a new API key
func (c *client) GenerateKey(ctx context.Context, req *GenerateKeyRequest) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/auth/generate", req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			APIKey string `json:"api_key"`
		} `json:"data"`
		Message string `json:"message"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return "", err
	}

	return result.Data.APIKey, nil
}

// ListKeys lists all API keys for the authenticated user
func (c *client) ListKeys(ctx context.Context) ([]KeyInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/auth/keys", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Keys []KeyInfo `json:"keys"`
		} `json:"data"`
		Message string `json:"message"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Data.Keys, nil
}

// RevokeKey revokes an API key by ID
func (c *client) RevokeKey(ctx context.Context, keyID string) error {
	path := fmt.Sprintf("/auth/keys/%s", keyID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.parseResponse(resp, nil)
}

// ListUsers lists all users (admin only)
func (c *client) ListUsers(ctx context.Context) ([]UserInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/users", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Users []UserInfo `json:"users"`
		} `json:"data"`
		Message string `json:"message"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Data.Users, nil
}

// CreateUser creates a new user (admin only)
func (c *client) CreateUser(ctx context.Context, req CreateUserRequest) (*UserInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/users", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data    UserInfo `json:"data"`
		Message string   `json:"message"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// UpdateUser updates a user (admin only)
func (c *client) UpdateUser(ctx context.Context, userID string, req UpdateUserRequest) (*UserInfo, error) {
	path := fmt.Sprintf("/users/%s", userID)
	resp, err := c.doRequest(ctx, http.MethodPatch, path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data    UserInfo `json:"data"`
		Message string   `json:"message"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// DeleteUser deletes a user (admin only)
func (c *client) DeleteUser(ctx context.Context, userID string) error {
	path := fmt.Sprintf("/users/%s", userID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.parseResponse(resp, nil)
}

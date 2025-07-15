package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/compozy/compozy/cli/auth/tui/models"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// Client represents an HTTP client for auth API operations
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient creates a new auth API client
func NewClient(cfg *config.Config, apiKey string) (*Client, error) {
	baseURL := fmt.Sprintf("https://%s:%d/api/v0", cfg.Server.Host, cfg.Server.Port)

	// Parse and validate base URL
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
		apiKey:  apiKey,
	}, nil
}

// prepareRequestBody marshals the body to JSON if provided
func (c *Client) prepareRequestBody(body any) (io.Reader, error) {
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
func (c *Client) createRequest(ctx context.Context, method, url string, bodyReader io.Reader) (*http.Request, error) {
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
func (c *Client) shouldRetry(resp *http.Response, attempt, maxRetries int) bool {
	// Don't retry on client errors (4xx)
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return false
	}

	// Retry on server errors (5xx) if not at max attempts
	return resp.StatusCode >= 500 && attempt < maxRetries
}

// doRequest performs an HTTP request with retry logic
func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	log := logger.FromContext(ctx)

	bodyReader, err := c.prepareRequestBody(body)
	if err != nil {
		return nil, err
	}

	url := c.baseURL + path
	maxRetries := 3
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Debug("Retrying request", "attempt", attempt, "backoff", backoff)
			select {
			case <-time.After(backoff):
				backoff *= 2 // Exponential backoff
			case <-ctx.Done():
				return nil, ctx.Err()
			}
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
func (c *Client) parseResponse(resp *http.Response, result any) error {
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log but don't fail on close error
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

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

// GenerateKeyRequest represents the request to generate an API key
type GenerateKeyRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Expires     string `json:"expires,omitempty"`
}

// GenerateKey generates a new API key
func (c *Client) GenerateKey(ctx context.Context, req *GenerateKeyRequest) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/auth/generate", req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		APIKey  string `json:"api_key"`
		Message string `json:"message"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return "", err
	}

	return result.APIKey, nil
}

// ListKeys lists all API keys for the authenticated user
func (c *Client) ListKeys(ctx context.Context) ([]models.KeyInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/auth/keys", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Keys []models.KeyInfo `json:"keys"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Keys, nil
}

// RevokeKey revokes an API key by ID
func (c *Client) RevokeKey(ctx context.Context, keyID string) error {
	path := fmt.Sprintf("/auth/keys/%s", keyID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.parseResponse(resp, nil)
}

// ListUsers lists all users (admin only)
func (c *Client) ListUsers(ctx context.Context) ([]models.UserInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/users", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Users []models.UserInfo `json:"users"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Users, nil
}

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// CreateUser creates a new user (admin only)
func (c *Client) CreateUser(ctx context.Context, req CreateUserRequest) (*models.UserInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/users", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user models.UserInfo
	if err := c.parseResponse(resp, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Email *string `json:"email,omitempty"`
	Name  *string `json:"name,omitempty"`
	Role  *string `json:"role,omitempty"`
}

// UpdateUser updates a user (admin only)
func (c *Client) UpdateUser(ctx context.Context, userID string, req UpdateUserRequest) (*models.UserInfo, error) {
	path := fmt.Sprintf("/users/%s", userID)
	resp, err := c.doRequest(ctx, http.MethodPatch, path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user models.UserInfo
	if err := c.parseResponse(resp, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// DeleteUser deletes a user (admin only)
func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	path := fmt.Sprintf("/users/%s", userID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.parseResponse(resp, nil)
}

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/engine/infra/server/routes"
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
	if cfg.CLI.BaseURL != "" {
		if _, err := url.Parse(cfg.CLI.BaseURL); err != nil {
			return "", fmt.Errorf("invalid base URL from CLI config: %w", err)
		}
		return strings.TrimRight(cfg.CLI.BaseURL, "/"), nil
	}
	scheme := "https"
	if cfg.Server.Host == "localhost" || cfg.Server.Host == "127.0.0.1" || cfg.Server.Host == "0.0.0.0" ||
		cfg.Server.Host == "[::1]" ||
		cfg.Server.Host == "::1" {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s:%d%s", scheme, cfg.Server.Host, cfg.Server.Port, routes.Base())
	baseURL = strings.TrimRight(baseURL, "/")
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
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	return req, nil
}

// shouldRetry determines if a response should trigger a retry
func (c *client) shouldRetry(resp *http.Response, attempt, maxRetries int) bool {
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return false
	}
	return resp.StatusCode >= 500 && attempt < maxRetries
}

// doRequest performs an HTTP request with retry logic
func (c *client) doRequest(
	ctx context.Context,
	method, path string,
	body any,
	headers map[string]string,
) (*http.Response, error) {
	log := logger.FromContext(ctx)
	cfg := config.FromContext(ctx)
	maxRetries := config.DefaultCLIMaxRetries
	if cfg != nil {
		switch {
		case cfg.CLI.MaxRetries < 0:
			maxRetries = 0
		case cfg.CLI.MaxRetries == 0:
			maxRetries = config.DefaultCLIMaxRetries
		default:
			maxRetries = cfg.CLI.MaxRetries
		}
	}
	url := c.baseURL + path
	backoff := 100 * time.Millisecond
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Debug("Retrying request", "attempt", attempt, "backoff", backoff)
			if err := waitForBackoff(ctx, backoff); err != nil {
				return nil, err
			}
			backoff *= 2
		}

		resp, err := c.executeRequestAttempt(ctx, method, url, body, headers)
		if err != nil {
			if attempt == maxRetries {
				return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries+1, err)
			}
			continue
		}

		if c.shouldRetry(resp, attempt, maxRetries) {
			closeResponseBody(ctx, resp.Body)
			if attempt == maxRetries {
				return nil, fmt.Errorf("server error (status %d) after %d attempts", resp.StatusCode, maxRetries+1)
			}
			continue
		}

		return resp, nil
	}
	return nil, fmt.Errorf("request failed after all retry attempts")
}

func (c *client) executeRequestAttempt(
	ctx context.Context,
	method, url string,
	body any,
	headers map[string]string,
) (*http.Response, error) {
	bodyReader, err := c.prepareRequestBody(body)
	if err != nil {
		return nil, err
	}
	req, err := c.createRequest(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	applyRequestHeaders(req, headers)
	return c.httpClient.Do(req)
}

func applyRequestHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
}

func waitForBackoff(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func closeResponseBody(ctx context.Context, body io.ReadCloser) {
	if body == nil {
		return
	}
	if err := body.Close(); err != nil {
		log := logger.FromContext(ctx)
		log.Debug("Failed to close response body", "error", err)
	}
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

// CallGET performs a raw GET request to a path under the API base URL using the concrete client.
// It parses the standard API envelope and returns any server-side error as Go error.
func CallGET(ctx context.Context, ac AuthClient, path string) error {
	_, err := callDecode(ctx, ac, http.MethodGet, path, nil, nil)
	return err
}

// CallGETDecode performs GET and decodes the response envelope for callers who
// want to display additional details.
func CallGETDecode(ctx context.Context, ac AuthClient, path string) (*models.APIResponse, error) {
	return callDecode(ctx, ac, http.MethodGet, path, nil, nil)
}

// CallPOSTDecode performs POST and decodes the response envelope for callers who
// want to display additional details. The body is JSON-encoded when non-nil.
func CallPOSTDecode(ctx context.Context, ac AuthClient, path string, body any) (*models.APIResponse, error) {
	return callDecode(ctx, ac, http.MethodPost, path, body, nil)
}

// CallPUTDecode performs PUT and decodes the response envelope.
func CallPUTDecode(
	ctx context.Context,
	ac AuthClient,
	path string,
	body any,
	headers map[string]string,
) (*models.APIResponse, error) {
	return callDecode(ctx, ac, http.MethodPut, path, body, headers)
}

// CallDELETE performs DELETE and returns any server error.
func CallDELETE(ctx context.Context, ac AuthClient, path string, headers map[string]string) error {
	_, err := callDecode(ctx, ac, http.MethodDelete, path, nil, headers)
	return err
}

// callDecode is an internal helper that executes the request and decodes the API envelope.
func callDecode(
	ctx context.Context,
	ac AuthClient,
	method, path string,
	body any,
	headers map[string]string,
) (*models.APIResponse, error) {
	impl, ok := ac.(*client)
	if !ok {
		return nil, fmt.Errorf("unsupported client implementation")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	resp, err := impl.doRequest(ctx, method, path, body, headers)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			logger.FromContext(ctx).Debug("Failed to close response body", "error", cerr)
		}
	}()
	var envelope models.APIResponse
	if err := impl.parseResponse(resp, &envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}

// GenerateKey generates a new API key
func (c *client) GenerateKey(ctx context.Context, req *GenerateKeyRequest) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/auth/generate", req, nil)
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
	resp, err := c.doRequest(ctx, http.MethodGet, "/auth/keys", nil, nil)
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
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.parseResponse(resp, nil)
}

// ListUsers lists all users (admin only)
func (c *client) ListUsers(ctx context.Context) ([]UserInfo, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/users", nil, nil)
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
	resp, err := c.doRequest(ctx, http.MethodPost, "/users", req, nil)
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
	resp, err := c.doRequest(ctx, http.MethodPatch, path, req, nil)
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
func (c *client) DeleteUser(ctx context.Context, userID string, opts DeleteUserOptions) error {
	path := fmt.Sprintf("/users/%s", url.PathEscape(userID))
	if opts.Cascade {
		path += "?cascade=true"
	}
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.parseResponse(resp, nil)
}

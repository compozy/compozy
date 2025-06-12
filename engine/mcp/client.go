package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/logger"
)

// Definition represents the structure for registering an MCP with the proxy
type Definition struct {
	Name        string            `json:"name"`
	Command     []string          `json:"command,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Transport   string            `json:"transport"`
	StartupArgs []string          `json:"startup_args,omitempty"`
}

// Client provides HTTP communication with the MCP proxy service
type Client struct {
	baseURL   string
	http      *http.Client
	adminTok  string
	retryConf RetryConfig
}

// RetryConfig configures retry behavior for proxy operations
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   200 * time.Millisecond,
		MaxDelay:    2 * time.Second,
	}
}

// NewProxyClient creates a new proxy client with the specified configuration
func NewProxyClient(baseURL, adminToken string, timeout time.Duration) *Client {
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return &Client{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		adminTok: adminToken,
		http: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
				DisableKeepAlives:  false,
			},
		},
		retryConf: DefaultRetryConfig(),
	}
}

// Health checks if the proxy service is healthy and accessible
func (c *Client) Health(ctx context.Context) error {
	start := time.Now()
	defer TimeOperation("proxy_health_check", start)
	IncrementProxyHealthCheck()

	err := c.withRetry(ctx, "health check", func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/healthz", http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create health check request: %w", err)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("health check request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			}
			return fmt.Errorf("proxy service unhealthy (status %d): %s", resp.StatusCode, string(body))
		}

		logger.Debug("Proxy health check successful", "proxy_url", c.baseURL)
		return nil
	})

	if err != nil {
		IncrementProxyHealthFailure()
	}

	return err
}

// Register registers an MCP definition with the proxy service
func (c *Client) Register(ctx context.Context, def *Definition) error {
	return c.withRetry(ctx, "register MCP", func() error {
		payload, err := json.Marshal(def)
		if err != nil {
			return fmt.Errorf("failed to marshal MCP definition: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/admin/mcps", bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("failed to create register request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if c.adminTok != "" {
			req.Header.Set("Authorization", "Bearer "+c.adminTok)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("register request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		// Handle different status codes
		switch resp.StatusCode {
		case http.StatusCreated:
			logger.Info("Successfully registered MCP with proxy",
				"mcp_name", def.Name, "proxy_url", c.baseURL)
			return nil
		case http.StatusConflict:
			logger.Warn("MCP already registered with proxy",
				"mcp_name", def.Name, "proxy_url", c.baseURL)
			return nil // Treat as success - idempotent operation
		case http.StatusUnauthorized:
			return fmt.Errorf("unauthorized: invalid admin token")
		case http.StatusBadRequest:
			return fmt.Errorf("bad request: %s", string(body))
		default:
			return fmt.Errorf("registration failed (status %d): %s", resp.StatusCode, string(body))
		}
	})
}

// Deregister removes an MCP from the proxy service
func (c *Client) Deregister(ctx context.Context, name string) error {
	return c.withRetry(ctx, "deregister MCP", func() error {
		url := fmt.Sprintf("%s/admin/mcps/%s", c.baseURL, name)
		req, err := http.NewRequestWithContext(ctx, "DELETE", url, http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create deregister request: %w", err)
		}

		if c.adminTok != "" {
			req.Header.Set("Authorization", "Bearer "+c.adminTok)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("deregister request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			logger.Info("Successfully deregistered MCP from proxy",
				"mcp_name", name, "proxy_url", c.baseURL)
			return nil
		case http.StatusNotFound:
			logger.Warn("MCP not found in proxy (already deregistered)",
				"mcp_name", name, "proxy_url", c.baseURL)
			return nil // Treat as success - idempotent operation
		case http.StatusUnauthorized:
			return fmt.Errorf("unauthorized: invalid admin token")
		default:
			return fmt.Errorf("deregistration failed (status %d): %s", resp.StatusCode, string(body))
		}
	})
}

// ListMCPs retrieves all registered MCPs from the proxy
func (c *Client) ListMCPs(ctx context.Context) ([]Definition, error) {
	var mcps []Definition

	err := c.withRetry(ctx, "list MCPs", func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/admin/mcps", http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create list request: %w", err)
		}

		if c.adminTok != "" {
			req.Header.Set("Authorization", "Bearer "+c.adminTok)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("list request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("list failed (status %d): %s", resp.StatusCode, string(body))
		}

		var response struct {
			MCPs []Definition `json:"mcps"`
		}

		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}

		mcps = response.MCPs
		return nil
	})

	if err != nil {
		return nil, err
	}

	return mcps, nil
}

// ToolDefinition represents a tool definition from the proxy
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	MCPName     string         `json:"mcpName"`
}

// ToolsResponse represents the response from the tools endpoint
type ToolsResponse struct {
	Tools []ToolDefinition `json:"tools"`
}

// ListTools retrieves all available tools from registered MCPs via the proxy
func (c *Client) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	var tools []ToolDefinition

	err := c.withRetry(ctx, "list tools", func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/admin/tools", http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create tools request: %w", err)
		}

		if c.adminTok != "" {
			req.Header.Set("Authorization", "Bearer "+c.adminTok)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("tools request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("tools request failed (status %d): %s", resp.StatusCode, string(body))
		}

		var response ToolsResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("failed to unmarshal tools response: %w", err)
		}

		tools = response.Tools
		return nil
	})

	if err != nil {
		return nil, err
	}

	return tools, nil
}

// Close cleans up the HTTP client resources
func (c *Client) Close() error {
	// Close idle connections
	c.http.CloseIdleConnections()
	return nil
}

// withRetry executes the provided function with exponential backoff retry logic
func (c *Client) withRetry(ctx context.Context, operation string, fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= c.retryConf.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on the last attempt or for certain errors
		if attempt == c.retryConf.MaxAttempts || !isRetryableError(err) {
			break
		}

		// Calculate exponential backoff delay
		delay := time.Duration(attempt-1) * c.retryConf.BaseDelay
		if delay > c.retryConf.MaxDelay {
			delay = c.retryConf.MaxDelay
		}

		logger.Warn("Proxy operation failed, retrying",
			"operation", operation,
			"attempt", attempt,
			"max_attempts", c.retryConf.MaxAttempts,
			"delay", delay,
			"error", err)

		select {
		case <-time.After(delay):
			// Continue with retry
		case <-ctx.Done():
			return fmt.Errorf("operation canceled during retry: %w", ctx.Err())
		}
	}
	return fmt.Errorf("operation failed after %d attempts: %w", c.retryConf.MaxAttempts, lastErr)
}

// isRetryableError determines if an error is worth retrying
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "network is unreachable") {
		return true
	}
	// HTTP status code errors - only retry on server errors
	if strings.Contains(errStr, "status 5") {
		return true
	}
	// Don't retry client errors (4xx) or authentication issues
	return false
}

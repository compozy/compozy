package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	"github.com/sethvargo/go-retry"
)

// Definition represents the structure for registering an MCP with the proxy
type Definition struct {
	Name      string                 `json:"name"`
	Env       map[string]string      `json:"env,omitempty"`
	Headers   map[string]string      `json:"headers,omitempty"`
	URL       string                 `json:"url,omitempty"`
	Transport mcpproxy.TransportType `json:"transport"`
	Command   string                 `json:"command,omitempty"`
	Args      []string               `json:"args,omitempty"`
	Timeout   time.Duration          `json:"timeout,omitempty"`
}

// Client provides HTTP communication with the MCP proxy service
type Client struct {
	baseURL   string
	http      *http.Client
	retryConf RetryConfig
}

// RetryConfig configures retry behavior for proxy operations
type RetryConfig struct {
	MaxAttempts uint64
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// ProxyRequestError represents an error from proxy HTTP requests with structured data
type ProxyRequestError struct {
	StatusCode int
	Message    string
	Code       string
	Err        error
}

func (e *ProxyRequestError) Error() string {
	if e.Err != nil {
		if e.Code != "" {
			return fmt.Sprintf(
				"proxy request failed (status %d, code %s): %s: %v",
				e.StatusCode,
				e.Code,
				e.Message,
				e.Err,
			)
		}
		return fmt.Sprintf("proxy request failed (status %d): %s: %v", e.StatusCode, e.Message, e.Err)
	}
	if e.Code != "" {
		return fmt.Sprintf("proxy request failed (status %d, code %s): %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("proxy request failed (status %d): %s", e.StatusCode, e.Message)
}

func (e *ProxyRequestError) Unwrap() error {
	return e.Err
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
func NewProxyClient(baseURL string, timeout time.Duration) *Client {
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
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
	log := logger.FromContext(ctx)
	return c.withRetry(ctx, "health check", func() error {
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
			return &ProxyRequestError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("proxy service unhealthy: %s", string(body)),
			}
		}
		log.Debug("Proxy health check successful", "proxy_url", c.baseURL)
		return nil
	})
}

// Register registers an MCP definition with the proxy service
func (c *Client) Register(ctx context.Context, def *Definition) error {
	log := logger.FromContext(ctx)
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
			log.Info("Successfully registered MCP with proxy",
				"mcp_name", def.Name, "proxy_url", c.baseURL)
			return nil
		case http.StatusConflict:
			log.Warn("MCP already registered with proxy",
				"mcp_name", def.Name, "proxy_url", c.baseURL)
			return nil // Treat as success - idempotent operation
		case http.StatusUnauthorized:
			// Preserve expected substring for tests and human readability
			return &ProxyRequestError{StatusCode: resp.StatusCode, Message: strings.ToLower(string(body))}
		case http.StatusBadRequest:
			return parseProxyError(resp.StatusCode, body)
		default:
			return parseProxyError(resp.StatusCode, body)
		}
	})
}

// Deregister removes an MCP from the proxy service
func (c *Client) Deregister(ctx context.Context, name string) error {
	log := logger.FromContext(ctx)
	return c.withRetry(ctx, "deregister MCP", func() error {
		reqURL := fmt.Sprintf("%s/admin/mcps/%s", c.baseURL, url.PathEscape(name))
		req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create deregister request: %w", err)
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
		case http.StatusOK, http.StatusNoContent:
			log.Info("Successfully deregistered MCP from proxy",
				"mcp_name", name, "proxy_url", c.baseURL)
			return nil
		case http.StatusNotFound:
			log.Warn("MCP not found in proxy (already deregistered)",
				"mcp_name", name, "proxy_url", c.baseURL)
			return nil // Treat as success - idempotent operation
		case http.StatusUnauthorized:
			return parseProxyError(resp.StatusCode, body)
		default:
			return parseProxyError(resp.StatusCode, body)
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
			return &ProxyRequestError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("tools request failed: %s", string(body)),
			}
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

// Status mirrors the runtime status from the proxy admin API
type Status struct {
	Name              string `json:"name"`
	Status            string `json:"status"`
	LastError         string `json:"lastError,omitempty"`
	ReconnectAttempts int    `json:"reconnectAttempts"`
}

// Details represents an MCP definition with its current status
type Details struct {
	Definition Definition `json:"definition"`
	Status     Status     `json:"status"`
}

// listMCPDetailsResponse is the admin API response shape for /admin/mcps
type listMCPDetailsResponse struct {
	MCPs  []Details `json:"mcps"`
	Count int       `json:"count"`
}

// parseProxyError attempts to decode a structured error from proxy responses.
// It accepts bodies like: {"error":"...","details":"...","code":"..."}
// and returns a ProxyRequestError with populated Code and Message.
func parseProxyError(status int, body []byte) error {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err == nil {
		msg := ""
		if m, ok := data["message"].(string); ok && m != "" {
			msg = m
		} else if e, ok := data["error"].(string); ok && e != "" {
			msg = e
		} else {
			msg = string(body)
		}
		code := ""
		if c, ok := data["code"].(string); ok {
			code = c
		}
		return &ProxyRequestError{StatusCode: status, Message: msg, Code: code}
	}
	return &ProxyRequestError{StatusCode: status, Message: string(body)}
}

// ListMCPDetails returns definitions with live status from the proxy admin API
func (c *Client) ListMCPDetails(ctx context.Context) ([]Details, error) {
	var details []Details
	err := c.withRetry(ctx, "list MCP details", func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/admin/mcps", http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create list request: %w", err)
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
			return &ProxyRequestError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("tools request failed: %s", string(body)),
			}
		}
		var response listMCPDetailsResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
		details = response.MCPs
		return nil
	})
	if err != nil {
		return nil, err
	}
	return details, nil
}

// WaitForConnections blocks until all specified MCPs report status=connected.
// buildNameSet creates a set from the list of names
func buildNameSet(names []string) map[string]struct{} {
	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		if n != "" {
			nameSet[n] = struct{}{}
		}
	}
	return nameSet
}

// formatConnectionErrors formats the last errors into a readable message
func formatConnectionErrors(lastErrors map[string]string) string {
	var b strings.Builder
	b.WriteString("MCP connection wait canceled/expired; statuses:")
	for n, msg := range lastErrors {
		b.WriteString(" ")
		b.WriteString(n)
		b.WriteString("=")
		if msg == "" {
			b.WriteString("pending")
		} else {
			b.WriteString(msg)
		}
		b.WriteString(";")
	}
	return b.String()
}

// checkConnections checks the connection status of MCPs and updates error tracking
func (c *Client) checkConnections(
	ctx context.Context,
	nameSet map[string]struct{},
	lastErrors map[string]string,
) (int, error) {
	details, err := c.ListMCPDetails(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch MCP statuses: %w", err)
	}
	connected := 0
	for i := range details {
		d := &details[i]
		if _, ok := nameSet[d.Definition.Name]; !ok {
			continue
		}
		switch strings.ToLower(d.Status.Status) {
		case "connected":
			connected++
			delete(lastErrors, d.Definition.Name)
		case "error":
			lastErrors[d.Definition.Name] = d.Status.LastError
		default:
			lastErrors[d.Definition.Name] = "" // pending/connecting
		}
	}
	return connected, nil
}

// Returns detailed error when any fail or the timeout expires.
func (c *Client) WaitForConnections(ctx context.Context, names []string, pollInterval time.Duration) error {
	if len(names) == 0 {
		return nil
	}
	nameSet := buildNameSet(names)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	// Track last observed errors for clearer reporting
	lastErrors := make(map[string]string)
	for {
		select {
		case <-ctx.Done():
			if len(lastErrors) == 0 {
				return ctx.Err()
			}
			// Include last seen connection errors in timeout/error message
			return errors.New(formatConnectionErrors(lastErrors))
		case <-ticker.C:
			connected, err := c.checkConnections(ctx, nameSet, lastErrors)
			if err != nil {
				return err
			}
			if connected >= len(nameSet) {
				return nil
			}
		}
	}
}

// ListTools retrieves all available tools from registered MCPs via the proxy
func (c *Client) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	var tools []ToolDefinition
	err := c.withRetry(ctx, "list tools", func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/admin/tools", http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create tools request: %w", err)
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
			return &ProxyRequestError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("tools request failed: %s", string(body)),
			}
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

// ToolCallRequest represents a request to call a tool
type ToolCallRequest struct {
	MCPName   string         `json:"mcpName"`
	ToolName  string         `json:"toolName"`
	Arguments map[string]any `json:"arguments"`
}

// ToolCallResponse represents the response from a tool call
type ToolCallResponse struct {
	Result any    `json:"result"`
	Error  string `json:"error,omitempty"`
}

// CallTool executes a tool via the proxy
func (c *Client) CallTool(ctx context.Context, mcpName, toolName string, arguments map[string]any) (any, error) {
	var response ToolCallResponse
	err := c.withRetry(ctx, "call tool", func() error {
		payload, err := json.Marshal(ToolCallRequest{
			MCPName:   mcpName,
			ToolName:  toolName,
			Arguments: arguments,
		})
		if err != nil {
			return fmt.Errorf("failed to marshal tool call request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/admin/tools/call", bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("failed to create tool call request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("tool call request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return &ProxyRequestError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("tool call failed (status %d): %s", resp.StatusCode, string(body)),
			}
		}

		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("failed to unmarshal tool response: %w", err)
		}

		if response.Error != "" {
			return fmt.Errorf("tool execution error: %s", response.Error)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	// Normalize proxy-wrapped content: when result is a typed text payload
	// like {"type":"text","text":"..."}, unwrap to plain string so
	// downstream success/error heuristics can reason over the textual content.
	if m, ok := response.Result.(map[string]any); ok {
		if t, ok := m["type"].(string); ok && t == "text" {
			if txt, ok := m["text"].(string); ok {
				return txt, nil
			}
		}
	}
	return response.Result, nil
}

// Close cleans up the HTTP client resources
func (c *Client) Close() error {
	// Close idle connections
	c.http.CloseIdleConnections()
	return nil
}

// withRetry executes the provided function with exponential backoff retry logic
func (c *Client) withRetry(ctx context.Context, operation string, fn func() error) error {
	log := logger.FromContext(ctx)
	return retry.Do(
		ctx,
		retry.WithMaxRetries(c.retryConf.MaxAttempts, retry.NewExponential(c.retryConf.BaseDelay)),
		func(_ context.Context) error {
			err := fn()
			if err != nil {
				if !isRetryableError(err) {
					return err
				}
				log.Warn("Proxy operation failed, retrying", "operation", operation, "error", err)
				return retry.RetryableError(err)
			}
			return nil
		},
	)
}

// isRetryableError determines if an error is worth retrying
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for structured proxy errors
	var proxyErr *ProxyRequestError
	if errors.As(err, &proxyErr) {
		// Only retry on server errors (5xx status codes)
		return proxyErr.StatusCode >= 500 && proxyErr.StatusCode < 600
	}

	// Unwrap *url.Error and net.Error for better classification
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if ne, ok := urlErr.Err.(net.Error); ok {
			if ne.Timeout() {
				return true
			}
		}
		// Connection refused and similar syscall errors often wrapped; keep conservative retry
		if strings.Contains(strings.ToLower(urlErr.Err.Error()), "connection refused") {
			return true
		}
		return false
	}

	// Respect context cancellation without retry
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}

	return false
}

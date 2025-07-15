package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// Client represents an HTTP client for workflow API operations
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient creates a new workflow API client
func NewClient(cfg *config.Config, apiKey string) (*Client, error) {
	// Use HTTP for localhost development, HTTPS for production
	scheme := "https"
	if cfg.Server.Host == "localhost" || cfg.Server.Host == "127.0.0.1" {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s:%d/api/v0", scheme, cfg.Server.Host, cfg.Server.Port)

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
func (c *Client) parseResponse(resp *http.Response, result any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp APIResponse
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

// Info represents basic workflow information
type Info struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Detail represents detailed workflow information
type Detail struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Config      map[string]any `json:"config"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

// ExecutionInfo represents workflow execution information
type ExecutionInfo struct {
	ID         string         `json:"id"`
	WorkflowID string         `json:"workflow_id"`
	Status     string         `json:"status"`
	Input      map[string]any `json:"input"`
	Output     map[string]any `json:"output"`
	CreatedAt  string         `json:"created_at"`
	UpdatedAt  string         `json:"updated_at"`
}

// ExecuteWorkflowRequest represents the request to execute a workflow
type ExecuteWorkflowRequest struct {
	Input map[string]any `json:"input,omitempty"`
}

// ExecuteWorkflowResponse represents the response from executing a workflow
type ExecuteWorkflowResponse struct {
	ExecutionID string `json:"execution_id"`
	Status      string `json:"status"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	Data    any            `json:"data,omitempty"`
	Error   string         `json:"error,omitempty"`
	Message string         `json:"message,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// ListWorkflows lists all workflows
func (c *Client) ListWorkflows(ctx context.Context) ([]Info, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/workflows", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Workflows []Info `json:"workflows"`
		} `json:"data"`
		Message string `json:"message"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return result.Data.Workflows, nil
}

// GetWorkflow gets a specific workflow by ID
func (c *Client) GetWorkflow(ctx context.Context, workflowID string) (*Detail, error) {
	path := fmt.Sprintf("/workflows/%s", workflowID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data    Detail `json:"data"`
		Message string `json:"message"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// ExecuteWorkflow executes a workflow
func (c *Client) ExecuteWorkflow(
	ctx context.Context,
	workflowID string,
	input map[string]any,
) (*ExecuteWorkflowResponse, error) {
	path := fmt.Sprintf("/workflows/%s/executions", workflowID)
	req := ExecuteWorkflowRequest{Input: input}

	resp, err := c.doRequest(ctx, http.MethodPost, path, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data    ExecuteWorkflowResponse `json:"data"`
		Message string                  `json:"message"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// ValidateWorkflow validates a workflow configuration
func (c *Client) ValidateWorkflow(ctx context.Context, workflowID string) (*ValidationResult, error) {
	path := fmt.Sprintf("/workflows/%s/validate", workflowID)
	resp, err := c.doRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data    ValidationResult `json:"data"`
		Message string           `json:"message"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// ValidationResult represents workflow validation results
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors"`
	Warnings []ValidationError `json:"warnings"`
}

// ValidationError represents a validation error or warning
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

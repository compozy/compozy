package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

// Client wraps HTTP communication with a Compozy server.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	rawBase    string
}

type envelope[T any] struct {
	Data    T      `json:"data"`
	Message string `json:"message"`
	Error   any    `json:"error"`
}

// DeployProject uploads the provided project configuration and associated
// workflows to the remote server.
func (c *Client) DeployProject(ctx context.Context, proj *project.Config) error {
	if c == nil {
		return fmt.Errorf("client is required")
	}
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if proj == nil {
		return fmt.Errorf("project config is required")
	}
	name := strings.TrimSpace(proj.Name)
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	log := logger.FromContext(ctx)
	cfg := config.FromContext(ctx)
	path := fmt.Sprintf("/project?project=%s", url.QueryEscape(name))
	if err := c.putJSON(ctx, path, proj, nil); err == nil {
		log.Info("project deployed", "project", name)
		return nil
	} else if !isIfMatchRequired(err) {
		return err
	}
	etag, getErr := c.fetchProjectETag(ctx, name)
	if getErr != nil {
		return fmt.Errorf("retrieve project etag: %w", getErr)
	}
	headers := map[string]string{"If-Match": fmt.Sprintf("\"%s\"", etag)}
	if err := c.putJSON(ctx, path, proj, headers); err != nil {
		return err
	}
	if cfg != nil {
		log.Debug("project redeployed with concurrency control", "project", name)
	} else {
		log.Info("project redeployed", "project", name)
	}
	return nil
}

// ExecuteWorkflow triggers a workflow execution and returns the resulting execution metadata.
func (c *Client) ExecuteWorkflow(
	ctx context.Context,
	workflowID string,
	input map[string]any,
) (*ExecutionResult, error) {
	if c == nil {
		return nil, fmt.Errorf("client is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	id := strings.TrimSpace(workflowID)
	if id == "" {
		return nil, fmt.Errorf("workflow id is required")
	}
	log := logger.FromContext(ctx)
	body := map[string]any{"input": input}
	path := fmt.Sprintf("/workflows/%s/executions", url.PathEscape(id))
	var resp envelope[struct {
		ExecURL    string `json:"exec_url"`
		ExecID     string `json:"exec_id"`
		WorkflowID string `json:"workflow_id"`
	}]
	if err := c.doJSON(ctx, http.MethodPost, path, body, nil, &resp); err != nil {
		return nil, err
	}
	log.Info("workflow execution requested", "workflow", id, "execution_id", resp.Data.ExecID)
	return &ExecutionResult{
		ExecutionID: resp.Data.ExecID,
		WorkflowID:  resp.Data.WorkflowID,
		Endpoint:    resp.Data.ExecURL,
	}, nil
}

// GetWorkflowStatus retrieves the state of a workflow execution.
func (c *Client) GetWorkflowStatus(ctx context.Context, executionID string) (*WorkflowStatus, error) {
	if c == nil {
		return nil, fmt.Errorf("client is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	id := strings.TrimSpace(executionID)
	if id == "" {
		return nil, fmt.Errorf("execution id is required")
	}
	log := logger.FromContext(ctx)
	var resp envelope[workflowExecutionPayload]
	path := fmt.Sprintf("/executions/workflows/%s", url.PathEscape(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &resp); err != nil {
		return nil, err
	}
	payload := resp.Data
	status := &WorkflowStatus{
		WorkflowID:  payload.WorkflowID,
		ExecutionID: payload.WorkflowExecID,
		Status:      payload.Status,
		Output:      payload.Output,
		Error:       payload.Error,
	}
	log.Debug("workflow status retrieved", "workflow", status.WorkflowID, "status", status.Status)
	return status, nil
}

func (c *Client) putJSON(
	ctx context.Context,
	path string,
	body any,
	headers map[string]string,
) error {
	var resp envelope[map[string]any]
	return c.doJSON(ctx, http.MethodPut, path, body, headers, &resp)
}

func (c *Client) doJSON(
	ctx context.Context,
	method, path string,
	body any,
	headers map[string]string,
	target any,
) error {
	resp, err := c.do(ctx, method, path, body, headers)
	if err != nil {
		return err
	}
	defer closeBody(ctx, resp.Body)
	if target == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil && err != io.EOF {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) do(
	ctx context.Context,
	method, path string,
	body any,
	headers map[string]string,
) (*http.Response, error) {
	if c == nil {
		return nil, fmt.Errorf("client is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if c.httpClient == nil {
		return nil, fmt.Errorf("http client is not configured")
	}
	fullURL := c.baseURL + ensureLeadingSlash(path)
	reqBody, err := prepareBody(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform request: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		defer closeBody(ctx, resp.Body)
		return nil, c.errorFromResponse(ctx, resp)
	}
	return resp, nil
}

func (c *Client) errorFromResponse(ctx context.Context, resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("request failed with unknown error")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("request failed with status %d (body unreadable): %w", resp.StatusCode, err)
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	var apiErr struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if err := json.Unmarshal(body, &apiErr); err == nil {
		text := strings.TrimSpace(apiErr.Error)
		if text == "" {
			text = strings.TrimSpace(apiErr.Message)
		}
		if text == "" {
			text = strings.TrimSpace(apiErr.Detail)
		}
		if text != "" {
			msg = text
		}
	}
	logger.FromContext(ctx).
		Warn("request failed", "status", resp.StatusCode, "message", msg, "url", resp.Request.URL.String())
	return fmt.Errorf("server returned %d: %s", resp.StatusCode, msg)
}

func (c *Client) fetchProjectETag(ctx context.Context, projectName string) (string, error) {
	path := fmt.Sprintf("/project?project=%s", url.QueryEscape(projectName))
	resp, err := c.do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return "", err
	}
	defer closeBody(ctx, resp.Body)
	etag := strings.TrimSpace(resp.Header.Get("ETag"))
	if etag == "" {
		return "", fmt.Errorf("project etag missing from response")
	}
	return strings.Trim(etag, "\""), nil
}

func prepareBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}
	return bytes.NewReader(data), nil
}

func closeBody(ctx context.Context, body io.ReadCloser) {
	if body == nil {
		return
	}
	if err := body.Close(); err != nil {
		logger.FromContext(ctx).Debug("failed to close response body", "error", err)
	}
}

func ensureLeadingSlash(path string) string {
	if path == "" || path[0] == '/' {
		return path
	}
	return "/" + path
}

func isIfMatchRequired(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "precondition") || strings.Contains(err.Error(), "If-Match")
}

type workflowExecutionPayload struct {
	WorkflowID     string          `json:"workflow_id"`
	WorkflowExecID string          `json:"workflow_exec_id"`
	Status         core.StatusType `json:"status"`
	Output         *core.Output    `json:"output"`
	Error          *core.Error     `json:"error"`
}

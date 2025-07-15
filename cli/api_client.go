package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/cli/services"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/go-resty/resty/v2"
)

// APIClient provides unified access to all Compozy API services
type APIClient struct {
	client      *resty.Client
	config      *config.Config
	baseURL     string
	apiKey      string
	rateLimiter *RateLimiter

	// Service interfaces with segregation
	workflow        services.WorkflowService
	workflowMutate  services.WorkflowMutateService
	execution       services.ExecutionService
	executionMutate services.ExecutionMutateService
	schedule        services.ScheduleService
	scheduleMutate  services.ScheduleMutateService
	event           services.EventService
}

// NewAPIClient creates a new unified API client
func NewAPIClient(cfg *config.Config) (*APIClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	baseURL, apiKey, err := buildAPIConfig(cfg)
	if err != nil {
		return nil, err
	}

	client := buildHTTPClient(cfg, baseURL, apiKey)

	rateLimiter := buildRateLimiter(cfg, client)

	apiClient := &APIClient{
		client:      client,
		config:      cfg,
		baseURL:     baseURL,
		apiKey:      apiKey,
		rateLimiter: rateLimiter,
	}

	initializeServices(apiClient)

	return apiClient, nil
}

// buildAPIConfig extracts and validates API configuration
func buildAPIConfig(cfg *config.Config) (string, string, error) {
	// Use HTTP for localhost development, HTTPS for production
	scheme := "https"
	if cfg.Server.Host == "localhost" || cfg.Server.Host == "127.0.0.1" {
		scheme = "http"
	}
	baseURL := fmt.Sprintf("%s://%s:%d/api/v0", scheme, cfg.Server.Host, cfg.Server.Port)

	// Parse and validate base URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid base URL: %w", err)
	}

	// Validate that URL is absolute with proper scheme and host
	if !parsedURL.IsAbs() {
		return "", "", fmt.Errorf("base URL must be absolute, got: %s", baseURL)
	}
	if parsedURL.Scheme == "" {
		return "", "", fmt.Errorf("base URL must have a scheme (http/https), got: %s", baseURL)
	}
	if parsedURL.Host == "" {
		return "", "", fmt.Errorf("base URL must have a host, got: %s", baseURL)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", "", fmt.Errorf("base URL scheme must be http or https, got: %s", parsedURL.Scheme)
	}

	// Get API key from CLI config
	apiKey := string(cfg.CLI.APIKey)
	if apiKey == "" {
		return "", "", fmt.Errorf(
			"API key is required (set CLI.APIKey in config or COMPOZY_API_KEY environment variable)",
		)
	}

	return baseURL, apiKey, nil
}

// buildHTTPClient creates and configures the HTTP client
func buildHTTPClient(cfg *config.Config, baseURL, apiKey string) *resty.Client {
	client := resty.New().
		SetBaseURL(baseURL).
		SetTimeout(cfg.CLI.Timeout).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+apiKey).
		SetRetryCount(3).
		SetRetryWaitTime(100 * time.Millisecond).
		SetRetryMaxWaitTime(2 * time.Second)

	client.AddRetryCondition(retryCondition)

	// Add request/response logging in debug mode
	if cfg.Runtime.LogLevel == "debug" {
		client.SetDebug(true)
	}

	return client
}

// retryCondition determines if a request should be retried
func retryCondition(r *resty.Response, err error) bool {
	// Retry on network errors
	if err != nil {
		return true
	}
	// Check response is not nil before accessing StatusCode
	if r == nil {
		return false
	}
	// Retry on server errors (5xx) and specific client errors
	code := r.StatusCode()
	return code >= 500 || code == 429 || code == 408 || code == 502 || code == 503 || code == 504
}

// buildRateLimiter creates rate limiter if configured
func buildRateLimiter(cfg *config.Config, client *resty.Client) *RateLimiter {
	if cfg.RateLimit.GlobalRate.Limit <= 0 {
		return nil
	}

	rateLimiter := NewRateLimiter(cfg.RateLimit.GlobalRate.Limit, cfg.RateLimit.GlobalRate.Period)
	if rateLimiter != nil {
		client.OnBeforeRequest(rateLimitMiddleware(rateLimiter))
	}
	return rateLimiter
}

// initializeServices sets up all service implementations
func initializeServices(apiClient *APIClient) {
	apiClient.workflow = &workflowService{client: apiClient}
	apiClient.workflowMutate = &workflowMutateService{client: apiClient}
	apiClient.execution = &executionService{client: apiClient}
	apiClient.executionMutate = &executionMutateService{client: apiClient}
	apiClient.schedule = &scheduleService{client: apiClient}
	apiClient.scheduleMutate = &scheduleMutateService{client: apiClient}
	apiClient.event = &eventService{client: apiClient}
}

// Service accessors with interface segregation
func (c *APIClient) Workflow() services.WorkflowService {
	return c.workflow
}

func (c *APIClient) WorkflowMutate() services.WorkflowMutateService {
	return c.workflowMutate
}

func (c *APIClient) Execution() services.ExecutionService {
	return c.execution
}

func (c *APIClient) ExecutionMutate() services.ExecutionMutateService {
	return c.executionMutate
}

func (c *APIClient) Schedule() services.ScheduleService {
	return c.schedule
}

func (c *APIClient) ScheduleMutate() services.ScheduleMutateService {
	return c.scheduleMutate
}

func (c *APIClient) Event() services.EventService {
	return c.event
}

// doRequest performs a request with context cancellation support
func (c *APIClient) doRequest(ctx context.Context, method, path string, body any, result any) error {
	log := logger.FromContext(ctx)

	req := prepareRequest(ctx, c.client, body, result)

	resp, err := executeRequest(req, method, path)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if err := handleResponse(resp); err != nil {
		return err
	}

	log.Debug("API request completed", "method", method, "path", path, "status", resp.StatusCode())
	return nil
}

// prepareRequest sets up the request with body and result
func prepareRequest(ctx context.Context, client *resty.Client, body, result any) *resty.Request {
	req := client.R().SetContext(ctx)

	if body != nil {
		req.SetBody(body)
	}
	if result != nil {
		req.SetResult(result)
	}

	req.SetError(&APIError{})
	return req
}

// executeRequest performs the HTTP request
func executeRequest(req *resty.Request, method, path string) (*resty.Response, error) {
	switch method {
	case "GET":
		return req.Get(path)
	case "POST":
		return req.Post(path)
	case "PUT":
		return req.Put(path)
	case "PATCH":
		return req.Patch(path)
	case "DELETE":
		return req.Delete(path)
	default:
		return nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}
}

// handleResponse processes the response and handles errors
func handleResponse(resp *resty.Response) error {
	if resp.StatusCode() < 400 {
		return nil
	}

	if apiErr, ok := resp.Error().(*APIError); ok && apiErr != nil {
		return apiErr
	}
	return fmt.Errorf("API error: %s (status %d)", resp.String(), resp.StatusCode())
}

// APIError represents an API error response
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Service implementations

// workflowService implements WorkflowService
type workflowService struct {
	client *APIClient
}

func (s *workflowService) List(ctx context.Context, filters services.WorkflowFilters) ([]services.Workflow, error) {
	var result struct {
		Data struct {
			Workflows []services.Workflow `json:"workflows"`
		} `json:"data"`
	}

	req := s.client.client.R().SetContext(ctx).SetResult(&result)

	// Add query parameters from filters
	if filters.Status != "" {
		req.SetQueryParam("status", filters.Status)
	}
	if len(filters.Tags) > 0 {
		req.SetQueryParam("tags", strings.Join(filters.Tags, ","))
	}
	if filters.Limit > 0 {
		req.SetQueryParam("limit", fmt.Sprintf("%d", filters.Limit))
	}
	if filters.Offset > 0 {
		req.SetQueryParam("offset", fmt.Sprintf("%d", filters.Offset))
	}

	_, err := req.Get("/workflows")
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}

	return result.Data.Workflows, nil
}

func (s *workflowService) Get(ctx context.Context, id core.ID) (*services.WorkflowDetail, error) {
	var result struct {
		Data services.WorkflowDetail `json:"data"`
	}

	err := s.client.doRequest(ctx, "GET", fmt.Sprintf("/workflows/%s", id), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	return &result.Data, nil
}

// workflowMutateService implements WorkflowMutateService
type workflowMutateService struct {
	client *APIClient
}

func (s *workflowMutateService) Execute(
	ctx context.Context,
	id core.ID,
	input services.ExecutionInput,
) (*services.ExecutionResult, error) {
	var result struct {
		Data services.ExecutionResult `json:"data"`
	}

	err := s.client.doRequest(ctx, "POST", fmt.Sprintf("/workflows/%s/execute", id), input, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to execute workflow: %w", err)
	}

	return &result.Data, nil
}

// executionService implements ExecutionService
type executionService struct {
	client *APIClient
}

func (s *executionService) List(ctx context.Context, filters services.ExecutionFilters) ([]services.Execution, error) {
	var result struct {
		Data struct {
			Executions []services.Execution `json:"executions"`
		} `json:"data"`
	}

	req := s.client.client.R().SetContext(ctx).SetResult(&result)

	// Add query parameters from filters
	if filters.WorkflowID != "" {
		req.SetQueryParam("workflow_id", string(filters.WorkflowID))
	}
	if filters.Status != "" {
		req.SetQueryParam("status", string(filters.Status))
	}
	if filters.Limit > 0 {
		req.SetQueryParam("limit", fmt.Sprintf("%d", filters.Limit))
	}
	if filters.Offset > 0 {
		req.SetQueryParam("offset", fmt.Sprintf("%d", filters.Offset))
	}

	_, err := req.Get("/executions")
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}

	return result.Data.Executions, nil
}

func (s *executionService) Get(ctx context.Context, id core.ID) (*services.ExecutionDetail, error) {
	var result struct {
		Data services.ExecutionDetail `json:"data"`
	}

	err := s.client.doRequest(ctx, "GET", fmt.Sprintf("/executions/%s", id), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}

	return &result.Data, nil
}

func (s *executionService) Follow(ctx context.Context, id core.ID) (<-chan services.ExecutionEvent, error) {
	ch := make(chan services.ExecutionEvent, 100)

	go func() {
		defer close(ch)
		if err := s.followSSE(ctx, id, ch); err != nil {
			log := logger.FromContext(ctx)
			log.Error("SSE connection failed", "error", err, "execution_id", id)
		}
	}()

	return ch, nil
}

func (s *executionService) followSSE(ctx context.Context, id core.ID, ch chan<- services.ExecutionEvent) error {
	url := fmt.Sprintf("/executions/%s/follow", id)

	req, err := http.NewRequestWithContext(ctx, "GET", s.client.baseURL+url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Add authentication if available
	if s.client.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.client.apiKey)
	}

	resp, err := s.client.client.GetClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to establish SSE connection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE connection failed with status %d", resp.StatusCode)
	}

	return s.parseSSEStream(ctx, resp.Body, ch)
}

func (s *executionService) parseSSEStream(
	ctx context.Context,
	body io.ReadCloser,
	ch chan<- services.ExecutionEvent,
) error {
	scanner := bufio.NewScanner(body)
	var eventData strings.Builder

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		if line == "" {
			// Empty line indicates end of event
			if eventData.Len() > 0 {
				if err := s.processSSEEvent(eventData.String(), ch); err != nil {
					log := logger.FromContext(ctx)
					log.Error("failed to process SSE event", "error", err)
				}
				eventData.Reset()
			}
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			eventData.WriteString(strings.TrimPrefix(line, "data: "))
		}
	}

	return scanner.Err()
}

func (s *executionService) processSSEEvent(data string, ch chan<- services.ExecutionEvent) error {
	var event services.ExecutionEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return fmt.Errorf("failed to unmarshal SSE event: %w", err)
	}

	select {
	case ch <- event:
		return nil
	default:
		// Channel is full, skip event
		return nil
	}
}

// executionMutateService implements ExecutionMutateService
type executionMutateService struct {
	client *APIClient
}

func (s *executionMutateService) Signal(ctx context.Context, execID core.ID, signal string, payload any) error {
	req := struct {
		Signal  string `json:"signal"`
		Payload any    `json:"payload,omitempty"`
	}{
		Signal:  signal,
		Payload: payload,
	}

	err := s.client.doRequest(ctx, "POST", fmt.Sprintf("/executions/%s/signal", execID), req, nil)
	if err != nil {
		return fmt.Errorf("failed to send signal: %w", err)
	}

	return nil
}

func (s *executionMutateService) Cancel(ctx context.Context, execID core.ID) error {
	err := s.client.doRequest(ctx, "POST", fmt.Sprintf("/executions/%s/cancel", execID), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to cancel execution: %w", err)
	}

	return nil
}

// scheduleService implements ScheduleService
type scheduleService struct {
	client *APIClient
}

func (s *scheduleService) List(ctx context.Context) ([]services.Schedule, error) {
	var result struct {
		Data struct {
			Schedules []services.Schedule `json:"schedules"`
		} `json:"data"`
	}

	err := s.client.doRequest(ctx, "GET", "/schedules", nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedules: %w", err)
	}

	return result.Data.Schedules, nil
}

func (s *scheduleService) Get(ctx context.Context, workflowID core.ID) (*services.Schedule, error) {
	var result struct {
		Data services.Schedule `json:"data"`
	}

	err := s.client.doRequest(ctx, "GET", fmt.Sprintf("/schedules/%s", workflowID), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	return &result.Data, nil
}

// scheduleMutateService implements ScheduleMutateService
type scheduleMutateService struct {
	client *APIClient
}

func (s *scheduleMutateService) Update(
	ctx context.Context,
	workflowID core.ID,
	req services.UpdateScheduleRequest,
) error {
	err := s.client.doRequest(ctx, "PUT", fmt.Sprintf("/schedules/%s", workflowID), req, nil)
	if err != nil {
		return fmt.Errorf("failed to update schedule: %w", err)
	}

	return nil
}

func (s *scheduleMutateService) Delete(ctx context.Context, workflowID core.ID) error {
	err := s.client.doRequest(ctx, "DELETE", fmt.Sprintf("/schedules/%s", workflowID), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	return nil
}

// eventService implements EventService
type eventService struct {
	client *APIClient
}

func (s *eventService) Send(ctx context.Context, event services.Event) error {
	err := s.client.doRequest(ctx, "POST", "/events", event, nil)
	if err != nil {
		return fmt.Errorf("failed to send event: %w", err)
	}

	return nil
}

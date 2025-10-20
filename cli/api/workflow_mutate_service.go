package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/go-resty/resty/v2"
)

// NewWorkflowMutateService builds a WorkflowMutateService backed by the authenticated HTTP client.
func NewWorkflowMutateService(authClient AuthClient, httpClient *resty.Client) WorkflowMutateService {
	if authClient == nil {
		panic("authClient is required to build workflow mutate service")
	}
	if httpClient == nil {
		panic("httpClient is required to build workflow mutate service")
	}
	return &workflowMutateAPIService{
		authClient:     authClient,
		httpClient:     httpClient,
		isNetworkError: defaultIsNetworkError,
		isTimeoutError: defaultIsTimeoutError,
	}
}

type workflowMutateAPIService struct {
	authClient     AuthClient
	httpClient     *resty.Client
	isNetworkError func(error) bool
	isTimeoutError func(error) bool
}

func (s *workflowMutateAPIService) Execute(
	ctx context.Context,
	id core.ID,
	input ExecutionInput,
) (*ExecutionResult, error) {
	log := logger.FromContext(ctx)
	result, err := s.requestWorkflowExecution(ctx, id, input)
	if err != nil {
		return nil, err
	}
	log.Debug("workflow execution requested", "workflow_id", id, "execution_id", result.ExecutionID)
	return result, nil
}

func (s *workflowMutateAPIService) requestWorkflowExecution(
	ctx context.Context,
	id core.ID,
	input ExecutionInput,
) (*ExecutionResult, error) {
	var response struct {
		Data ExecutionResult `json:"data"`
	}
	resp, err := s.httpClient.R().
		SetContext(ctx).
		SetBody(map[string]any{"input": input}).
		SetResult(&response).
		Post(fmt.Sprintf("/workflows/%s/executions", id))
	if err != nil {
		return nil, s.transformWorkflowRequestError(err)
	}
	if err := s.validateExecutionResponse(resp, id); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (s *workflowMutateAPIService) transformWorkflowRequestError(err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("request canceled by user")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("request timed out: context deadline exceeded")
	}
	if s.isNetworkError(err) {
		return fmt.Errorf("network error: unable to connect to Compozy server: %w", err)
	}
	if s.isTimeoutError(err) {
		return fmt.Errorf("request timed out: server may be busy: %w", err)
	}
	return fmt.Errorf("failed to execute workflow: %w", err)
}

func (s *workflowMutateAPIService) validateExecutionResponse(resp *resty.Response, id core.ID) error {
	if resp.StatusCode() < http.StatusBadRequest {
		return nil
	}
	switch resp.StatusCode() {
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed: please check your API key or login credentials")
	case http.StatusForbidden:
		return fmt.Errorf("permission denied: you don't have access to execute this workflow")
	case http.StatusNotFound:
		return fmt.Errorf("workflow not found: workflow with ID %s does not exist", id)
	case http.StatusBadRequest:
		return s.apiErrorWithPrefix(resp, http.StatusBadRequest, "invalid request")
	case http.StatusConflict:
		return s.apiErrorWithPrefix(resp, http.StatusConflict, "conflict")
	case http.StatusUnprocessableEntity:
		return s.apiErrorWithPrefix(resp, http.StatusUnprocessableEntity, "unprocessable entity")
	case http.StatusTooManyRequests:
		if ra := strings.TrimSpace(resp.Header().Get("Retry-After")); ra != "" {
			return fmt.Errorf("rate limit exceeded: retry after %s", ra)
		}
		return fmt.Errorf("rate limit exceeded: please retry later")
	default:
		if resp.StatusCode() >= http.StatusInternalServerError {
			return fmt.Errorf("server error (status %d): try again later", resp.StatusCode())
		}
		if msg := parseAPIError(resp); msg != "" {
			return fmt.Errorf("API error: %s (status %d)", msg, resp.StatusCode())
		}
		return fmt.Errorf("API error (status %d)", resp.StatusCode())
	}
}

func (s *workflowMutateAPIService) apiErrorWithPrefix(resp *resty.Response, code int, prefix string) error {
	if msg := parseAPIError(resp); msg != "" {
		return fmt.Errorf("%s: %s (status %d)", prefix, msg, code)
	}
	return fmt.Errorf("%s (status %d)", prefix, code)
}

func parseAPIError(resp *resty.Response) string {
	if resp == nil {
		return ""
	}
	body := resp.Body()
	if len(body) == 0 {
		return ""
	}
	var envelope struct {
		Error   string          `json:"error"`
		Message string          `json:"message"`
		Details json.RawMessage `json:"details"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return ""
	}
	message := strings.TrimSpace(envelope.Error)
	if message == "" {
		message = strings.TrimSpace(envelope.Message)
	}
	if message == "" {
		return ""
	}
	if len(envelope.Details) == 0 || string(envelope.Details) == "null" {
		return message
	}
	var details string
	if err := json.Unmarshal(envelope.Details, &details); err == nil && strings.TrimSpace(details) != "" {
		return fmt.Sprintf("%s: %s", message, strings.TrimSpace(details))
	}
	return message
}

func defaultIsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return errors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "timed out")
}

func defaultIsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	networkKeywords := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"no route to host",
		"network unreachable",
		"dns",
		"name resolution failed",
		"temporary failure",
	}
	for _, keyword := range networkKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	ErrCodeLLMCreation         = "LLM_CREATION_ERROR"
	ErrCodeLLMGeneration       = "LLM_GENERATION_ERROR"
	ErrCodeInvalidResponse     = "INVALID_LLM_RESPONSE"
	ErrCodePromptBuild         = "PROMPT_BUILD_ERROR"
	ErrCodeToolDefinitions     = "TOOL_DEFINITIONS_ERROR"
	ErrCodeInvalidConversation = "INVALID_CONVERSATION"

	ErrCodeToolNotFound      = "TOOL_NOT_FOUND"
	ErrCodeToolExecution     = "TOOL_EXECUTION_ERROR"
	ErrCodeToolInvalidInput  = "TOOL_INVALID_INPUT"
	ErrCodeToolInvalidOutput = "TOOL_INVALID_OUTPUT"

	ErrCodeMCPConnection = "MCP_CONNECTION_ERROR"
	ErrCodeMCPTimeout    = "MCP_TIMEOUT_ERROR"
	ErrCodeMCPResponse   = "MCP_RESPONSE_ERROR"

	ErrCodeInputValidation  = "INPUT_VALIDATION_ERROR"
	ErrCodeOutputValidation = "OUTPUT_VALIDATION_ERROR"
	ErrCodeSchemaValidation = "SCHEMA_VALIDATION_ERROR"

	ErrCodeInvalidConfig = "INVALID_CONFIGURATION"
	ErrCodeMissingConfig = "MISSING_CONFIGURATION"

	ErrCodeBudgetExceeded = "BUDGET_EXCEEDED"
)

// ErrNoProgress is returned when the loop detects no progress across iterations.
var ErrNoProgress = errors.New("no progress")

// ErrBudgetExceeded signals that a tool or loop budget threshold has been exceeded.
var ErrBudgetExceeded = errors.New("budget exceeded")

// transientRetryPattern is compiled once and reused to avoid overhead.
var transientRetryPattern = regexp.MustCompile(`(?i)(timeout|temporarily|try again|temporarily unavailable)`)

type ToolExecutionResult struct {
	Success bool           `json:"success"`
	Data    map[string]any `json:"data,omitempty"`
	Error   *ToolError     `json:"error,omitempty"`
}

type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func IsToolExecutionError(result string) (*ToolError, bool) {
	var structured ToolExecutionResult
	if err := json.Unmarshal([]byte(result), &structured); err == nil {
		if !structured.Success && structured.Error != nil {
			return structured.Error, true
		}
		return nil, false
	}
	if containsErrorIndicators(result) {
		return &ToolError{
			Code:    ErrCodeToolExecution,
			Message: "Tool execution failed",
			Details: result,
		}, true
	}
	return nil, false
}

func containsErrorIndicators(text string) bool {
	lower := strings.ToLower(text)
	indicators := []string{
		"error:",
		"failed:",
		"exception:",
		"panic:",
		"fatal:",
		"stderr:",
		"traceback:",
		"stack trace:",
	}
	for _, indicator := range indicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}

func NewLLMError(err error, code string, details map[string]any) error {
	return core.NewError(err, code, details)
}

func NewToolError(err error, code, toolName string, details map[string]any) error {
	if details == nil {
		details = make(map[string]any)
	}
	details["tool"] = toolName
	return core.NewError(err, code, details)
}

func NewValidationError(err error, field string, value any) error {
	return core.NewError(err, ErrCodeInputValidation, map[string]any{
		"field": field,
		"value": value,
	})
}

func WrapMCPError(err error, operation string) error {
	return core.NewError(err, ErrCodeMCPConnection, map[string]any{
		"operation": operation,
	})
}

func isRetryableErrorWithContext(ctx context.Context, err error) bool {
	log := logger.FromContext(ctx)
	retryable := isRetryableError(err)
	fields := []any{
		"error_type", fmt.Sprintf("%T", err),
		"retryable", retryable,
	}
	if llmErr, ok := llmadapter.IsLLMError(err); ok {
		fields = append(fields,
			"llm_error_code", string(llmErr.Code),
			"http_status", llmErr.HTTPStatus,
			"provider", llmErr.Provider,
		)
	}
	if retryable {
		log.Debug("Error is retryable, will retry", fields...)
	} else {
		log.Debug("Error is not retryable", fields...)
	}
	return retryable
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var retryableAdapterErr interface{ Retryable() bool }
	if errors.As(err, &retryableAdapterErr) {
		return retryableAdapterErr.Retryable()
	}
	var coreErr *core.Error
	if errors.As(err, &coreErr) {
		switch coreErr.Code {
		case ErrCodeMCPTimeout, ErrCodeMCPConnection:
			return true
		}
	}
	var llmErr *llmadapter.Error
	if errors.As(err, &llmErr) {
		switch llmErr.Code {
		case llmadapter.ErrCodeRateLimit,
			llmadapter.ErrCodeTimeout,
			llmadapter.ErrCodeServiceUnavailable,
			llmadapter.ErrCodeInternalServer,
			llmadapter.ErrCodeBadGateway,
			llmadapter.ErrCodeGatewayTimeout,
			llmadapter.ErrCodeCapacityError,
			llmadapter.ErrCodeQuotaExceeded:
			return true
		case llmadapter.ErrCodeBadRequest,
			llmadapter.ErrCodeInvalidModel,
			llmadapter.ErrCodeContentPolicy:
			return false
		}
	}
	return transientRetryPattern.MatchString(err.Error())
}

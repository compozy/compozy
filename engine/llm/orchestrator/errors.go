package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/toolerrors"
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

// ToolExecutionResult mirrors the shared toolerrors definition for orchestrator usage.
type ToolExecutionResult = toolerrors.ToolExecutionResult

// ToolError mirrors the shared toolerrors definition for orchestrator usage.
type ToolError = toolerrors.ToolError

func IsToolExecutionError(result string) (*ToolError, bool) {
	return toolerrors.IsToolExecutionError(result, ErrCodeToolExecution)
}

func NewLLMError(err error, code string, details map[string]any) error {
	return core.NewError(err, code, details)
}

func NewToolError(err error, code, toolName string, details map[string]any) error {
	return toolerrors.NewToolError(err, code, toolName, details)
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
		log.Warn("Error is not retryable", fields...)
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

package llm

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/toolerrors"
)

// Error codes for LLM operations
const (
	// LLM interaction errors
	ErrCodeLLMCreation     = "LLM_CREATION_ERROR"
	ErrCodeLLMGeneration   = "LLM_GENERATION_ERROR"
	ErrCodeInvalidResponse = "INVALID_LLM_RESPONSE"

	// Tool execution errors
	ErrCodeToolNotFound      = "TOOL_NOT_FOUND"
	ErrCodeToolExecution     = "TOOL_EXECUTION_ERROR"
	ErrCodeToolInvalidInput  = "TOOL_INVALID_INPUT"
	ErrCodeToolInvalidOutput = "TOOL_INVALID_OUTPUT"

	// MCP proxy errors
	ErrCodeMCPConnection = "MCP_CONNECTION_ERROR"
	ErrCodeMCPTimeout    = "MCP_TIMEOUT_ERROR"
	ErrCodeMCPResponse   = "MCP_RESPONSE_ERROR"

	// Validation errors
	ErrCodeInputValidation  = "INPUT_VALIDATION_ERROR"
	ErrCodeOutputValidation = "OUTPUT_VALIDATION_ERROR"
	ErrCodeSchemaValidation = "SCHEMA_VALIDATION_ERROR"

	// Configuration errors
	ErrCodeInvalidConfig = "INVALID_CONFIGURATION"
	ErrCodeMissingConfig = "MISSING_CONFIGURATION"
)

// ToolExecutionResult represents the result of a tool execution.
type ToolExecutionResult = toolerrors.ToolExecutionResult

// ToolError represents a structured tool execution error.
type ToolError = toolerrors.ToolError

// IsToolExecutionError checks if a tool result indicates an error.
func IsToolExecutionError(result string) (*ToolError, bool) {
	return toolerrors.IsToolExecutionError(result, ErrCodeToolExecution)
}

// NewLLMError creates a new LLM-related error
func NewLLMError(err error, code string, details map[string]any) error {
	return core.NewError(err, code, details)
}

// NewToolError creates a new tool-related error
func NewToolError(err error, code, toolName string, details map[string]any) error {
	return toolerrors.NewToolError(err, code, toolName, details)
}

// NewValidationError creates a new validation error
func NewValidationError(err error, field string, value any) error {
	return core.NewError(err, ErrCodeInputValidation, map[string]any{
		"field": field,
		"value": value,
	})
}

// WrapMCPError wraps an MCP-related error with context
func WrapMCPError(err error, operation string) error {
	return core.NewError(err, ErrCodeMCPConnection, map[string]any{
		"operation": operation,
	})
}

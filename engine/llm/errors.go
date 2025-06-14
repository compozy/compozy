package llm

import (
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/engine/core"
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

// ToolExecutionResult represents the result of a tool execution
type ToolExecutionResult struct {
	Success bool           `json:"success"`
	Data    map[string]any `json:"data,omitempty"`
	Error   *ToolError     `json:"error,omitempty"`
}

// ToolError represents a structured tool execution error
type ToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// IsToolExecutionError checks if a tool result indicates an error
func IsToolExecutionError(result string) (*ToolError, bool) {
	// Try to parse as structured result first
	var structuredResult ToolExecutionResult
	if err := json.Unmarshal([]byte(result), &structuredResult); err == nil {
		if !structuredResult.Success && structuredResult.Error != nil {
			return structuredResult.Error, true
		}
		return nil, false
	}

	// Fallback: check for common error indicators in plain text
	// This is more robust than just checking for "error" substring
	if containsErrorIndicators(result) {
		return &ToolError{
			Code:    ErrCodeToolExecution,
			Message: "Tool execution failed",
			Details: result,
		}, true
	}

	return nil, false
}

// containsErrorIndicators checks for various error indicators in text
func containsErrorIndicators(text string) bool {
	errorIndicators := []string{
		"error:",
		"failed:",
		"exception:",
		"panic:",
		"fatal:",
		"stderr:",
		"traceback:",
		"stack trace:",
	}

	lowerText := strings.ToLower(text)
	for _, indicator := range errorIndicators {
		if strings.Contains(lowerText, indicator) {
			return true
		}
	}

	return false
}

// NewLLMError creates a new LLM-related error
func NewLLMError(err error, code string, details map[string]any) error {
	return core.NewError(err, code, details)
}

// NewToolError creates a new tool-related error
func NewToolError(err error, toolName string, details map[string]any) error {
	if details == nil {
		details = make(map[string]any)
	}
	details["tool"] = toolName

	return core.NewError(err, ErrCodeToolExecution, details)
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

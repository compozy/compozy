package toolerrors

import (
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/engine/core"
)

// ToolExecutionResult represents the result of a tool execution.
type ToolExecutionResult struct {
	Success bool           `json:"success"`
	Data    map[string]any `json:"data,omitempty"`
	Error   *ToolError     `json:"error,omitempty"`
}

// ToolError represents a structured tool execution error.
type ToolError struct {
	Code            string `json:"code"`
	Message         string `json:"message"`
	Details         string `json:"details,omitempty"`
	RemediationHint string `json:"remediation_hint,omitempty"`
}

// IsToolExecutionError checks if a tool result indicates an error.
func IsToolExecutionError(result string, fallbackCode string) (*ToolError, bool) {
	var structured ToolExecutionResult
	if err := json.Unmarshal([]byte(result), &structured); err == nil {
		if !structured.Success && structured.Error != nil {
			return structured.Error, true
		}
		return nil, false
	}
	if containsErrorIndicators(result) {
		return &ToolError{
			Code:    fallbackCode,
			Message: "Tool execution failed",
			Details: result,
		}, true
	}
	return nil, false
}

// NewToolError creates a new tool-related error.
func NewToolError(err error, code, toolName string, details map[string]any) error {
	if details == nil {
		details = make(map[string]any)
	}
	details["tool"] = toolName
	return core.NewError(err, code, details)
}

func containsErrorIndicators(text string) bool {
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
	lower := strings.ToLower(text)
	for _, indicator := range indicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}

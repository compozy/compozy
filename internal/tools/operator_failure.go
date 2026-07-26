package tools

import "strings"

// OperatorFailure carries error detail explicitly authored for operator display.
type OperatorFailure struct {
	Cause    string `json:"cause"`
	Recovery string `json:"recovery"`
}

// NewOperatorToolError builds a tool error with operator-safe cause and recovery text.
func NewOperatorToolError(
	code ErrorCode,
	id ToolID,
	message string,
	err error,
	cause string,
	recovery string,
	reasons ...ReasonCode,
) *ToolError {
	toolErr := NewToolError(code, id, message, err, reasons...)
	toolErr.Operator = &OperatorFailure{
		Cause:    strings.TrimSpace(cause),
		Recovery: strings.TrimSpace(recovery),
	}
	return toolErr
}

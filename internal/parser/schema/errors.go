package schema

import "fmt"

// SchemaError represents a base error type for schema validation
type SchemaError struct {
	Message string
	Code    string
}

func (e *SchemaError) Error() string {
	return e.Message
}

// Error codes
const (
	ErrCodeInvalidWithParams = "INVALID_WITH_PARAMS"
	ErrCodeMissingCWD        = "MISSING_CWD"
)

// Error messages
const (
	ErrMsgInvalidWithParams = "With parameters invalid for %s: %s"
	ErrMsgMissingCWD        = "Current working directory is required for %s"
)

// NewSchemaError creates a new SchemaError with the given code and message
func NewSchemaError(code string, message string) *SchemaError {
	return &SchemaError{
		Code:    code,
		Message: message,
	}
}

// NewSchemaErrorf creates a new SchemaError with the given code and formatted message
func NewSchemaErrorf(code string, format string, args ...any) *SchemaError {
	return &SchemaError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

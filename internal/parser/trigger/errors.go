package trigger

import (
	"fmt"
)

// Error codes
const (
	ErrCodeInvalidType        = "INVALID_TRIGGER_TYPE"
	ErrCodeInvalidInputSchema = "INVALID_INPUT_SCHEMA"
)

// Error messages
const (
	ErrMsgInvalidType        = "Invalid trigger type: %s"
	ErrMsgMissingWebhook     = "Webhook configuration is required for webhook trigger type"
	ErrMsgInvalidInputSchema = "Invalid input schema: %s"
)

// TriggerError represents errors that can occur during trigger configuration
type TriggerError struct {
	Message string
	Code    string
}

func (e *TriggerError) Error() string {
	return e.Message
}

// NewError creates a new TriggerError with the given code and message
func NewError(code, message string) *TriggerError {
	return &TriggerError{
		Code:    code,
		Message: message,
	}
}

// NewErrorf creates a new TriggerError with the given code and formatted message
func NewErrorf(code, format string, args ...any) *TriggerError {
	return &TriggerError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Common error constructors
func NewInvalidTypeError(triggerType string) *TriggerError {
	return NewErrorf(ErrCodeInvalidType, ErrMsgInvalidType, triggerType)
}

func NewMissingWebhookError() *TriggerError {
	return NewError(ErrCodeInvalidType, ErrMsgMissingWebhook)
}

func NewInvalidInputSchemaError(err error) *TriggerError {
	return NewErrorf(ErrCodeInvalidInputSchema, ErrMsgInvalidInputSchema, err.Error())
}

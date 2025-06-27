package uc

import (
	"errors"
	"fmt"
)

// Base error types
var (
	// Configuration errors
	ErrMemoryManagerNotAvailable = errors.New("memory manager not available")
	ErrInvalidConfiguration      = errors.New("invalid memory configuration")

	// Input validation errors
	ErrInvalidMemoryRef   = errors.New("invalid memory reference")
	ErrInvalidKey         = errors.New("invalid memory key")
	ErrInvalidPayload     = errors.New("invalid payload")
	ErrEmptyMessages      = errors.New("messages cannot be empty")
	ErrInvalidMessageRole = errors.New("invalid message role")

	// Operation errors
	ErrMemoryNotFound    = errors.New("memory not found")
	ErrMemoryEmpty       = errors.New("memory is empty")
	ErrOperationFailed   = errors.New("memory operation failed")
	ErrFlushNotSupported = errors.New("flush not supported for this memory type")

	// Resource errors
	ErrResourceExhausted = errors.New("memory resource exhausted")
	ErrQuotaExceeded     = errors.New("memory quota exceeded")
)

// ErrorContext wraps an error with additional context
type ErrorContext struct {
	Err       error
	Operation string
	MemoryRef string
	Key       string
	Details   map[string]any
}

// Error implements the error interface
func (e *ErrorContext) Error() string {
	msg := fmt.Sprintf("%s: %v", e.Operation, e.Err)
	if e.MemoryRef != "" {
		msg += fmt.Sprintf(" (memory_ref=%s", e.MemoryRef)
		if e.Key != "" {
			msg += fmt.Sprintf(", key=%s", e.Key)
		}
		msg += ")"
	}
	return msg
}

// Unwrap returns the wrapped error
func (e *ErrorContext) Unwrap() error {
	return e.Err
}

// IsRetryable checks if the error is retryable
func (e *ErrorContext) IsRetryable() bool {
	return errors.Is(e.Err, ErrResourceExhausted) ||
		errors.Is(e.Err, ErrOperationFailed)
}

// NewErrorContext creates a new error with context
func NewErrorContext(err error, operation, memoryRef, key string) *ErrorContext {
	return &ErrorContext{
		Err:       err,
		Operation: operation,
		MemoryRef: memoryRef,
		Key:       key,
		Details:   make(map[string]any),
	}
}

// WithDetail adds a detail to the error context
func (e *ErrorContext) WithDetail(key string, value any) *ErrorContext {
	e.Details[key] = value
	return e
}

// ValidationError represents a validation error with field information
type ValidationError struct {
	Field   string
	Value   any
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("validation failed for %s='%v': %s", e.Field, e.Value, e.Message)
	}
	return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field string, value any, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

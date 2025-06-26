package store

import (
	"errors"
	"fmt"
)

var (
	ErrMemoryKeyNotFound = errors.New("memory key not found")
	ErrInvalidMessage    = errors.New("invalid message format in store")
)

// Memory operation error types for precise error handling
type LockError struct {
	Operation string
	Key       string
	Err       error
}

func (e *LockError) Error() string {
	return fmt.Sprintf("memory lock error during %s for key %s: %v", e.Operation, e.Key, e.Err)
}

func (e *LockError) Unwrap() error {
	return e.Err
}

func NewLockError(operation, key string, err error) *LockError {
	return &LockError{
		Operation: operation,
		Key:       key,
		Err:       err,
	}
}

type ConfigError struct {
	ResourceID string
	Reason     string
	Err        error
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("memory configuration error for resource %s: %s (%v)", e.ResourceID, e.Reason, e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

func NewConfigError(resourceID, reason string, err error) *ConfigError {
	return &ConfigError{
		ResourceID: resourceID,
		Reason:     reason,
		Err:        err,
	}
}

// Validation error types
type ValidationError struct {
	Field   string
	Value   any
	Reason  string
	Context string
}

func (e *ValidationError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf(
			"validation error in %s: field '%s' with value '%v' %s",
			e.Context,
			e.Field,
			e.Value,
			e.Reason,
		)
	}
	return fmt.Sprintf("validation error: field '%s' with value '%v' %s", e.Field, e.Value, e.Reason)
}

func NewValidationError(field string, value any, reason string) *ValidationError {
	return &ValidationError{
		Field:  field,
		Value:  value,
		Reason: reason,
	}
}

func (e *ValidationError) WithContext(context string) *ValidationError {
	e.Context = context
	return e
}

// TTL/Expiration error types
type TTLError struct {
	Key       string
	Operation string
	Err       error
}

func (e *TTLError) Error() string {
	return fmt.Sprintf("TTL error during %s for key %s: %v", e.Operation, e.Key, e.Err)
}

func (e *TTLError) Unwrap() error {
	return e.Err
}

func NewTTLError(key, operation string, err error) *TTLError {
	return &TTLError{
		Key:       key,
		Operation: operation,
		Err:       err,
	}
}

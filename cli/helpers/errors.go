package helpers

import (
	"errors"
	"fmt"
)

// Define sentinel errors for common error types
var (
	// ErrTimeout represents a timeout error
	ErrTimeout = errors.New("operation timed out")

	// ErrNetwork represents a network error
	ErrNetwork = errors.New("network error")

	// ErrAuth represents an authentication error
	ErrAuth = errors.New("authentication error")
)

// TimeoutError represents a timeout error with additional context
type TimeoutError struct {
	Operation string
	Duration  string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("operation %s timed out after %s", e.Operation, e.Duration)
}

func (e *TimeoutError) Is(target error) bool {
	return target == ErrTimeout
}

// NetworkError represents a network-related error
type NetworkError struct {
	Operation string
	Cause     error
}

func (e *NetworkError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("network error during %s: %v", e.Operation, e.Cause)
	}
	return fmt.Sprintf("network error during %s", e.Operation)
}

func (e *NetworkError) Is(target error) bool {
	return target == ErrNetwork
}

func (e *NetworkError) Unwrap() error {
	return e.Cause
}

// AuthError represents an authentication error
type AuthError struct {
	Reason string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication failed: %s", e.Reason)
}

func (e *AuthError) Is(target error) bool {
	return target == ErrAuth
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(operation, duration string) error {
	return &TimeoutError{
		Operation: operation,
		Duration:  duration,
	}
}

// NewNetworkError creates a new network error
func NewNetworkError(operation string, cause error) error {
	return &NetworkError{
		Operation: operation,
		Cause:     cause,
	}
}

// NewAuthError creates a new authentication error
func NewAuthError(reason string) error {
	return &AuthError{
		Reason: reason,
	}
}

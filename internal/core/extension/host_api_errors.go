package extensions

import (
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/subprocess"
)

const (
	notInitializedCode    = -32003
	notInitializedMessage = "Not initialized"

	shutdownInProgressCode    = -32004
	shutdownInProgressMessage = "Shutdown in progress"
)

// NewMethodNotFoundError creates the standard method-not-found response.
func NewMethodNotFoundError(method string) *subprocess.RequestError {
	return subprocess.NewMethodNotFound(strings.TrimSpace(method))
}

// NewNotInitializedError creates the standard not-initialized response.
func NewNotInitializedError() *subprocess.RequestError {
	return &subprocess.RequestError{
		Code:    notInitializedCode,
		Message: notInitializedMessage,
		Data: map[string]any{
			"allowed_methods": []string{"initialize"},
		},
	}
}

// NewShutdownInProgressError creates the standard shutdown-in-progress response.
func NewShutdownInProgressError(deadline time.Duration) *subprocess.RequestError {
	return &subprocess.RequestError{
		Code:    shutdownInProgressCode,
		Message: shutdownInProgressMessage,
		Data: map[string]any{
			"deadline_ms": durationMilliseconds(deadline),
		},
	}
}

func toRequestError(err error, method string) error {
	if err == nil {
		return nil
	}

	var denied *CapabilityDeniedError
	if errors.As(err, &denied) {
		return denied.RequestError()
	}

	var unknown *UnknownCapabilityTargetError
	if errors.As(err, &unknown) {
		return NewMethodNotFoundError(method)
	}

	return err
}

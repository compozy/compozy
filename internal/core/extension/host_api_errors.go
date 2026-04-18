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

	hostCapabilityTokenInvalidCode    = -32005
	hostCapabilityTokenInvalidMessage = "Host capability token invalid"
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

// NewHostCapabilityTokenInvalidError reports that a daemon-owned Host API call
// cannot proceed because the run-scoped capability token context is absent or
// invalid.
func NewHostCapabilityTokenInvalidError(method string, reason string) *subprocess.RequestError {
	return &subprocess.RequestError{
		Code:    hostCapabilityTokenInvalidCode,
		Message: hostCapabilityTokenInvalidMessage,
		Data: map[string]any{
			"method": strings.TrimSpace(method),
			"reason": strings.TrimSpace(reason),
		},
	}
}

func NewCapabilityDeniedReasonError(
	method string,
	reason string,
	data map[string]any,
) *subprocess.RequestError {
	payload := map[string]any{
		"method": strings.TrimSpace(method),
		"reason": strings.TrimSpace(reason),
	}
	for key, value := range data {
		payload[key] = value
	}
	return &subprocess.RequestError{
		Code:    capabilityDeniedCode,
		Message: capabilityDeniedMessage,
		Data:    payload,
	}
}

func NewPathOutOfScopeError(method string, path string, allowedRoots []string) *subprocess.RequestError {
	return NewCapabilityDeniedReasonError(method, "path_out_of_scope", map[string]any{
		"path":          strings.TrimSpace(path),
		"allowed_roots": allowedRoots,
	})
}

func NewRecursionDepthExceededError(method string, parentRunID string, depth int) *subprocess.RequestError {
	return NewCapabilityDeniedReasonError(method, "recursion_depth_exceeded", map[string]any{
		"parent_run_id": strings.TrimSpace(parentRunID),
		"depth":         depth,
	})
}

func NewCancelledByExtensionError(method string, path string) *subprocess.RequestError {
	return subprocess.NewInternalError(map[string]any{
		"method": strings.TrimSpace(method),
		"path":   strings.TrimSpace(path),
		"reason": "canceled_by_extension",
	})
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

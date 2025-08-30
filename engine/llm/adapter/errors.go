package llmadapter

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrCode represents specific error codes for LLM operations
type ErrCode string

const (
	// Client errors (4xx) - Non-retryable
	ErrCodeBadRequest   ErrCode = "BAD_REQUEST"
	ErrCodeUnauthorized ErrCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrCode = "FORBIDDEN"
	ErrCodeNotFound     ErrCode = "NOT_FOUND"
	ErrCodeRateLimit    ErrCode = "RATE_LIMIT" // 429 is retryable

	// Server errors (5xx) - Retryable
	ErrCodeInternalServer     ErrCode = "INTERNAL_SERVER_ERROR"
	ErrCodeBadGateway         ErrCode = "BAD_GATEWAY"
	ErrCodeServiceUnavailable ErrCode = "SERVICE_UNAVAILABLE"
	ErrCodeGatewayTimeout     ErrCode = "GATEWAY_TIMEOUT"

	// Network errors - Retryable
	ErrCodeTimeout           ErrCode = "TIMEOUT"
	ErrCodeConnectionReset   ErrCode = "CONNECTION_RESET"
	ErrCodeConnectionRefused ErrCode = "CONNECTION_REFUSED"

	// Provider-specific errors
	ErrCodeQuotaExceeded ErrCode = "QUOTA_EXCEEDED"
	ErrCodeCapacityError ErrCode = "CAPACITY_ERROR"
	ErrCodeContentPolicy ErrCode = "CONTENT_POLICY"
	ErrCodeInvalidModel  ErrCode = "INVALID_MODEL"
)

// Error represents a structured error from LLM operations
type Error struct {
	Code       ErrCode
	HTTPStatus int
	Message    string
	Provider   string
	Retryable  bool
	Err        error // Original underlying error
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Provider != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Provider, e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Err
}

// IsRetryable returns whether this error should trigger a retry
func (e *Error) IsRetryable() bool {
	return e.Retryable
}

// NewError creates a new LLM error with appropriate classification
func NewError(httpStatus int, message string, provider string, underlying error) *Error {
	code, retryable := classifyHTTPStatus(httpStatus)
	return &Error{
		Code:       code,
		HTTPStatus: httpStatus,
		Message:    message,
		Provider:   provider,
		Retryable:  retryable,
		Err:        underlying,
	}
}

// NewErrorWithCode creates an error with explicit code
func NewErrorWithCode(code ErrCode, message string, provider string, underlying error) *Error {
	httpStatus := mapCodeToHTTPStatus(code)
	retryable := isRetryableCode(code)
	return &Error{
		Code:       code,
		HTTPStatus: httpStatus,
		Message:    message,
		Provider:   provider,
		Retryable:  retryable,
		Err:        underlying,
	}
}

// classifyHTTPStatus maps HTTP status codes to error codes and retry behavior
func classifyHTTPStatus(status int) (ErrCode, bool) {
	switch status {
	case http.StatusBadRequest:
		return ErrCodeBadRequest, false
	case http.StatusUnauthorized:
		return ErrCodeUnauthorized, false
	case http.StatusForbidden:
		return ErrCodeForbidden, false
	case http.StatusNotFound:
		return ErrCodeNotFound, false
	case http.StatusTooManyRequests:
		return ErrCodeRateLimit, true
	case http.StatusInternalServerError:
		return ErrCodeInternalServer, true
	case http.StatusBadGateway:
		return ErrCodeBadGateway, true
	case http.StatusServiceUnavailable:
		return ErrCodeServiceUnavailable, true
	case http.StatusGatewayTimeout:
		return ErrCodeGatewayTimeout, true
	default:
		if status >= 500 {
			return ErrCodeInternalServer, true
		}
		if status >= 400 {
			return ErrCodeBadRequest, false
		}
		return ErrCodeInternalServer, false
	}
}

// mapCodeToHTTPStatus maps error codes back to HTTP status codes
func mapCodeToHTTPStatus(code ErrCode) int {
	switch code {
	case ErrCodeBadRequest, ErrCodeInvalidModel, ErrCodeContentPolicy:
		return http.StatusBadRequest
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeRateLimit, ErrCodeQuotaExceeded:
		return http.StatusTooManyRequests
	case ErrCodeInternalServer, ErrCodeCapacityError:
		return http.StatusInternalServerError
	case ErrCodeBadGateway:
		return http.StatusBadGateway
	case ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeGatewayTimeout, ErrCodeTimeout:
		return http.StatusGatewayTimeout
	case ErrCodeConnectionReset, ErrCodeConnectionRefused:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// isRetryableCode determines if an error code is retryable
func isRetryableCode(code ErrCode) bool {
	switch code {
	// Retryable errors
	case ErrCodeRateLimit,
		ErrCodeInternalServer,
		ErrCodeBadGateway,
		ErrCodeServiceUnavailable,
		ErrCodeGatewayTimeout,
		ErrCodeTimeout,
		ErrCodeConnectionReset,
		ErrCodeConnectionRefused,
		ErrCodeQuotaExceeded,
		ErrCodeCapacityError:
		return true
	// Non-retryable errors
	case ErrCodeBadRequest,
		ErrCodeUnauthorized,
		ErrCodeForbidden,
		ErrCodeNotFound,
		ErrCodeContentPolicy,
		ErrCodeInvalidModel:
		return false
	default:
		return false
	}
}

// IsLLMError checks if an error is an LLM adapter error
func IsLLMError(err error) (*Error, bool) {
	var llmErr *Error
	if errors.As(err, &llmErr) {
		return llmErr, true
	}
	return nil, false
}

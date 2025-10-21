package router

import (
	"errors"
	"fmt"
	"net/http"
)

// Common sentinel errors
var (
	ErrInternal        = errors.New("internal server error")
	ErrInvalidAddress  = errors.New("invalid address")
	ErrBindError       = errors.New("server bind error")
	ErrRouteConflict   = errors.New("route conflict")
	ErrRouteNotDefined = errors.New("route not defined")
	ErrExecutionFailed = errors.New("execution failed")
	ErrProjectConfig   = errors.New("project configuration error")
)

// Error codes
const (
	ErrInternalCode           = "INTERNAL_ERROR"
	ErrBadRequestCode         = "BAD_REQUEST"
	ErrUnauthorizedCode       = "UNAUTHORIZED"
	ErrForbiddenCode          = "FORBIDDEN"
	ErrNotFoundCode           = "NOT_FOUND"
	ErrConflictCode           = "CONFLICT"
	ErrRequestTimeoutCode     = "REQUEST_TIMEOUT"
	ErrServiceUnavailableCode = "SERVICE_UNAVAILABLE"
)

// Knowledge router specific problem codes.
const (
	KnowledgeErrInvalidInputCode   = "invalid_input"
	KnowledgeErrProjectMissingCode = "project_missing"
	KnowledgeErrIDMissingCode      = "id_missing"
	KnowledgeErrIDMismatchCode     = "id_mismatch"
	KnowledgeErrValidationCode     = "validation_failed"
	KnowledgeErrNotFoundCode       = "knowledge_not_found"
	KnowledgeErrAlreadyExistsCode  = "already_exists"
	KnowledgeErrPreconditionCode   = "precondition_failed"
	ErrSerializationCode           = "serialization_error"
)

// Error messages
const (
	ErrMsgAppStateNotInitialized = "application state not initialized"
	ErrMsgWorkerNotRunning       = "worker is not running; configure Redis or start the worker"
)

// Error represents errors that can occur during server operations
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
	Details string `json:"details"`
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// NewServerError creates a new ServerError
func NewServerError(code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// WrapServerError wraps an existing error with a server error
func WrapServerError(code, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// RequestError represents errors that can occur during request handling
type RequestError struct {
	WorkflowID string
	Reason     string
	StatusCode int
	Err        error
}

func (e *RequestError) Error() string {
	if e.WorkflowID != "" {
		return fmt.Sprintf("Workflow %s failed: %s", e.WorkflowID, e.Reason)
	}
	return e.Reason
}

func (e *RequestError) Unwrap() error {
	return e.Err
}

// NewRequestError creates a new RequestError
func NewRequestError(statusCode int, reason string, err error) *RequestError {
	return &RequestError{
		StatusCode: statusCode,
		Reason:     reason,
		Err:        err,
	}
}

// WorkflowExecutionError creates a new workflow execution error
func WorkflowExecutionError(workflowID, reason string, err error) *RequestError {
	return &RequestError{
		StatusCode: http.StatusInternalServerError,
		WorkflowID: workflowID,
		Reason:     reason,
		Err:        err,
	}
}

// IsRequestError checks if the given error is a RequestError
func IsRequestError(err error) bool {
	var reqErr *RequestError
	return errors.As(err, &reqErr)
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// GetErrorInfo extracts error information for the standardized response
func (e *RequestError) GetErrorInfo() *ErrorInfo {
	var details string
	if e.Err != nil {
		details = e.Err.Error()
	}
	code := ErrInternalCode
	switch e.StatusCode {
	case http.StatusBadRequest:
		code = ErrBadRequestCode
	case http.StatusNotFound:
		code = ErrNotFoundCode
	case http.StatusUnauthorized:
		code = ErrUnauthorizedCode
	case http.StatusForbidden:
		code = ErrForbiddenCode
	case http.StatusConflict:
		code = ErrConflictCode
	case http.StatusRequestTimeout:
		code = ErrRequestTimeoutCode
	}
	return &ErrorInfo{
		Code:    code,
		Message: e.Reason,
		Details: details,
	}
}

// getStatusCode returns the appropriate HTTP status code for an error code
func getStatusCode(code string) int {
	switch code {
	case ErrBadRequestCode:
		return http.StatusBadRequest
	case ErrUnauthorizedCode:
		return http.StatusUnauthorized
	case ErrForbiddenCode:
		return http.StatusForbidden
	case ErrNotFoundCode:
		return http.StatusNotFound
	case ErrConflictCode:
		return http.StatusConflict
	case ErrRequestTimeoutCode:
		return http.StatusRequestTimeout
	default:
		return http.StatusInternalServerError
	}
}

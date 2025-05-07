package server

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

// ServerError represents errors that can occur during server operations
type ServerError struct {
	Message string
	Err     error
}

func (e *ServerError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *ServerError) Unwrap() error {
	return e.Err
}

// NewServerError creates a new ServerError
func NewServerError(err error, message string) *ServerError {
	return &ServerError{
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

// ErrorResponse represents the structure of error responses
type ErrorResponse struct {
	Status     int    `json:"status"`
	Message    string `json:"message"`
	WorkflowID string `json:"workflow_id,omitempty"`
	Details    any    `json:"details,omitempty"`
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
func WorkflowExecutionError(workflowID, reason string) *RequestError {
	return &RequestError{
		StatusCode: http.StatusInternalServerError,
		WorkflowID: workflowID,
		Reason:     reason,
	}
}

// IsRequestError checks if the given error is a RequestError
func IsRequestError(err error) bool {
	var reqErr *RequestError
	return errors.As(err, &reqErr)
}

// ToErrorResponse converts a RequestError to an ErrorResponse
func (e *RequestError) ToErrorResponse() ErrorResponse {
	resp := ErrorResponse{
		Status:  e.StatusCode,
		Message: e.Reason,
	}

	if e.WorkflowID != "" {
		resp.WorkflowID = e.WorkflowID
	}

	return resp
}

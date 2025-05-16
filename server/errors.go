package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
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
	ErrInternalCode     = "INTERNAL_ERROR"
	ErrBadRequestCode   = "BAD_REQUEST"
	ErrUnauthorizedCode = "UNAUTHORIZED"
	ErrForbiddenCode    = "FORBIDDEN"
	ErrNotFoundCode     = "NOT_FOUND"
)

// Error represents errors that can occur during server operations
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
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

// ErrorResponse represents the structure of error responses
type ErrorResponse struct {
	Error Error `json:"error"`
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
		Error: Error{
			Code:    ErrInternalCode,
			Message: e.Reason,
		},
	}

	if e.WorkflowID != "" {
		resp.Error.Code = ErrInternalCode
		resp.Error.Message = e.Reason
	}

	return resp
}

// ErrorHandler is a middleware that handles errors
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			var serverErr *Error

			// Try to convert to ServerError
			if se, ok := err.(*Error); ok {
				serverErr = se
			} else {
				// If not a ServerError, wrap it
				serverErr = WrapServerError(ErrInternalCode, "An unexpected error occurred", err)
			}

			// Log the error
			logger.Error("request failed",
				"error_code", serverErr.Code,
				"error_message", serverErr.Message,
				"error", serverErr.Err,
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"status_code", getStatusCode(serverErr.Code),
			)

			// Set the status code
			c.JSON(getStatusCode(serverErr.Code), ErrorResponse{
				Error: *serverErr,
			})
		}
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
	default:
		return http.StatusInternalServerError
	}
}

// MarshalJSON implements the json.Marshaler interface
func (e *Error) MarshalJSON() ([]byte, error) {
	type Alias Error
	return json.Marshal(&struct {
		*Alias
		Error string `json:"error,omitempty"`
	}{
		Alias: (*Alias)(e),
		Error: e.Err.Error(),
	})
}

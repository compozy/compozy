package project

import (
	"fmt"
)

// Error codes
const (
	ErrCodeFileOpen        = "FILE_OPEN_ERROR"
	ErrCodeDecode          = "DECODE_ERROR"
	ErrCodeMissingPath     = "MISSING_FILE_PATH"
	ErrCodeNoWorkflows     = "NO_WORKFLOWS_DEFINED"
	ErrCodeWorkflowLoad    = "WORKFLOW_LOAD_ERROR"
	ErrCodeInvalidEnv      = "INVALID_ENVIRONMENT"
	ErrCodeInvalidLogLevel = "INVALID_LOG_LEVEL"
	ErrCodeFileClose       = "FILE_CLOSE_ERROR"
)

// Error messages
const (
	ErrMsgFileOpen        = "Failed to open project config file: %s"
	ErrMsgDecode          = "Failed to decode project config: %s"
	ErrMsgMissingPath     = "Missing file path for project"
	ErrMsgNoWorkflows     = "No workflows defined in project"
	ErrMsgWorkflowLoad    = "Failed to load workflow: %s"
	ErrMsgInvalidEnv      = "Invalid environment configuration: %s"
	ErrMsgInvalidLogLevel = "Invalid log level: %s"
	ErrMsgFileClose       = "Failed to close project config file: %s"
)

// ProjectError represents errors that can occur during project configuration
type ProjectError struct {
	Message string
	Code    string
}

func (e *ProjectError) Error() string {
	return e.Message
}

// NewError creates a new ProjectError with the given code and message
func NewError(code, message string) *ProjectError {
	return &ProjectError{
		Code:    code,
		Message: message,
	}
}

// NewErrorf creates a new ProjectError with the given code and formatted message
func NewErrorf(code, format string, args ...any) *ProjectError {
	return &ProjectError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Common error constructors
func NewFileOpenError(err error) *ProjectError {
	return NewErrorf(ErrCodeFileOpen, ErrMsgFileOpen, err.Error())
}

func NewDecodeError(err error) *ProjectError {
	return NewErrorf(ErrCodeDecode, ErrMsgDecode, err.Error())
}

func NewMissingPathError() *ProjectError {
	return NewError(ErrCodeMissingPath, ErrMsgMissingPath)
}

func NewNoWorkflowsError() *ProjectError {
	return NewError(ErrCodeNoWorkflows, ErrMsgNoWorkflows)
}

func NewWorkflowLoadError(err error) *ProjectError {
	return NewErrorf(ErrCodeWorkflowLoad, ErrMsgWorkflowLoad, err.Error())
}

func NewInvalidEnvironmentError(envName string) *ProjectError {
	return NewErrorf(ErrCodeInvalidEnv, ErrMsgInvalidEnv, envName)
}

func NewInvalidLogLevelError(level string) *ProjectError {
	return NewErrorf(ErrCodeInvalidLogLevel, ErrMsgInvalidLogLevel, level)
}

func NewFileCloseError(err error) *ProjectError {
	return NewErrorf(ErrCodeFileClose, ErrMsgFileClose, err.Error())
}

package agent

import (
	"fmt"
)

// Error codes
const (
	ErrCodeFileOpen             = "FILE_OPEN_ERROR"
	ErrCodeDecode               = "DECODE_ERROR"
	ErrCodeMissingPath          = "MISSING_FILE_PATH"
	ErrCodeInvalidPackageRef    = "INVALID_PACKAGE_REF"
	ErrCodeInvalidComponentType = "INVALID_COMPONENT_TYPE"
	ErrCodeMerge                = "MERGE_ERROR"
	ErrCodeFileClose            = "FILE_CLOSE_ERROR"
)

// Error messages
const (
	ErrMsgFileOpen             = "Failed to open agent config file: %s"
	ErrMsgDecode               = "Failed to decode agent config: %s"
	ErrMsgMissingPath          = "Missing file path for agent: %s"
	ErrMsgInvalidPackageRef    = "Invalid package reference: %s"
	ErrMsgInvalidComponentType = "Package reference must be an agent"
	ErrMsgMerge                = "Failed to merge agent configs: %s"
	ErrMsgFileClose            = "Failed to close agent config file: %s"
)

// AgentConfigError represents errors that can occur during agent configuration
type AgentConfigError struct {
	Message string
	Code    string
}

func (e *AgentConfigError) Error() string {
	return e.Message
}

// NewError creates a new AgentConfigError with the given code and message
func NewError(code, message string) *AgentConfigError {
	return &AgentConfigError{
		Code:    code,
		Message: message,
	}
}

// NewErrorf creates a new AgentConfigError with the given code and formatted message
func NewErrorf(code, format string, args ...any) *AgentConfigError {
	return &AgentConfigError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// Common error constructors
func NewFileOpenError(err error) *AgentConfigError {
	return NewErrorf(ErrCodeFileOpen, ErrMsgFileOpen, err.Error())
}

func NewDecodeError(err error) *AgentConfigError {
	return NewErrorf(ErrCodeDecode, ErrMsgDecode, err.Error())
}

func NewMissingPathError(action string) *AgentConfigError {
	return NewErrorf(ErrCodeMissingPath, ErrMsgMissingPath, action)
}

func NewInvalidPackageRefError(err error) *AgentConfigError {
	return NewErrorf(ErrCodeInvalidPackageRef, ErrMsgInvalidPackageRef, err.Error())
}

func NewInvalidComponentTypeError() *AgentConfigError {
	return NewError(ErrCodeInvalidComponentType, ErrMsgInvalidComponentType)
}

func NewMergeError(err error) *AgentConfigError {
	return NewErrorf(ErrCodeMerge, ErrMsgMerge, err.Error())
}

func NewFileCloseError(err error) *AgentConfigError {
	return NewErrorf(ErrCodeFileClose, ErrMsgFileClose, err.Error())
}

// InvalidConfigurationError represents an error when the configuration is invalid
type InvalidConfigurationError struct {
	Message string
}

func (e InvalidConfigurationError) Error() string {
	return e.Message
}

func NewInvalidConfigurationError(message string) error {
	return InvalidConfigurationError{Message: message}
}

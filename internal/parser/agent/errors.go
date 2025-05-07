package agent

import (
	"errors"
	"fmt"
)

// Common sentinel errors
var (
	ErrFileOpen             = errors.New("failed to open agent config file")
	ErrDecode               = errors.New("failed to decode agent config")
	ErrMissingPath          = errors.New("missing file path for agent")
	ErrInvalidPackageRef    = errors.New("invalid package reference")
	ErrInvalidComponentType = errors.New("package reference must be an agent")
	ErrMerge                = errors.New("failed to merge agent configs")
	ErrFileClose            = errors.New("failed to close agent config file")
	ErrMissingIdField       = errors.New("missing ID field")
	ErrUnimplemented        = errors.New("feature not implemented")
	ErrInvalidRef           = errors.New("invalid reference type")
)

// AgentError represents errors that can occur during agent operations
type AgentError struct {
	Code    string
	Message string
	Err     error
}

func (e *AgentError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AgentError) Unwrap() error {
	return e.Err
}

// NewAgentError creates a new AgentError
func NewAgentError(code string, message string) *AgentError {
	return &AgentError{
		Code:    code,
		Message: message,
	}
}

// WrapAgentError wraps an existing error with an agent error
func WrapAgentError(code string, message string, err error) *AgentError {
	return &AgentError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Error constructors
func NewFileOpenError(err error) error {
	return fmt.Errorf("%w: %w", ErrFileOpen, err)
}

func NewDecodeError(err error) error {
	return fmt.Errorf("%w: %w", ErrDecode, err)
}

func NewMissingPathError(action string) error {
	return fmt.Errorf("%w: %s", ErrMissingPath, action)
}

func NewInvalidPackageRefError(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidPackageRef, err)
}

func NewInvalidComponentTypeError() error {
	return ErrInvalidComponentType
}

func NewMergeError(err error) error {
	return fmt.Errorf("%w: %w", ErrMerge, err)
}

func NewFileCloseError(err error) error {
	return fmt.Errorf("%w: %w", ErrFileClose, err)
}

func NewMissingIdFieldError() error {
	return ErrMissingIdField
}

func NewUnimplementedError(feature string) error {
	return fmt.Errorf("%w: %s", ErrUnimplemented, feature)
}

func NewInvalidRefError() error {
	return ErrInvalidRef
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

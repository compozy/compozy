package project

import (
	"errors"
	"fmt"
)

// Common sentinel errors
var (
	ErrFileOpen        = errors.New("failed to open project config file")
	ErrDecode          = errors.New("failed to decode project config")
	ErrMissingPath     = errors.New("missing file path for project")
	ErrNoWorkflows     = errors.New("no workflows defined in project")
	ErrWorkflowLoad    = errors.New("failed to load workflow")
	ErrInvalidEnv      = errors.New("invalid environment configuration")
	ErrInvalidLogLevel = errors.New("invalid log level")
	ErrFileClose       = errors.New("failed to close project config file")
)

// Error constructors
func NewFileOpenError(err error) error {
	return fmt.Errorf("%w: %w", ErrFileOpen, err)
}

func NewDecodeError(err error) error {
	return fmt.Errorf("%w: %w", ErrDecode, err)
}

func NewMissingPathError() error {
	return ErrMissingPath
}

func NewNoWorkflowsError() error {
	return ErrNoWorkflows
}

func NewWorkflowLoadError(err error) error {
	return fmt.Errorf("%w: %w", ErrWorkflowLoad, err)
}

func NewInvalidEnvironmentError(envName string) error {
	return fmt.Errorf("%w: %s", ErrInvalidEnv, envName)
}

func NewInvalidLogLevelError(level string) error {
	return fmt.Errorf("%w: %s", ErrInvalidLogLevel, level)
}

func NewFileCloseError(err error) error {
	return fmt.Errorf("%w: %w", ErrFileClose, err)
}

package task

import (
	"errors"
	"fmt"
)

// Common sentinel errors
var (
	ErrFileOpen            = errors.New("failed to open task config file")
	ErrDecode              = errors.New("failed to decode task config")
	ErrMissingPath         = errors.New("missing file path for task")
	ErrInvalidPackageRef   = errors.New("invalid package reference")
	ErrInvalidType         = errors.New("package reference must be a task, agent, or tool")
	ErrInvalidTaskType     = errors.New("invalid task type")
	ErrInvalidInputSchema  = errors.New("invalid input schema")
	ErrInvalidOutputSchema = errors.New("invalid output schema")
	ErrInvalidDecisionTask = errors.New("decision task must have at least one route")
	ErrMerge               = errors.New("failed to merge task configs")
	ErrFileClose           = errors.New("failed to close task config file")
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

func NewInvalidPackageRefError(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidPackageRef, err)
}

func NewInvalidTypeError() error {
	return ErrInvalidType
}

func NewInvalidTaskTypeError(taskType string) error {
	return fmt.Errorf("%w: %s", ErrInvalidTaskType, taskType)
}

func NewInvalidInputSchemaError(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidInputSchema, err)
}

func NewInvalidOutputSchemaError(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidOutputSchema, err)
}

func NewInvalidDecisionTaskError() error {
	return ErrInvalidDecisionTask
}

func NewMergeError(err error) error {
	return fmt.Errorf("%w: %w", ErrMerge, err)
}

func NewFileCloseError(err error) error {
	return fmt.Errorf("%w: %w", ErrFileClose, err)
}

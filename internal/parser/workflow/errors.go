package workflow

import (
	"errors"
	"fmt"
)

// Common sentinel errors
var (
	ErrFileOpen               = errors.New("failed to open workflow config file")
	ErrDecode                 = errors.New("failed to decode workflow config")
	ErrMissingPath            = errors.New("missing file path for workflow")
	ErrInvalidComponent       = errors.New("invalid component")
	ErrNoAgentsDefined        = errors.New("there's no agents defined in your workflow")
	ErrAgentNotFound          = errors.New("agent not found")
	ErrNoToolsDefined         = errors.New("there's no tools defined in your workflow")
	ErrToolNotFound           = errors.New("tool not found")
	ErrNoTasksDefined         = errors.New("there's no tasks defined in your workflow")
	ErrTaskNotFound           = errors.New("task not found")
	ErrNotImplemented         = errors.New("not implemented yet")
	ErrInvalidRefType         = errors.New("invalid reference type")
	ErrAgentValidationError   = errors.New("agent validation error")
	ErrToolValidationError    = errors.New("tool validation error")
	ErrTaskValidationError    = errors.New("task validation error")
	ErrTriggerValidationError = errors.New("trigger validation error")
	ErrMerge                  = errors.New("failed to merge workflow configs")
	ErrFileClose              = errors.New("failed to close workflow config file")
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

func NewInvalidComponentError(componentType string) error {
	return fmt.Errorf("%w for %s reference", ErrInvalidComponent, componentType)
}

func NewNoAgentsDefinedError() error {
	return ErrNoAgentsDefined
}

func NewAgentNotFoundError(ref string) error {
	return fmt.Errorf("%w with reference: %s", ErrAgentNotFound, ref)
}

func NewNoToolsDefinedError() error {
	return ErrNoToolsDefined
}

func NewToolNotFoundError(ref string) error {
	return fmt.Errorf("%w with reference: %s", ErrToolNotFound, ref)
}

func NewNoTasksDefinedError() error {
	return ErrNoTasksDefined
}

func NewTaskNotFoundError(ref string) error {
	return fmt.Errorf("%w with reference: %s", ErrTaskNotFound, ref)
}

func NewNotImplementedError() error {
	return ErrNotImplemented
}

func NewInvalidRefTypeError(componentType string) error {
	return fmt.Errorf("%w for %s", ErrInvalidRefType, componentType)
}

func NewAgentValidationError(err error) error {
	return fmt.Errorf("%w: %w", ErrAgentValidationError, err)
}

func NewToolValidationError(err error) error {
	return fmt.Errorf("%w: %w", ErrToolValidationError, err)
}

func NewTaskValidationError(err error) error {
	return fmt.Errorf("%w: %w", ErrTaskValidationError, err)
}

func NewTriggerValidationError(err error) error {
	return fmt.Errorf("%w: %w", ErrTriggerValidationError, err)
}

func NewMergeError(err error) error {
	return fmt.Errorf("%w: %w", ErrMerge, err)
}

func NewFileCloseError(err error) error {
	return fmt.Errorf("%w: %w", ErrFileClose, err)
}

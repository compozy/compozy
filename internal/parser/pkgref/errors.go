package pkgref

import (
	"errors"
	"fmt"
)

// Common sentinel errors
var (
	ErrInvalidComponent = errors.New("invalid component")
	ErrInvalidType      = errors.New("invalid type")
	ErrInvalidFormat    = errors.New("invalid package reference format")
	ErrEmptyValue       = errors.New("reference value cannot be empty")
	ErrInvalidFile      = errors.New("invalid file")
	ErrInvalidDep       = errors.New("invalid dependency")
)

// Error constructors
func NewInvalidComponentError(component string, err error) error {
	return fmt.Errorf("%w %q: %w", ErrInvalidComponent, component, err)
}

func NewInvalidTypeError(typeStr string, err error) error {
	return fmt.Errorf("%w %q: %w", ErrInvalidType, typeStr, err)
}

func NewInvalidFormatError(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidFormat, err)
}

func NewEmptyValueError() error {
	return ErrEmptyValue
}

func NewInvalidFileError(path string, err error) error {
	return fmt.Errorf("%w %q: %w", ErrInvalidFile, path, err)
}

func NewInvalidDependencyError(value string, err error) error {
	return fmt.Errorf("%w %q: %w", ErrInvalidDep, value, err)
}

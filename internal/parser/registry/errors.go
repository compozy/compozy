package registry

import (
	"errors"
	"fmt"
)

// Common sentinel errors
var (
	ErrFileOpen         = errors.New("failed to open registry config file")
	ErrDecode           = errors.New("failed to decode registry config")
	ErrMissingPath      = errors.New("missing file path for registry")
	ErrInvalidType      = errors.New("invalid component type")
	ErrMainPathNotFound = errors.New("main path does not exist")
	ErrFileClose        = errors.New("failed to close registry config file")
)

// Common error constructors
func NewFileOpenError(err error) error {
	return fmt.Errorf("%w: %w", ErrFileOpen, err)
}

func NewDecodeError(err error) error {
	return fmt.Errorf("%w: %w", ErrDecode, err)
}

func NewMissingPathError() error {
	return ErrMissingPath
}

func NewInvalidTypeError(componentType string) error {
	return fmt.Errorf("%w: %s", ErrInvalidType, componentType)
}

func NewMainPathNotFoundError(mainPath string) error {
	return fmt.Errorf("%w: %s", ErrMainPathNotFound, mainPath)
}

func NewFileCloseError(err error) error {
	return fmt.Errorf("%w: %w", ErrFileClose, err)
}

package tool

import (
	"errors"
	"fmt"
)

// Common sentinel errors
var (
	ErrFileOpen            = errors.New("failed to open tool config file")
	ErrDecode              = errors.New("failed to decode tool config")
	ErrMissingPath         = errors.New("missing file path for tool")
	ErrInvalidPackageRef   = errors.New("invalid package reference")
	ErrInvalidType         = errors.New("package reference must be a tool")
	ErrInvalidExecutePath  = errors.New("invalid execute path")
	ErrMissingToolID       = errors.New("tool ID is required for TypeScript execution")
	ErrInvalidToolExecute  = errors.New("invalid tool execute path")
	ErrInvalidInputSchema  = errors.New("invalid input schema")
	ErrInvalidOutputSchema = errors.New("invalid output schema")
	ErrMerge               = errors.New("failed to merge tool configs")
	ErrFileClose           = errors.New("failed to close tool config file")
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

func NewInvalidExecutePathError(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidExecutePath, err)
}

func NewMissingToolIDError() error {
	return ErrMissingToolID
}

func NewInvalidToolExecuteError(path string) error {
	return fmt.Errorf("%w: %s", ErrInvalidToolExecute, path)
}

func NewInvalidInputSchemaError(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidInputSchema, err)
}

func NewInvalidOutputSchemaError(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidOutputSchema, err)
}

func NewMergeError(err error) error {
	return fmt.Errorf("%w: %w", ErrMerge, err)
}

func NewFileCloseError(err error) error {
	return fmt.Errorf("%w: %w", ErrFileClose, err)
}

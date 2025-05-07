package schema

import (
	"errors"
	"fmt"
)

// Common sentinel errors
var (
	ErrInvalidWithParams = errors.New("with parameters invalid")
	ErrMissingCWD        = errors.New("current working directory is required")
)

// Error constructors
func NewInvalidWithParamsError(id string, err error) error {
	return fmt.Errorf("%w for %s: %w", ErrInvalidWithParams, id, err)
}

func NewMissingCWDError(id string) error {
	return fmt.Errorf("%w for %s", ErrMissingCWD, id)
}

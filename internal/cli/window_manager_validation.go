package cli

import "fmt"

type windowManagerCLIValidationKind string

const (
	windowManagerCLIValidationRequired     windowManagerCLIValidationKind = "required"
	windowManagerCLIValidationConflicting  windowManagerCLIValidationKind = "conflicting"
	windowManagerCLIValidationUnsupported  windowManagerCLIValidationKind = "unsupported"
	windowManagerCLIValidationInvalidValue windowManagerCLIValidationKind = "invalid_value"
	windowManagerCLIValidationOutOfBounds  windowManagerCLIValidationKind = "out_of_bounds"
)

type windowManagerCLIValidationError struct {
	Kind  windowManagerCLIValidationKind
	Field string
	cause error
}

func (e *windowManagerCLIValidationError) Error() string {
	if e.cause != nil {
		return e.cause.Error()
	}
	return fmt.Sprintf("cli: invalid window-manager %s field %q", e.Kind, e.Field)
}

func (e *windowManagerCLIValidationError) Unwrap() error {
	return e.cause
}

func newWindowManagerCLIValidationError(
	kind windowManagerCLIValidationKind,
	field string,
	cause error,
) error {
	return &windowManagerCLIValidationError{Kind: kind, Field: field, cause: cause}
}

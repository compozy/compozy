package task

import (
	"errors"

	"github.com/compozy/compozy/internal/contracts"
)

// ResultContractValidationError carries bounded, sanitized validator output.
type ResultContractValidationError struct {
	Issues []contracts.ValidationIssue
	Cause  error
}

func (e *ResultContractValidationError) Error() string {
	if e == nil {
		return nilStringValue
	}
	return "task result does not satisfy its contract: " + contracts.BuildRepairPrompt(e.Issues)
}

// DiagnosticCode returns the stable public rejection code.
func (e *ResultContractValidationError) DiagnosticCode() string {
	return ResultContractInvalidCode
}

// Unwrap exposes both the stable task sentinel and an underlying contract error.
func (e *ResultContractValidationError) Unwrap() []error {
	if e == nil {
		return nil
	}
	if e.Cause == nil {
		return []error{ErrValidation}
	}
	return []error{ErrValidation, e.Cause}
}

func asResultContractValidationError(err error) (*ResultContractValidationError, bool) {
	var validationErr *ResultContractValidationError
	ok := errors.As(err, &validationErr)
	return validationErr, ok
}

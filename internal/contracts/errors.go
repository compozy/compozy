package contracts

import (
	"errors"
	"fmt"
)

// ErrorCode is a stable public failure classification.
type ErrorCode string

const (
	CodeExpectInvalid      ErrorCode = "call_expect_invalid"
	CodeContractNotFound   ErrorCode = "contract_not_found"
	CodeContractCompile    ErrorCode = "contract_compile_failed"
	CodeResultOverBudget   ErrorCode = "call_result_over_budget"
	CodeRedactionConflict  ErrorCode = "call_result_redaction_conflict"
	CodeSanitizationReject ErrorCode = "content_sanitization_rejected"
)

// ErrContractNotFound is returned by RegistryStore for an absent digest.
var ErrContractNotFound = errors.New("contract not found")

// FaultKind separates authored contract faults from child-output faults.
type FaultKind string

const (
	FaultContract FaultKind = "contract"
	FaultChild    FaultKind = "child"
)

// Error carries a stable code and fault owner while preserving the cause.
type Error struct {
	Code  ErrorCode
	Fault FaultKind
	Msg   string
	Err   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Msg != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Msg)
	}
	return string(e.Code)
}

// Unwrap exposes the underlying library or storage failure.
func (e *Error) Unwrap() error { return e.Err }

// IsCode reports whether err carries the requested stable code.
func IsCode(err error, code ErrorCode) bool {
	var contractErr *Error
	return errors.As(err, &contractErr) && contractErr.Code == code
}

// IsContractFault reports whether the contract, rather than child output, is faulty.
func IsContractFault(err error) bool {
	var contractErr *Error
	return errors.As(err, &contractErr) && contractErr.Fault == FaultContract
}

func newError(code ErrorCode, fault FaultKind, message string, cause error) error {
	return &Error{Code: code, Fault: fault, Msg: message, Err: cause}
}

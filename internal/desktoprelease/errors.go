package desktoprelease

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrorInventoryIncomplete ErrorCode = "inventory_incomplete"
	ErrorChannelCASConflict  ErrorCode = "channel_cas_conflict"
	ErrorVerificationFailed  ErrorCode = "verification_failed"
)

type OperatorError struct {
	Code ErrorCode `json:"code"`
	Err  error     `json:"-"`
}

func (e *OperatorError) Error() string {
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

func (e *OperatorError) Unwrap() error {
	return e.Err
}

func errorWithCode(code ErrorCode, err error) error {
	if err == nil {
		return nil
	}
	if operatorErrorFrom(err) != nil {
		return err
	}
	return &OperatorError{Code: code, Err: err}
}

func CodeOf(err error) ErrorCode {
	if operatorError := operatorErrorFrom(err); operatorError != nil {
		return operatorError.Code
	}
	return ErrorVerificationFailed
}

func operatorErrorFrom(err error) *OperatorError {
	var operatorError *OperatorError
	if !errors.As(err, &operatorError) {
		return nil
	}
	return operatorError
}

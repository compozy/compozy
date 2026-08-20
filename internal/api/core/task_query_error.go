package core

import "fmt"

type invalidTaskQueryFieldError struct {
	field string
	cause error
}

func (e *invalidTaskQueryFieldError) Error() string {
	return fmt.Sprintf("invalid task query field %s: %v", e.field, e.cause)
}

func (e *invalidTaskQueryFieldError) Unwrap() error {
	return e.cause
}

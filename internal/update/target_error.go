package update

import (
	"errors"
	"fmt"
)

// TargetError identifies the update track that produced a planning failure.
type TargetError struct {
	Target Target
	Err    error
}

func (e *TargetError) Error() string {
	if e == nil {
		return "update: target failure"
	}
	return fmt.Sprintf("update: %s target: %v", e.Target, e.Err)
}

func (e *TargetError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ErrorTarget returns the track carried by a planning error.
func ErrorTarget(err error) (Target, bool) {
	var targetErr *TargetError
	if !errors.As(err, &targetErr) || targetErr == nil || targetErr.Target == "" {
		return "", false
	}
	return targetErr.Target, true
}

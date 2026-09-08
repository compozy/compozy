package session

import "fmt"

// ActiveTurnMismatchError carries the live fence observed before accepting a command.
type ActiveTurnMismatchError struct {
	ExpectedTurnID string
	CurrentTurnID  string
}

func (e *ActiveTurnMismatchError) Error() string {
	return fmt.Sprintf("%s: expected %s, active %s", ErrActiveTurnMismatch, e.ExpectedTurnID, e.CurrentTurnID)
}

func (*ActiveTurnMismatchError) Unwrap() error { return ErrActiveTurnMismatch }

package terminal

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound          = errors.New("terminal not found")
	ErrRequiresWorkspace = errors.New("terminal requires workspace")
	ErrGenerationFenced  = errors.New("terminal generation fenced")
	ErrLeaseRevoked      = errors.New("terminal lease revoked")
	ErrWriteOwnerHeld    = errors.New("terminal write owner held")
	ErrNotInteractive    = errors.New("terminal not interactive")
	ErrExited            = errors.New("terminal exited")
	ErrExpired           = errors.New("terminal expired")
	ErrLimitReached      = errors.New("terminal limit reached")
	ErrSubscriberLimit   = errors.New("terminal subscriber limit reached")
	ErrInvalidCwd        = errors.New("terminal invalid cwd")
	ErrInteractive       = errors.New("terminal interactive unavailable")
	ErrRecording         = errors.New("terminal recording unavailable")
	ErrShuttingDown      = errors.New("terminal manager shutting down")
	ErrUnsupported       = errors.New("terminal operation unsupported")
)

type Error struct {
	Code       string
	Message    string
	Controller *Actor
	Current    int
	Max        int
	Err        error
}

func (e *Error) Error() string {
	if e == nil {
		return "terminal error"
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("terminal: %s", e.Code)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

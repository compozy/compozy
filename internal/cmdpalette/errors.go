package cmdpalette

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidDescriptor  = errors.New("cmd palette: invalid descriptor")
	ErrDuplicateCommandID = errors.New("cmd palette: duplicate command id")
	ErrCommandNotFound    = errors.New("cmd palette: command not found")
	ErrAlreadyRunning     = errors.New("cmd palette: command already running")
	ErrNoAttachedShell    = errors.New("cmd palette: no attached shell")
	ErrMultipleClients    = errors.New("cmd palette: multiple clients")
	ErrCannotDeferSecrets = errors.New("cmd palette: cannot defer secrets")
	ErrClientUnauthorized = errors.New("cmd palette: client unauthorized")
	ErrInvalidExecution   = errors.New("cmd palette: invalid execution result")
)

type InvalidArgumentsError struct {
	Fields map[string]string
}

func (e *InvalidArgumentsError) Error() string { return "cmd palette: invalid arguments" }

type UnavailableError struct {
	Reason string
}

func (e *UnavailableError) Error() string {
	return fmt.Sprintf("cmd palette: command unavailable: %s", e.Reason)
}

type MultipleClientsError struct {
	Clients []ClientID
}

func (e *MultipleClientsError) Error() string {
	return fmt.Sprintf("%v: %v", ErrMultipleClients, e.Clients)
}

func (e *MultipleClientsError) Unwrap() error { return ErrMultipleClients }

type DuplicateCommandIDError struct {
	ID     CommandID
	First  string
	Second string
}

func (e *DuplicateCommandIDError) Error() string {
	return fmt.Sprintf("%v %q from %q and %q", ErrDuplicateCommandID, e.ID, e.First, e.Second)
}

func (e *DuplicateCommandIDError) Unwrap() error { return ErrDuplicateCommandID }

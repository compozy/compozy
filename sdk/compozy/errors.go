package compozy

import "errors"

var (
	// ErrAlreadyStarted indicates the engine has already been started.
	ErrAlreadyStarted = errors.New("engine already started")
	// ErrNotStarted indicates the engine has not been started yet.
	ErrNotStarted = errors.New("engine not started")
	// ErrConfigUnavailable indicates no configuration is available on the context.
	ErrConfigUnavailable = errors.New("configuration is unavailable in context")
)

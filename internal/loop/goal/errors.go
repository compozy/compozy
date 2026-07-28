package goal

import "errors"

var (
	// ErrCheckpointNotFound reports a missing workspace-owned checkpoint.
	ErrCheckpointNotFound = errors.New("loop goal: checkpoint not found")
	// ErrTurnNotFound reports a missing workspace-owned turn.
	ErrTurnNotFound = errors.New("loop goal: turn not found")
	// ErrBindingNotFound reports a missing workspace-owned binding attempt.
	ErrBindingNotFound = errors.New("loop goal: binding not found")
	// ErrOutboxEventNotFound reports an unknown session projection event.
	ErrOutboxEventNotFound = errors.New("loop goal: outbox event not found")
)

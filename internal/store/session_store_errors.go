package store

import "errors"

var (
	// ErrClosed reports that a session database no longer accepts writes.
	ErrClosed = errors.New("store: session database closed")
	// ErrDrainTimeout reports that shutdown timed out before queued writes drained.
	ErrDrainTimeout = errors.New("store: writer drain timeout")
	// ErrSessionWorkspaceMismatch reports an attempt to reassign a durable session identity.
	ErrSessionWorkspaceMismatch = errors.New("store: session workspace owner does not match")
)

package notifications

import "errors"

var (
	// ErrCursorNotFound reports that no durable notification cursor matched the key.
	ErrCursorNotFound = errors.New("notifications: cursor not found")
	// ErrInvalidCursor reports invalid cursor identity or cursor payload.
	ErrInvalidCursor = errors.New("notifications: invalid cursor")
	// ErrNonMonotonicCursor reports a cursor advance that would move backward or fork delivery metadata.
	ErrNonMonotonicCursor = errors.New("notifications: non-monotonic cursor advance")
	// ErrResetReasonRequired reports a reset request without an explicit recovery reason.
	ErrResetReasonRequired = errors.New("notifications: reset reason required")
)

package clientstate

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound reports that no live value exists for a key.
	ErrNotFound = errors.New("client state not found")
	// ErrWorkspaceNotFound reports an unknown, deleting, or stale workspace generation.
	ErrWorkspaceNotFound = errors.New("client state workspace not found")
	// ErrRevConflict reports a failed compare-and-swap revision.
	ErrRevConflict = errors.New("client state revision conflict")
	// ErrValueTooLarge reports a payload beyond the configured byte limit.
	ErrValueTooLarge = errors.New("client state value too large")
	// ErrKeyQuota reports that a new retained key would exceed workspace quota.
	ErrKeyQuota = errors.New("client state key quota exceeded")
	// ErrInvalidDomain reports a malformed internal domain.
	ErrInvalidDomain = errors.New("client state invalid domain")
	// ErrInvalidKey reports a malformed key.
	ErrInvalidKey = errors.New("client state invalid key")
	// ErrInvalidValue reports malformed JSON or a non-object JSON payload.
	ErrInvalidValue = errors.New("client state invalid value")
	// ErrEmptyApply reports an Apply request with no operations.
	ErrEmptyApply = errors.New("client state apply is empty")
	// ErrSlowConsumer reports subscription eviction after its queue fills.
	ErrSlowConsumer = errors.New("client state slow consumer")
	// ErrClosed reports an operation attempted after service shutdown.
	ErrClosed = errors.New("client state service closed")
	// ErrStoreFormatTooNew reports a durable store written by a newer binary.
	ErrStoreFormatTooNew = errors.New("client state store format is newer than supported")
)

// StoreFormatTooNewError records the incompatible durable and supported versions.
type StoreFormatTooNewError struct {
	Version          uint32
	SupportedVersion uint32
}

func (e *StoreFormatTooNewError) Error() string {
	return fmt.Sprintf(
		"clientstate: store format version %d is newer than supported version %d",
		e.Version,
		e.SupportedVersion,
	)
}

func (e *StoreFormatTooNewError) Unwrap() error {
	return ErrStoreFormatTooNew
}

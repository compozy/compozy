package fileutil

import (
	"errors"
	"fmt"
)

var (
	// ErrTargetExists reports an atomic no-replace publication collision.
	ErrTargetExists = errors.New("fileutil: target exists")
	// ErrAtomicNoReplaceUnsupported reports a platform without an atomic
	// no-replace directory rename primitive. The operation is not attempted.
	ErrAtomicNoReplaceUnsupported = errors.New("fileutil: atomic no-replace directory move is unsupported")
	// ErrMoveSourceIdentityChanged reports that the path named as a move source
	// no longer refers to the entry retained by the supplied capability.
	ErrMoveSourceIdentityChanged = errors.New("fileutil: move source identity changed")
	// ErrDirectoryMoveRecoveryRequired reports that an uncommitted move could
	// not restore its source name from the private transaction directory.
	ErrDirectoryMoveRecoveryRequired = errors.New("fileutil: directory move recovery required")
	// ErrMoveRecoveryEntryNotFound reports that a retained entry left its private transaction.
	ErrMoveRecoveryEntryNotFound = errors.New("fileutil: move recovery entry not found")
)

// MoveRecoveryError identifies a preserved private transaction and its retained entry.
type MoveRecoveryError struct {
	TransactionName string
	CandidateName   string
	Cause           error
}

func (e *MoveRecoveryError) Error() string {
	if e == nil {
		return ErrDirectoryMoveRecoveryRequired.Error()
	}
	detail := "retained entry left the private transaction"
	if e.CandidateName != "" {
		detail = fmt.Sprintf("candidate %q preserved", e.CandidateName)
	}
	return fmt.Sprintf(
		"%s: private transaction %q %s: %v",
		ErrDirectoryMoveRecoveryRequired,
		e.TransactionName,
		detail,
		e.Cause,
	)
}

// Unwrap preserves both the stable recovery classification and its cause.
func (e *MoveRecoveryError) Unwrap() []error {
	if e == nil || e.Cause == nil {
		return []error{ErrDirectoryMoveRecoveryRequired}
	}
	return []error{ErrDirectoryMoveRecoveryRequired, e.Cause}
}

// DirectoryMoveState separates effects that have not reached the canonical
// target from an irrevocably committed rename.
type DirectoryMoveState uint8

const (
	DirectoryMovePreCommit DirectoryMoveState = iota
	DirectoryMoveCollision
	DirectoryMoveCommitted
)

// DirectoryMoveResult reports whether the canonical target changed. A
// PostCommitErr never means a caller may attempt to undo the completed move.
type DirectoryMoveResult struct {
	State         DirectoryMoveState
	PostCommitErr error
}

// Committed reports whether the destination rename completed.
func (r DirectoryMoveResult) Committed() bool {
	return r.State == DirectoryMoveCommitted
}

// Collision reports whether no-replace publication found an existing target.
func (r DirectoryMoveResult) Collision() bool {
	return r.State == DirectoryMoveCollision
}

type directoryMoveBinding struct {
	parent   *Directory
	name     string
	identity directoryMoveIdentity
}

// OpenDirectoryForMove opens a direct child directory with the rights and
// identity proof required to publish that exact directory through MoveTo.
func (d *Directory) OpenDirectoryForMove(name string) (*Directory, error) {
	file, err := d.openChild(name, entryDirectory, openIntentMoveSource)
	if err != nil {
		return nil, err
	}
	if err := ensureDirectory(file); err != nil {
		return nil, closeFileWithError(file, err)
	}
	identity, err := directoryMoveIdentityFromFile(file)
	if err != nil {
		return nil, closeFileWithError(file, fmt.Errorf("fileutil: identify move source %q: %w", name, err))
	}
	return &Directory{
		file: file,
		move: &directoryMoveBinding{
			parent:   d,
			name:     name,
			identity: identity,
		},
	}, nil
}

// MoveTo publishes the directory capability opened by OpenDirectoryForMove.
// A committed result retains any subsequent durability failure in
// PostCommitErr, so callers can never mistake it for a failed rename.
func (d *Directory) MoveTo(target *Directory, targetName string, replace bool) (DirectoryMoveResult, error) {
	if d == nil || d.file == nil || d.move == nil || target == nil || target.file == nil {
		return DirectoryMoveResult{State: DirectoryMovePreCommit}, fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if err := validateChildName(targetName); err != nil {
		return DirectoryMoveResult{State: DirectoryMovePreCommit}, err
	}

	identity := d.move.identity
	result, err := moveBoundDirectory(d, target, targetName, replace)
	if result.Committed() {
		// Keep the capability bound to its committed name so callers can carry
		// the same exact directory through a multi-phase transaction.
		d.move = &directoryMoveBinding{
			parent:   target,
			name:     targetName,
			identity: identity,
		}
	}
	return result, err
}

func wrapDirectoryMovePostCommitError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("fileutil: %s: %w", operation, err)
}

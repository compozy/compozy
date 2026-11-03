package store

import (
	"context"
	"errors"
	"time"
)

// ErrSessionProfileMoveExists reports an open move already journaled for the session.
var ErrSessionProfileMoveExists = errors.New("store: session profile move already open")

// SessionProfileMove journals one open move of a session to another profile. It is
// written before the source is archived and deleted once the move completes or
// rolls back. Every id is chosen before the step that creates it, so the move's
// progress reads from durable state: the archive flag, the worktree rows, and the
// copy's derive receipt under CopyKey.
type SessionProfileMove struct {
	SessionID       string
	WorkspaceID     string
	ProfileID       string
	TargetProfileID string
	// CopyKey is the derive idempotency key of the copy in the target profile.
	CopyKey string
	// SourceWorktreeID, Branch, and HeadSHA are the source checkout; empty without one.
	SourceWorktreeID string
	Branch           string
	HeadSHA          string
	// TargetWorktreeID is the copy's checkout; RestoreWorktreeID is the checkout a
	// rollback creates for the source when its own was already released.
	TargetWorktreeID  string
	RestoreWorktreeID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// SessionProfileMoveStore persists the move-to-profile journal.
type SessionProfileMoveStore interface {
	// InsertSessionProfileMove fails with ErrSessionProfileMoveExists when the session
	// already has an open move.
	InsertSessionProfileMove(ctx context.Context, move SessionProfileMove) error
	SessionProfileMove(ctx context.Context, sessionID string) (SessionProfileMove, bool, error)
	ListSessionProfileMoves(ctx context.Context) ([]SessionProfileMove, error)
	// SetSessionProfileMoveRestoreWorktree replaces the restore checkout id after a
	// restore attempt left its row unusable.
	SetSessionProfileMoveRestoreWorktree(ctx context.Context, sessionID string, worktreeID string) error
	DeleteSessionProfileMove(ctx context.Context, sessionID string) error
}

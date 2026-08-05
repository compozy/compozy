package store

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrSessionArchived reports an attempt to reactivate an archived session.
	ErrSessionArchived = errors.New("store: session is archived")
	// ErrSessionArchiveRequiresStopped reports an archive attempt for a live session.
	ErrSessionArchiveRequiresStopped = errors.New("store: session must be stopped before archiving")
)

// SessionCatalog manages global session index records.
type SessionCatalog interface {
	RegisterSession(ctx context.Context, session SessionInfo) error
	UpdateSessionState(ctx context.Context, update SessionStateUpdate) error
	DeleteSession(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, query SessionListQuery) ([]SessionInfo, error)
	AttachSession(ctx context.Context, req SessionAttachRequest) (SessionAttach, error)
	ReconcileSessions(ctx context.Context, sessions []SessionInfo) (ReconcileResult, error)
}

// SessionArchiveStore owns the workspace-scoped archive metadata for sessions.
type SessionArchiveStore interface {
	SessionArchivedAt(ctx context.Context, workspaceID string, sessionID string) (*time.Time, error)
	SetSessionArchived(
		ctx context.Context,
		workspaceID string,
		sessionID string,
		archived bool,
	) (SessionInfo, error)
}

// SessionTranscriptEpochStore manages destructive transcript reset epochs.
type SessionTranscriptEpochStore interface {
	SessionTranscriptEpoch(ctx context.Context, sessionID string) (int64, error)
	EnsureSessionTranscriptEpoch(ctx context.Context, update SessionTranscriptEpochUpdate) (int64, error)
}

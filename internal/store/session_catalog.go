package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrSessionIsInstanceMain = errors.New("session_is_instance_main")
	// ErrSubagentArchiveFollowsParent rejects independent subagent archive changes.
	ErrSubagentArchiveFollowsParent = errors.New("store: subagent archive follows parent")
	// ErrSessionArchived reports an attempt to reactivate an archived session.
	ErrSessionArchived = errors.New("store: session is archived")
	// ErrSessionArchiveRequiresStopped reports an archive attempt for a live session.
	ErrSessionArchiveRequiresStopped = errors.New("store: session must be stopped before archiving")
)

// SessionArchiveLiveMemberError names the archive-family session that is not
// stopped, so a caller that may stop sessions knows which one blocks archiving.
type SessionArchiveLiveMemberError struct {
	SessionID string
}

func (e *SessionArchiveLiveMemberError) Error() string {
	return ErrSessionArchiveRequiresStopped.Error() + ": " + e.SessionID
}

// Unwrap keeps the live-member refusal classified as ErrSessionArchiveRequiresStopped.
func (e *SessionArchiveLiveMemberError) Unwrap() error {
	return ErrSessionArchiveRequiresStopped
}

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

// SessionLineageUpdate replaces the hierarchy fields of one session's lineage:
// parent, root, depth, kind, and auto-stop. Governance fields stay untouched.
type SessionLineageUpdate struct {
	SessionID string
	Lineage   SessionLineage
	UpdatedAt time.Time
}

// SessionLineageStore rewrites persisted session hierarchy for a reparent.
type SessionLineageStore interface {
	// UpdateSessionLineages applies every update in one transaction and fails
	// when any targeted session is missing from the workspace.
	UpdateSessionLineages(ctx context.Context, workspaceID string, updates []SessionLineageUpdate) error
}

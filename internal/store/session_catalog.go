package store

import "context"

// SessionCatalog manages global session index records.
type SessionCatalog interface {
	RegisterSession(ctx context.Context, session SessionInfo) error
	UpdateSessionState(ctx context.Context, update SessionStateUpdate) error
	DeleteSession(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, query SessionListQuery) ([]SessionInfo, error)
	AttachSession(ctx context.Context, req SessionAttachRequest) (SessionAttach, error)
	ReconcileSessions(ctx context.Context, sessions []SessionInfo) (ReconcileResult, error)
}

// SessionTranscriptEpochStore manages destructive transcript reset epochs.
type SessionTranscriptEpochStore interface {
	SessionTranscriptEpoch(ctx context.Context, sessionID string) (int64, error)
	EnsureSessionTranscriptEpoch(ctx context.Context, update SessionTranscriptEpochUpdate) (int64, error)
}

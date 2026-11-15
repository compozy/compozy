package store

import (
	"context"
	"errors"
	"time"
)

// ErrSessionNotArchived refuses an archived-only deletion of a session that is
// no longer archived, or no longer owned by the expected profile and workspace.
var ErrSessionNotArchived = errors.New("store: session is not archived")

// SessionHistoryCleanupQuery reads the archived history of one profile, in one
// workspace or across all of them.
type SessionHistoryCleanupQuery struct {
	ProfileID string
	// WorkspaceID is a workspace id or the user selector; empty with AllWorkspaces.
	WorkspaceID   string
	AllWorkspaces bool
}

// SessionHistoryCleanupRow is the slice of a session the cleanup planner needs:
// every archived session in scope, and every unarchived session whose parent is
// archived, so the planner can keep it and say so.
type SessionHistoryCleanupRow struct {
	ID              string
	WorkspaceID     string
	Name            string
	ParentSessionID string
	SpawnRole       string
	LineageKind     LineageKind
	ArchivedAt      *time.Time
	// InstanceMain marks the main session of an always-on agent, which is never deleted.
	InstanceMain bool
}

// ArchivedSessionDelete names one archived session and the owner it must still have.
type ArchivedSessionDelete struct {
	SessionID   string
	ProfileID   string
	WorkspaceID string
}

// SessionHistoryCleanupStore reads archived history and deletes one archived
// session at a time, re-checking in the deleting transaction that it is still
// archived and still in its owner's scope.
type SessionHistoryCleanupStore interface {
	ListSessionHistoryCleanupRows(
		ctx context.Context,
		query SessionHistoryCleanupQuery,
	) ([]SessionHistoryCleanupRow, error)
	DeleteArchivedSession(ctx context.Context, target ArchivedSessionDelete) error
}

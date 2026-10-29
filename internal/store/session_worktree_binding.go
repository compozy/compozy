package store

import (
	"context"
	"errors"
	"time"
)

var ErrSessionWorktreeConflict = errors.New("store: session worktree binding conflict")

// SessionWorktreeTransition records an accepted change until both stores converge.
type SessionWorktreeTransition struct {
	Previous      SessionExecutionLocationState `json:"previous"`
	Current       SessionExecutionLocationState `json:"current"`
	Pending       bool                          `json:"pending,omitzero"`
	NoticePending bool                          `json:"notice_pending,omitzero"`
}

func CloneSessionWorktreeTransition(value *SessionWorktreeTransition) *SessionWorktreeTransition {
	if value == nil {
		return nil
	}
	return new(*value)
}

type SessionWorktreeBindingUpdate struct {
	SessionID          string
	WorkspaceID        string
	ProfileID          string
	ExpectedWorktreeID string
	WorktreeID         string
	UpdatedAt          time.Time
}

type SessionWorktreeBindingStore interface {
	UpdateSessionWorktreeBinding(context.Context, SessionWorktreeBindingUpdate) error
	RecoverSessionWorktreeBinding(context.Context, SessionWorktreeBindingUpdate) error
}

// RecoverSessionWorktreeTransition finishes only an already persisted binding intent.
func RecoverSessionWorktreeTransition(
	ctx context.Context,
	catalog SessionWorktreeBindingStore,
	path string,
	meta *SessionMeta,
) error {
	transition := meta.WorktreeTransition
	if transition == nil || !transition.Pending {
		return nil
	}
	if catalog == nil {
		return errors.New("store: worktree binding recovery store is unavailable")
	}
	if meta.Liveness != nil && meta.Liveness.SubprocessPID != 0 {
		return errors.New("store: cannot recover worktree binding before process exit verification")
	}
	if err := catalog.RecoverSessionWorktreeBinding(ctx, SessionWorktreeBindingUpdate{
		SessionID: meta.ID, WorkspaceID: meta.WorkspaceID, ProfileID: meta.ProfileID,
		ExpectedWorktreeID: transition.Previous.WorktreeID, WorktreeID: transition.Current.WorktreeID,
		UpdatedAt: meta.UpdatedAt,
	}); err != nil {
		return err
	}
	meta.SetWorktreeID(transition.Current.WorktreeID)
	meta.SetCWD(transition.Current.CWD)
	meta.WorktreeTransition = CloneSessionWorktreeTransition(transition)
	meta.WorktreeTransition.Pending = false
	return WriteSessionMeta(path, meta)
}

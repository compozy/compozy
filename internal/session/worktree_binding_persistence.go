package session

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) writeWorktreeBinding(
	ctx context.Context,
	meta *store.SessionMeta,
	cwd, worktreeID string,
	update func(),
) error {
	catalog, ok := m.sessionCatalog.(store.SessionWorktreeBindingStore)
	if !ok {
		return errors.New("session: worktree binding store is unavailable")
	}
	previous := *meta
	if meta.SessionExecutionLocationState != nil {
		previous.SessionExecutionLocationState = new(*meta.SessionExecutionLocationState)
	}
	transition := &store.SessionWorktreeTransition{
		Previous: store.SessionExecutionLocationState{WorktreeID: meta.WorktreeIDValue(), CWD: meta.CWDValue()},
		Current:  store.SessionExecutionLocationState{WorktreeID: worktreeID, CWD: cwd},
		Pending:  true, NoticePending: true,
	}
	meta.WorktreeTransition = transition
	meta.SetWorktreeID(worktreeID)
	meta.SetCWD(cwd)
	meta.UpdatedAt = m.now()
	path := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, meta.ID))
	if err := store.WriteSessionMeta(path, meta); err != nil {
		return err
	}
	if update != nil {
		update()
	}
	if err := catalog.UpdateSessionWorktreeBinding(ctx, store.SessionWorktreeBindingUpdate{
		SessionID: meta.ID, WorkspaceID: meta.WorkspaceID, ProfileID: meta.ProfileID,
		ExpectedWorktreeID: transition.Previous.WorktreeID, WorktreeID: worktreeID, UpdatedAt: meta.UpdatedAt,
	}); err != nil {
		if errors.Is(err, store.ErrSessionWorktreeConflict) {
			if rollbackErr := store.WriteSessionMeta(path, &previous); rollbackErr != nil {
				return errors.Join(err, rollbackErr)
			}
			*meta = previous
			if update != nil {
				update()
			}
		}
		return err
	}
	meta.WorktreeTransition = store.CloneSessionWorktreeTransition(transition)
	meta.WorktreeTransition.Pending = false
	if err := store.WriteSessionMeta(path, meta); err != nil {
		return err
	}
	if update != nil {
		update()
	}
	return nil
}

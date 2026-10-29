package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.SessionWorktreeBindingStore = (*SessionRepo)(nil)

func (g *SessionRepo) UpdateSessionWorktreeBinding(
	ctx context.Context,
	update store.SessionWorktreeBindingUpdate,
) error {
	return g.updateSessionWorktreeBinding(ctx, update, false)
}

func (g *SessionRepo) RecoverSessionWorktreeBinding(
	ctx context.Context,
	update store.SessionWorktreeBindingUpdate,
) error {
	return g.updateSessionWorktreeBinding(ctx, update, true)
}

func (g *SessionRepo) updateSessionWorktreeBinding(
	ctx context.Context,
	update store.SessionWorktreeBindingUpdate,
	recovery bool,
) error {
	if err := g.checkReady(ctx, "update session worktree binding"); err != nil {
		return err
	}
	if update.SessionID == "" || update.WorkspaceID == "" || update.ProfileID == "" {
		return fmt.Errorf("store: session, workspace and profile are required for worktree binding")
	}
	return g.withImmediateTransaction(ctx, "update session worktree binding", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		recoverAccepted := int64(0)
		if recovery {
			recoverAccepted = 1
		}
		affected, err := q.UpdateSessionWorktreeBinding(ctx, sqlcgen.UpdateSessionWorktreeBindingParams{
			ID: update.SessionID, WorkspaceID: update.WorkspaceID, ProfileID: update.ProfileID,
			ExpectedWorktreeID: nullableSessionString(update.ExpectedWorktreeID),
			WorktreeID:         nullableSessionString(update.WorktreeID),
			UpdatedAt:          store.FormatTimestamp(update.UpdatedAt), RecoverAccepted: recoverAccepted,
		})
		if err != nil {
			return fmt.Errorf("store: update session worktree binding: %w", err)
		}
		if affected == 0 {
			return store.ErrSessionWorktreeConflict
		}
		return nil
	})
}

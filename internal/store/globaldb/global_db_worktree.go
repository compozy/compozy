package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/worktree"
)

// WorktreeStore exposes the domain repository without widening the daemon registry aggregate.
func (g *GlobalDB) WorktreeStore() worktree.Store {
	if g == nil {
		return nil
	}
	return g.Worktrees
}

func (g *WorktreeRepo) Insert(ctx context.Context, item worktree.Worktree) error {
	if err := g.checkReady(ctx, "insert worktree"); err != nil {
		return err
	}
	return g.queries.InsertWorktree(ctx, sqlcgen.InsertWorktreeParams{
		ID: strings.TrimSpace(item.ID), WorkspaceID: strings.TrimSpace(item.WorkspaceID),
		Name: strings.TrimSpace(item.Name), Branch: strings.TrimSpace(item.Branch),
		Path: strings.TrimSpace(item.Path), GitDir: strings.TrimSpace(item.GitDir),
		State: string(item.State), PendingPhase: string(item.PendingPhase), Origin: string(item.Origin),
		SetupState: string(item.SetupState), SetupError: item.SetupError, BaseRef: item.BaseRef,
		CreatedBranch: boolToInt64(item.CreatedBranch), RunNamespace: item.RunNamespace,
		CreatedHead: item.CreatedHead, RunID: item.RunID,
		CreatedAt: store.FormatTimestamp(item.CreatedAt), UpdatedAt: store.FormatTimestamp(item.UpdatedAt),
	})
}

func (g *WorktreeRepo) Get(ctx context.Context, workspaceID, ref string) (*worktree.Worktree, error) {
	if err := g.checkReady(ctx, "get worktree"); err != nil {
		return nil, err
	}
	row, err := g.queries.GetWorktree(ctx, sqlcgen.GetWorktreeParams{
		WorkspaceID: strings.TrimSpace(workspaceID), Ref: strings.TrimSpace(ref),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, worktree.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get worktree: %w", err)
	}
	return worktreeFromSQLC(row)
}

func (g *WorktreeRepo) GetByPath(ctx context.Context, workspaceID, path string) (*worktree.Worktree, error) {
	if err := g.checkReady(ctx, "get worktree by path"); err != nil {
		return nil, err
	}
	row, err := g.queries.GetLiveWorktreeByPath(ctx, sqlcgen.GetLiveWorktreeByPathParams{
		WorkspaceID: strings.TrimSpace(workspaceID), Path: strings.TrimSpace(path),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, worktree.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get worktree by path: %w", err)
	}
	return worktreeFromSQLC(row)
}

func (g *WorktreeRepo) List(ctx context.Context, workspaceID string) ([]worktree.Worktree, error) {
	if err := g.checkReady(ctx, "list worktrees"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListWorktrees(ctx, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("store: list worktrees: %w", err)
	}
	return worktreesFromSQLC(rows)
}

func (g *WorktreeRepo) ListRecoverable(ctx context.Context) ([]worktree.Worktree, error) {
	if err := g.checkReady(ctx, "list recoverable worktrees"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListRecoverableWorktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list recoverable worktrees: %w", err)
	}
	return worktreesFromSQLC(rows)
}

func (g *WorktreeRepo) DeletePending(ctx context.Context, workspaceID, id string) error {
	if err := g.checkReady(ctx, "delete pending worktree"); err != nil {
		return err
	}
	count, err := g.queries.DeletePendingWorktree(ctx, sqlcgen.DeletePendingWorktreeParams{
		WorkspaceID: strings.TrimSpace(workspaceID), WorktreeID: strings.TrimSpace(id),
	})
	if err != nil {
		return fmt.Errorf("store: delete pending worktree: %w", err)
	}
	return g.pendingMutationResult(ctx, workspaceID, id, count)
}

func (g *WorktreeRepo) UpdatePending(
	ctx context.Context,
	workspaceID string,
	id string,
	update worktree.PendingUpdate,
) error {
	if err := g.checkReady(ctx, "update pending worktree"); err != nil {
		return err
	}
	count, err := g.queries.UpdatePendingWorktree(ctx, sqlcgen.UpdatePendingWorktreeParams{
		PendingPhase: string(update.Phase), Branch: update.Branch, GitDir: update.GitDir,
		BaseRef: update.BaseRef, CreatedBranch: boolToInt64(update.CreatedBranch),
		RunNamespace: update.RunNamespace, CreatedHead: update.CreatedHead,
		UpdatedAt:   store.FormatTimestamp(g.now().UTC()),
		WorkspaceID: strings.TrimSpace(workspaceID), WorktreeID: strings.TrimSpace(id),
	})
	if err != nil {
		return fmt.Errorf("store: update pending worktree: %w", err)
	}
	return g.pendingMutationResult(ctx, workspaceID, id, count)
}

func (g *WorktreeRepo) CompleteCreate(
	ctx context.Context,
	workspaceID string,
	id string,
	setupState worktree.SetupState,
	setupError string,
	updatedAt time.Time,
) error {
	if err := g.checkReady(ctx, "complete worktree create"); err != nil {
		return err
	}
	count, err := g.queries.CompleteWorktreeCreate(ctx, sqlcgen.CompleteWorktreeCreateParams{
		SetupState: string(setupState), SetupError: setupError,
		UpdatedAt: store.FormatTimestamp(updatedAt), WorkspaceID: strings.TrimSpace(workspaceID),
		WorktreeID: strings.TrimSpace(id),
	})
	if err != nil {
		return fmt.Errorf("store: complete worktree create: %w", err)
	}
	return g.pendingMutationResult(ctx, workspaceID, id, count)
}

func (g *WorktreeRepo) CompareAndSwapState(
	ctx context.Context,
	workspaceID string,
	id string,
	expected worktree.State,
	next worktree.State,
	updatedAt time.Time,
) (bool, error) {
	if err := g.checkReady(ctx, "compare and swap worktree state"); err != nil {
		return false, err
	}
	count, err := g.queries.CompareAndSwapWorktreeState(ctx, sqlcgen.CompareAndSwapWorktreeStateParams{
		NextState: string(next), UpdatedAt: store.FormatTimestamp(updatedAt),
		WorkspaceID: strings.TrimSpace(workspaceID), WorktreeID: strings.TrimSpace(id),
		ExpectedState: string(expected),
	})
	if err != nil {
		return false, fmt.Errorf("store: compare and swap worktree state: %w", err)
	}
	return count == 1, nil
}

func (g *WorktreeRepo) SetState(
	ctx context.Context,
	workspaceID string,
	id string,
	state worktree.State,
	updatedAt time.Time,
) error {
	if err := g.checkReady(ctx, "set worktree state"); err != nil {
		return err
	}
	count, err := g.queries.SetWorktreeState(ctx, sqlcgen.SetWorktreeStateParams{
		State: string(state), UpdatedAt: store.FormatTimestamp(updatedAt),
		WorkspaceID: strings.TrimSpace(workspaceID), WorktreeID: strings.TrimSpace(id),
	})
	if err != nil {
		return fmt.Errorf("store: set worktree state: %w", err)
	}
	if count == 0 {
		return worktree.ErrNotFound
	}
	return nil
}

func (g *WorktreeRepo) pendingMutationResult(
	ctx context.Context,
	workspaceID string,
	id string,
	count int64,
) error {
	if count == 1 {
		return nil
	}
	item, err := g.Get(ctx, workspaceID, id)
	if errors.Is(err, worktree.ErrNotFound) || item == nil {
		return worktree.ErrNotFound
	}
	if err != nil {
		return err
	}
	return worktree.ErrNotPending
}

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

func (g *WorktreeRepo) SaveStatus(
	ctx context.Context,
	workspaceID string,
	id string,
	status worktree.Status,
) error {
	if err := g.checkReady(ctx, "save worktree status"); err != nil {
		return err
	}
	persistedAhead := status.Ahead
	if persistedAhead == nil {
		persistedAhead = status.AheadOfBase
	}
	count, err := g.queries.UpsertWorktreeStatus(ctx, sqlcgen.UpsertWorktreeStatusParams{
		Branch: nullableString(status.Branch), Detached: nullableBool(status.Detached),
		HeadSha: nullableString(status.HeadSHA), DirtyFiles: nullableInt(status.DirtyFiles),
		Insertions: nullableInt(status.Insertions), Deletions: nullableInt(status.Deletions),
		HasUpstream: nullableBool(status.HasUpstream), Ahead: nullableInt(persistedAhead),
		Behind: nullableInt(status.Behind), ReadError: status.ReadError,
		RefreshedAt: nullableTime(status.RefreshedAt), WorkspaceID: strings.TrimSpace(workspaceID),
		WorktreeID: strings.TrimSpace(id),
	})
	if err != nil {
		return fmt.Errorf("store: save worktree status: %w", err)
	}
	if count == 0 {
		return worktree.ErrNotFound
	}
	return nil
}

func (g *WorktreeRepo) GetStatus(
	ctx context.Context,
	workspaceID string,
	id string,
) (*worktree.Status, error) {
	if err := g.checkReady(ctx, "get worktree status"); err != nil {
		return nil, err
	}
	row, err := g.queries.GetWorktreeStatus(ctx, sqlcgen.GetWorktreeStatusParams{
		WorkspaceID: strings.TrimSpace(workspaceID), WorktreeID: strings.TrimSpace(id),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get worktree status: %w", err)
	}
	refreshedAt, err := pointerTime(row.RefreshedAt)
	if err != nil {
		return nil, err
	}
	status := &worktree.Status{
		WorktreeID: row.WorktreeID, Branch: pointerString(row.Branch), Detached: pointerBool(row.Detached),
		HeadSHA: pointerString(row.HeadSha), DirtyFiles: pointerInt(row.DirtyFiles),
		Insertions: pointerInt(row.Insertions), Deletions: pointerInt(row.Deletions),
		HasUpstream: pointerBool(row.HasUpstream), Behind: pointerInt(row.Behind),
		ReadError: row.ReadError, RefreshedAt: refreshedAt,
	}
	if status.HasUpstream != nil && *status.HasUpstream {
		status.Ahead = pointerInt(row.Ahead)
	} else {
		status.AheadOfBase = pointerInt(row.Ahead)
	}
	return status, nil
}

func (g *WorktreeRepo) SaveForgeStatus(
	ctx context.Context,
	workspaceID string,
	id string,
	status worktree.ForgeStatus,
) error {
	if err := g.checkReady(ctx, "save worktree forge status"); err != nil {
		return err
	}
	count, err := g.queries.UpsertWorktreeForgeStatus(ctx, sqlcgen.UpsertWorktreeForgeStatusParams{
		Provider: status.Provider, PrNumber: nullableInt(status.PRNumber),
		PrState: nullableString(status.PRState), PrUrl: status.PRURL,
		Merged: nullableBool(status.Merged), FetchedAt: nullableTime(status.FetchedAt),
		WorkspaceID: strings.TrimSpace(workspaceID), WorktreeID: strings.TrimSpace(id),
	})
	if err != nil {
		return fmt.Errorf("store: save worktree forge status: %w", err)
	}
	if count == 0 {
		return worktree.ErrNotFound
	}
	return nil
}

func (g *WorktreeRepo) GetForgeStatus(
	ctx context.Context,
	workspaceID string,
	id string,
) (*worktree.ForgeStatus, error) {
	if err := g.checkReady(ctx, "get worktree forge status"); err != nil {
		return nil, err
	}
	row, err := g.queries.GetWorktreeForgeStatus(ctx, sqlcgen.GetWorktreeForgeStatusParams{
		WorkspaceID: strings.TrimSpace(workspaceID), WorktreeID: strings.TrimSpace(id),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get worktree forge status: %w", err)
	}
	fetchedAt, err := pointerTime(row.FetchedAt)
	if err != nil {
		return nil, err
	}
	return &worktree.ForgeStatus{
		WorktreeID: row.WorktreeID, Provider: row.Provider, PRNumber: pointerInt(row.PrNumber),
		PRState: pointerString(row.PrState), PRURL: row.PrUrl, Merged: pointerBool(row.Merged),
		FetchedAt: fetchedAt,
	}, nil
}

func (g *WorktreeRepo) InsertExitOperation(ctx context.Context, operation worktree.ExitOperation) error {
	if err := g.checkReady(ctx, "insert worktree exit operation"); err != nil {
		return err
	}
	err := g.withImmediateTransaction(ctx, "insert worktree exit operation", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		item, getErr := queries.GetWorktreeByID(ctx, sqlcgen.GetWorktreeByIDParams{
			WorkspaceID: strings.TrimSpace(operation.WorkspaceID),
			WorktreeID:  strings.TrimSpace(operation.WorktreeID),
		})
		if errors.Is(getErr, sql.ErrNoRows) {
			return worktree.ErrNotFound
		}
		if getErr != nil {
			return fmt.Errorf("read exit target: %w", getErr)
		}
		if item.State != string(worktree.StateReady) {
			return worktree.ErrNotReady
		}
		return queries.InsertWorktreeExitOperation(ctx, sqlcgen.InsertWorktreeExitOperationParams{
			OpID: operation.ID, WorkspaceID: operation.WorkspaceID, WorktreeID: operation.WorktreeID,
			Action: operation.Action, State: operation.State,
			StartedAt: store.FormatTimestamp(operation.StartedAt), FinishedAt: nullableTime(operation.FinishedAt),
		})
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return worktree.ErrOperationInProgress
		}
		return fmt.Errorf("store: insert worktree exit operation: %w", err)
	}
	return nil
}

func (g *WorktreeRepo) ListRunningExitOperations(ctx context.Context) ([]worktree.ExitOperation, error) {
	if err := g.checkReady(ctx, "list running worktree exit operations"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListRunningWorktreeExitOperations(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list running worktree exit operations: %w", err)
	}
	operations := make([]worktree.ExitOperation, 0, len(rows))
	for _, row := range rows {
		startedAt, parseErr := store.ParseTimestamp(row.StartedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("store: parse worktree exit start: %w", parseErr)
		}
		finishedAt, parseErr := pointerTime(row.FinishedAt)
		if parseErr != nil {
			return nil, parseErr
		}
		operations = append(operations, worktree.ExitOperation{
			ID: row.OpID, WorkspaceID: row.WorkspaceID, WorktreeID: row.WorktreeID,
			Action: row.Action, State: row.State, StartedAt: startedAt, FinishedAt: finishedAt,
		})
	}
	return operations, nil
}

func (g *WorktreeRepo) FinishExitOperation(
	ctx context.Context,
	workspaceID string,
	worktreeID string,
	opID string,
	state string,
	finishedAt time.Time,
) (bool, error) {
	if err := g.checkReady(ctx, "finish worktree exit operation"); err != nil {
		return false, err
	}
	count, err := g.queries.FinishWorktreeExitOperation(ctx, sqlcgen.FinishWorktreeExitOperationParams{
		State: state, FinishedAt: nullableTime(&finishedAt), WorkspaceID: workspaceID,
		WorktreeID: worktreeID, OpID: opID,
	})
	if err != nil {
		return false, fmt.Errorf("store: finish worktree exit operation: %w", err)
	}
	return count == 1, nil
}

func (g *WorktreeRepo) FailRunningExitOperations(ctx context.Context, finishedAt time.Time) error {
	if err := g.checkReady(ctx, "fail running worktree exit operations"); err != nil {
		return err
	}
	if err := g.queries.FailRunningWorktreeExitOperations(ctx, nullableTime(&finishedAt)); err != nil {
		return fmt.Errorf("store: fail running worktree exit operations: %w", err)
	}
	return nil
}

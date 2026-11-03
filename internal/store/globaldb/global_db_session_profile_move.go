package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.SessionProfileMoveStore = (*SessionRepo)(nil)

// InsertSessionProfileMove journals an open move before its source is archived.
func (g *SessionRepo) InsertSessionProfileMove(ctx context.Context, move store.SessionProfileMove) error {
	if err := g.checkReady(ctx, "insert session profile move"); err != nil {
		return err
	}
	now := g.now()
	if move.CreatedAt.IsZero() {
		move.CreatedAt = now
	}
	err := g.queries.InsertSessionProfileMove(ctx, sqlcgen.InsertSessionProfileMoveParams{
		SessionID: strings.TrimSpace(move.SessionID), WorkspaceID: strings.TrimSpace(move.WorkspaceID),
		ProfileID: strings.TrimSpace(move.ProfileID), TargetProfileID: strings.TrimSpace(move.TargetProfileID),
		CopyKey: strings.TrimSpace(move.CopyKey), SourceWorktreeID: strings.TrimSpace(move.SourceWorktreeID),
		Branch: strings.TrimSpace(move.Branch), HeadSha: strings.TrimSpace(move.HeadSHA),
		TargetWorktreeID:  strings.TrimSpace(move.TargetWorktreeID),
		RestoreWorktreeID: strings.TrimSpace(move.RestoreWorktreeID),
		CreatedAt:         store.FormatTimestamp(move.CreatedAt), UpdatedAt: store.FormatTimestamp(now),
	})
	if isSQLitePrimaryKeyConstraint(err) {
		return fmt.Errorf("%w: %s", store.ErrSessionProfileMoveExists, move.SessionID)
	}
	if err != nil {
		return fmt.Errorf("store: insert session profile move %q: %w", move.SessionID, err)
	}
	return nil
}

// SessionProfileMove reads the open move of one session.
func (g *SessionRepo) SessionProfileMove(
	ctx context.Context,
	sessionID string,
) (store.SessionProfileMove, bool, error) {
	if err := g.checkReady(ctx, "get session profile move"); err != nil {
		return store.SessionProfileMove{}, false, err
	}
	row, err := g.queries.GetSessionProfileMove(ctx, strings.TrimSpace(sessionID))
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionProfileMove{}, false, nil
	}
	if err != nil {
		return store.SessionProfileMove{}, false, fmt.Errorf("store: get session profile move %q: %w", sessionID, err)
	}
	move, err := sessionProfileMoveFromRow(row)
	return move, err == nil, err
}

// ListSessionProfileMoves lists every open move, oldest first.
func (g *SessionRepo) ListSessionProfileMoves(ctx context.Context) ([]store.SessionProfileMove, error) {
	if err := g.checkReady(ctx, "list session profile moves"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListSessionProfileMoves(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list session profile moves: %w", err)
	}
	moves := make([]store.SessionProfileMove, 0, len(rows))
	for _, row := range rows {
		move, err := sessionProfileMoveFromRow(row)
		if err != nil {
			return nil, err
		}
		moves = append(moves, move)
	}
	return moves, nil
}

// SetSessionProfileMoveRestoreWorktree records the checkout id the next restore attempt uses.
func (g *SessionRepo) SetSessionProfileMoveRestoreWorktree(
	ctx context.Context,
	sessionID string,
	worktreeID string,
) error {
	if err := g.checkReady(ctx, "update session profile move"); err != nil {
		return err
	}
	count, err := g.queries.SetSessionProfileMoveRestoreWorktree(
		ctx,
		sqlcgen.SetSessionProfileMoveRestoreWorktreeParams{
			RestoreWorktreeID: strings.TrimSpace(worktreeID), UpdatedAt: store.FormatTimestamp(g.now()),
			SessionID: strings.TrimSpace(sessionID),
		},
	)
	if err != nil {
		return fmt.Errorf("store: update session profile move %q: %w", sessionID, err)
	}
	if count == 0 {
		return fmt.Errorf("store: session profile move %q is not open", sessionID)
	}
	return nil
}

// DeleteSessionProfileMove closes a move that completed or rolled back.
func (g *SessionRepo) DeleteSessionProfileMove(ctx context.Context, sessionID string) error {
	if err := g.checkReady(ctx, "delete session profile move"); err != nil {
		return err
	}
	if err := g.queries.DeleteSessionProfileMove(ctx, strings.TrimSpace(sessionID)); err != nil {
		return fmt.Errorf("store: delete session profile move %q: %w", sessionID, err)
	}
	return nil
}

// CloseCompletedSessionProfileMove closes a move whose copy still exists.
func (g *SessionRepo) CloseCompletedSessionProfileMove(ctx context.Context, sessionID string) (bool, error) {
	if err := g.checkReady(ctx, "close completed session profile move"); err != nil {
		return false, err
	}
	count, err := g.queries.CloseCompletedSessionProfileMove(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return false, fmt.Errorf("store: close completed session profile move %q: %w", sessionID, err)
	}
	return count == 1, nil
}

func sessionProfileMoveFromRow(row sqlcgen.SessionProfileMove) (store.SessionProfileMove, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return store.SessionProfileMove{}, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return store.SessionProfileMove{}, err
	}
	return store.SessionProfileMove{
		SessionID: row.SessionID, WorkspaceID: row.WorkspaceID, ProfileID: row.ProfileID,
		TargetProfileID: row.TargetProfileID, CopyKey: row.CopyKey, SourceWorktreeID: row.SourceWorktreeID,
		Branch: row.Branch, HeadSHA: row.HeadSha, TargetWorktreeID: row.TargetWorktreeID,
		RestoreWorktreeID: row.RestoreWorktreeID, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

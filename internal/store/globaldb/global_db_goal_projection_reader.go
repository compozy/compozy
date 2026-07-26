package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ goal.SessionProjectionReader = (*GoalRepo)(nil)

// GetSessionGoalProjection selects the newest session-origin Run before applying its clear tombstone.
func (g *GoalRepo) GetSessionGoalProjection(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
) (projection goal.SessionProjection, err error) {
	if err := g.checkReady(ctx, "get session Goal projection"); err != nil {
		return goal.SessionProjection{}, err
	}
	workspaceID = looppkg.WorkspaceID(strings.TrimSpace(string(workspaceID)))
	sessionID = strings.TrimSpace(sessionID)
	if workspaceID == "" || sessionID == "" {
		return goal.SessionProjection{}, fmt.Errorf(
			"%w: Goal workspace_id and session_id are required",
			looppkg.ErrValidation,
		)
	}
	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return goal.SessionProjection{}, fmt.Errorf("store: begin session Goal projection read: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			joinCleanupError(&err, rollbackTx(tx, "session Goal projection read"))
		}
	}()

	projection, err = readSessionGoalProjection(ctx, tx, workspaceID, sessionID)
	if err != nil {
		return goal.SessionProjection{}, err
	}
	if err = tx.Commit(); err != nil {
		return goal.SessionProjection{}, fmt.Errorf("store: commit session Goal projection read: %w", err)
	}
	committed = true
	return projection, nil
}

func readSessionGoalProjection(
	ctx context.Context,
	tx *sql.Tx,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
) (goal.SessionProjection, error) {
	row, err := sqlcgen.New(tx).ReadGoalSessionProjection(ctx, sqlcgen.ReadGoalSessionProjectionParams{
		WorkspaceID: string(workspaceID), OriginSessionID: goalNullableString(sessionID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return goal.SessionProjection{}, nil
	}
	if err != nil {
		return goal.SessionProjection{}, fmt.Errorf("store: select newest session Goal: %w", err)
	}
	var projection goal.SessionProjection
	projection.Found = true
	projection.RunID = looppkg.RunID(strings.TrimSpace(row.ID))
	projection.RunStatus = looppkg.Status(strings.TrimSpace(row.Status))
	projection.DefinitionDigest = strings.TrimSpace(row.DefinitionDigest)
	projection.OriginSessionID = strings.TrimSpace(row.OriginSessionID.String)
	projection.ContextNudgeRatio = row.GoalContextNudgeRatio
	projection.Cleared = row.GoalClearedAt.Valid
	if !projection.RunStatus.Valid() {
		return goal.SessionProjection{}, fmt.Errorf(
			"%w: session Goal run status is invalid: %q",
			looppkg.ErrValidation,
			row.Status,
		)
	}
	if projection.Cleared {
		return projection, nil
	}

	projection.Checkpoint, err = readLatestSessionGoalCheckpoint(ctx, tx, workspaceID, projection.RunID)
	if err != nil {
		return goal.SessionProjection{}, err
	}
	projection.LastVerdict, err = readLatestSessionGoalVerdict(ctx, tx, workspaceID, projection.RunID)
	if err != nil {
		return goal.SessionProjection{}, err
	}
	return projection, nil
}

func readLatestSessionGoalCheckpoint(
	ctx context.Context,
	tx *sql.Tx,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (*goal.Checkpoint, error) {
	row, err := sqlcgen.New(tx).ReadLatestGoalSessionCheckpoint(ctx, string(runID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: select session Goal checkpoint: %w", err)
	}
	checkpoint, err := goalCheckpointFromGenerated(&row, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: decode session Goal checkpoint: %w", err)
	}
	return &checkpoint, nil
}

func readLatestSessionGoalVerdict(
	ctx context.Context,
	tx *sql.Tx,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (*goal.Turn, error) {
	row, err := sqlcgen.New(tx).ReadLatestGoalSessionVerdict(ctx, string(runID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: select session Goal verdict: %w", err)
	}
	verdict, err := goalTurnFromGenerated(row, workspaceID, runID)
	if err != nil {
		return nil, fmt.Errorf("store: decode session Goal verdict: %w", err)
	}
	return &verdict, nil
}

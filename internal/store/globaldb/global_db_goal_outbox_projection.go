package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func enqueueGoalStatusOutboxIfSessionOrigin(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.TurnKey,
	sourceRevision int64,
	at time.Time,
) error {
	originSessionID, sessionOrigin, err := loadGoalSessionOrigin(
		ctx,
		exec,
		key.WorkspaceID,
		key.LoopRunID,
	)
	if err != nil {
		return err
	}
	if !sessionOrigin {
		return nil
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, key)
	if err != nil {
		return err
	}
	boundSessionID := strings.TrimSpace(checkpoint.SessionID)
	if boundSessionID == "" {
		boundSessionID = originSessionID
	}
	return enqueueGoalProjectionOutboxForSessionOrigin(
		ctx,
		exec,
		originSessionID,
		key.WorkspaceID,
		key.LoopRunID,
		&boundSessionID,
		goal.SessionOutboxCauseStatus,
		strconv.FormatInt(sourceRevision, 10),
		at,
	)
}

func enqueueGoalReseedOutboxIfSessionOrigin(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.BindingKey,
	binding goal.SessionBinding,
	at time.Time,
) error {
	boundSessionID := strings.TrimSpace(binding.SessionID)
	return enqueueGoalProjectionOutboxIfSessionOrigin(
		ctx,
		exec,
		key.WorkspaceID,
		key.LoopRunID,
		&boundSessionID,
		goal.SessionOutboxCauseReseed,
		strconv.FormatInt(binding.BindingEpoch, 10),
		at,
	)
}

func enqueueGoalProjectionOutboxIfSessionOrigin(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	boundSessionID *string,
	cause goal.SessionOutboxCause,
	sourceRevision string,
	at time.Time,
) error {
	originSessionID, sessionOrigin, err := loadGoalSessionOrigin(
		ctx,
		exec,
		workspaceID,
		runID,
	)
	if err != nil {
		return err
	}
	if !sessionOrigin {
		return nil
	}
	return enqueueGoalProjectionOutboxForSessionOrigin(
		ctx,
		exec,
		originSessionID,
		workspaceID,
		runID,
		boundSessionID,
		cause,
		sourceRevision,
		at,
	)
}

func enqueueGoalProjectionOutboxForSessionOrigin(
	ctx context.Context,
	exec taskSQLExecutor,
	originSessionID string,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
	boundSessionID *string,
	cause goal.SessionOutboxCause,
	sourceRevision string,
	at time.Time,
) error {
	normalizedRevision := strings.TrimSpace(sourceRevision)
	if normalizedRevision == "" {
		return fmt.Errorf("%w: goal outbox source revision is required", looppkg.ErrValidation)
	}
	eventID := fmt.Sprintf(
		"goal-snapshot:%s:%s:%s:%s",
		originSessionID,
		runID,
		cause,
		normalizedRevision,
	)
	_, err := enqueueGoalSessionOutboxWithExecutor(ctx, exec, goal.EnqueueSessionOutboxRequest{
		EventID:         eventID,
		WorkspaceID:     workspaceID,
		OriginSessionID: originSessionID,
		LoopRunID:       runID,
		BoundSessionID:  boundSessionID,
		Cause:           cause,
		CreatedAt:       at,
	})
	return err
}

func loadGoalSessionOrigin(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (string, bool, error) {
	origin, err := sqlcgen.New(exec).GetGoalRunOrigin(ctx, sqlcgen.GetGoalRunOriginParams{
		ID: string(runID), WorkspaceID: string(workspaceID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("%w: %s", looppkg.ErrRunNotFound, runID)
	}
	if err != nil {
		return "", false, fmt.Errorf("store: load goal session origin %q: %w", runID, err)
	}
	if origin.OriginKind != goalRunOriginKindSession {
		return "", false, nil
	}
	normalizedOriginSessionID := strings.TrimSpace(origin.OriginSessionID.String)
	if !origin.OriginSessionID.Valid || normalizedOriginSessionID == "" {
		return "", false, fmt.Errorf(
			"%w: session-origin Goal run %q has no origin session",
			looppkg.ErrTransitionConflict,
			runID,
		)
	}
	return normalizedOriginSessionID, true, nil
}

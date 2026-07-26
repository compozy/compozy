package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func advanceRevokedGoalCheckpoint(
	ctx context.Context,
	exec taskSQLExecutor,
	checkpoint goal.Checkpoint,
	request goal.RevokePromptRequest,
) error {
	taskRunID := strings.TrimSpace(request.TaskRunID)
	queueEntryID := strings.TrimSpace(request.QueueEntryID)
	promptID := strings.TrimSpace(request.PromptID)
	if checkpoint.JudgeAttemptID != "" {
		if _, err := sqlcgen.New(exec).MarkGoalJudgeAttemptAmbiguous(
			ctx,
			sqlcgen.MarkGoalJudgeAttemptAmbiguousParams{
				CompletedAt: store.FormatTimestamp(request.RevokedAt), AttemptID: checkpoint.JudgeAttemptID,
				LoopRunID: string(request.Key.LoopRunID),
			},
		); err != nil {
			return fmt.Errorf("store: revoke running Goal judge attempt: %w", err)
		}
	}
	affected, err := sqlcgen.New(exec).AdvanceRevokedGoalCheckpoint(ctx, sqlcgen.AdvanceRevokedGoalCheckpointParams{
		GoalStatus:       request.Status,
		ControlCause:     goalNullableString(string(request.Cause)),
		ControlActorKind: goalNullableString(request.ActorKind),
		ControlActorID:   goalNullableString(request.ActorID),
		ControlRequestedAt: store.FormatTimestamp(
			request.RevokedAt,
		),
		UpdatedAt:    store.FormatTimestamp(request.RevokedAt),
		LoopRunID:    string(request.Key.LoopRunID),
		Generation:   int64(request.Key.Generation),
		NodeID:       string(request.Key.NodeID),
		ItemIndex:    int64(request.Key.ItemIndex),
		ControlEpoch: request.ExpectedControlEpoch,
		BindingEpoch: goalNullableInt64(&request.ExpectedBindingEpoch),
		TaskRunID:    goalNullableString(taskRunID),
		QueueEntryID: goalNullableString(queueEntryID),
		PromptID:     goalNullableString(promptID),
	})
	if err != nil {
		return fmt.Errorf("store: advance revoked Goal checkpoint: %w", err)
	}
	return requireGoalAffectedCount(affected, "advance revoked Goal checkpoint")
}

func markGoalRunCleared(
	ctx context.Context,
	exec taskSQLExecutor,
	request goal.RevokePromptRequest,
) error {
	if request.ProjectionCause != goal.SessionOutboxCauseClear {
		return nil
	}
	affected, err := sqlcgen.New(exec).MarkGoalRunCleared(ctx, sqlcgen.MarkGoalRunClearedParams{
		GoalClearedAt: store.FormatTimestamp(request.RevokedAt), ID: string(request.Key.LoopRunID),
		WorkspaceID: string(request.Key.WorkspaceID),
	})
	if err != nil {
		return fmt.Errorf("store: mark Goal run cleared: %w", err)
	}
	return requireGoalAffectedCount(affected, "mark Goal run cleared")
}

func enqueueRevokedGoalProjection(
	ctx context.Context,
	exec taskSQLExecutor,
	checkpoint goal.Checkpoint,
	request goal.RevokePromptRequest,
) error {
	var boundSessionID *string
	if request.ProjectionCause != goal.SessionOutboxCauseClear {
		value := checkpoint.SessionID
		boundSessionID = &value
	}
	return enqueueGoalProjectionOutboxIfSessionOrigin(
		ctx,
		exec,
		request.Key.WorkspaceID,
		request.Key.LoopRunID,
		boundSessionID,
		request.ProjectionCause,
		fmt.Sprintf("%d", request.ExpectedControlEpoch+1),
		request.RevokedAt,
	)
}

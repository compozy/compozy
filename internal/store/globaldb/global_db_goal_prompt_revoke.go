package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
)

// RevokeGoalPrompt atomically fences prepared work or terminalizes one already-claimed operation.
func (g *GoalRepo) RevokeGoalPrompt(
	ctx context.Context,
	request goal.RevokePromptRequest,
) (goal.Checkpoint, error) {
	if err := g.checkReady(ctx, "revoke Goal prompt"); err != nil {
		return goal.Checkpoint{}, err
	}
	if request.RevokedAt.IsZero() {
		request.RevokedAt = g.now().UTC()
	} else {
		request.RevokedAt = request.RevokedAt.UTC()
	}
	if err := validateRevokeGoalPromptRequest(request); err != nil {
		return goal.Checkpoint{}, err
	}

	var revoked goal.Checkpoint
	err := g.withTaskImmediateTransaction(ctx, "revoke Goal prompt", func(exec taskSQLExecutor) error {
		var err error
		revoked, err = revokeGoalPromptWithExecutor(ctx, exec, request)
		return err
	})
	if err != nil {
		return goal.Checkpoint{}, err
	}
	return revoked, nil
}

func revokeGoalPromptWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	request goal.RevokePromptRequest,
) (goal.Checkpoint, error) {
	return revokeGoalPromptWithExecutorOptions(ctx, exec, request, true)
}

func revokeGoalPromptWithExecutorOptions(
	ctx context.Context,
	exec taskSQLExecutor,
	request goal.RevokePromptRequest,
	enqueueProjection bool,
) (goal.Checkpoint, error) {
	checkpoint, row, err := loadRevokeGoalPromptOwner(ctx, exec, request)
	if err != nil {
		return goal.Checkpoint{}, err
	}
	turnTerminalized, err := revokeGoalPromptEvidence(ctx, exec, checkpoint, &row, request)
	if err != nil {
		return goal.Checkpoint{}, err
	}
	if err := closeRevokedGoalBinding(ctx, exec, checkpoint, request); err != nil {
		return goal.Checkpoint{}, err
	}
	if err := advanceRevokedGoalCheckpoint(ctx, exec, checkpoint, request); err != nil {
		return goal.Checkpoint{}, err
	}
	if err := projectGoalCheckpointCounts(
		ctx,
		exec,
		request.Key,
		request.Status,
		checkpoint.TurnsUsed,
		checkpoint.TurnLimit,
	); err != nil {
		return goal.Checkpoint{}, err
	}
	if turnTerminalized {
		if err := appendGoalTurnCompletedEvent(
			ctx,
			exec,
			request.Key,
			request.PromptID,
			request.RevokedAt,
		); err != nil {
			return goal.Checkpoint{}, err
		}
	}
	if _, _, err := appendGoalStatusChangedRunEvent(
		ctx,
		exec,
		request.Key,
		checkpoint.Status,
		request.Status,
		request.Cause,
		request.ActorKind,
		request.ActorID,
		request.RevokedAt,
	); err != nil {
		return goal.Checkpoint{}, err
	}
	if err := markGoalRunCleared(ctx, exec, request); err != nil {
		return goal.Checkpoint{}, err
	}
	if enqueueProjection {
		if err := enqueueRevokedGoalProjection(ctx, exec, checkpoint, request); err != nil {
			return goal.Checkpoint{}, err
		}
	}
	return loadGoalCheckpointWithExecutor(ctx, exec, request.Key)
}

func validateRevokeGoalPromptRequest(request goal.RevokePromptRequest) error {
	if err := request.Key.Validate(); err != nil {
		return err
	}
	if request.ExpectedControlEpoch < 1 || request.ExpectedBindingEpoch < 1 ||
		strings.TrimSpace(request.TaskRunID) == "" || strings.TrimSpace(request.QueueEntryID) == "" ||
		strings.TrimSpace(request.PromptID) == "" || !request.Disposition.Valid() ||
		!goalStatusValid(request.Status) || strings.TrimSpace(string(request.Cause)) == "" ||
		strings.TrimSpace(request.ActorKind) == "" || strings.TrimSpace(request.ActorID) == "" ||
		!request.ProjectionCause.Valid() || request.ProjectionCause == goal.SessionOutboxCauseStart ||
		request.ProjectionCause == goal.SessionOutboxCauseReseed || request.RevokedAt.IsZero() {
		return fmt.Errorf("%w: Goal prompt revocation identity is incomplete", looppkg.ErrValidation)
	}
	return nil
}

func loadRevokeGoalPromptOwner(
	ctx context.Context,
	exec taskSQLExecutor,
	request goal.RevokePromptRequest,
) (goal.Checkpoint, goalPromptRow, error) {
	if err := validateGoalRunWorkspace(ctx, exec, request.Key); err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, request.Key)
	if err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	if checkpoint.ControlEpoch != request.ExpectedControlEpoch ||
		checkpoint.BindingEpoch != request.ExpectedBindingEpoch ||
		checkpoint.Status != goalStatusActive || checkpoint.TaskRunID != strings.TrimSpace(request.TaskRunID) ||
		checkpoint.QueueEntryID != strings.TrimSpace(request.QueueEntryID) ||
		checkpoint.PromptID != strings.TrimSpace(request.PromptID) ||
		!revocableGoalCheckpointPhase(checkpoint.Phase) {
		return goal.Checkpoint{}, goalPromptRow{}, goalControlStaleError("Goal revoke checkpoint owner changed")
	}
	row, err := loadGoalPromptRow(ctx, exec, request.Key.LoopRunID, request.PromptID)
	if err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	if err := requireGoalPromptOwner(
		&row,
		request.Key,
		request.TaskRunID,
		request.QueueEntryID,
		request.ExpectedBindingEpoch,
	); err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	if !row.ownerEpoch.Valid || row.ownerEpoch.Int64 != request.ExpectedControlEpoch ||
		!row.promptID.Valid || row.promptID.String != strings.TrimSpace(request.PromptID) ||
		row.sessionID != checkpoint.SessionID {
		return goal.Checkpoint{}, goalPromptRow{}, goalControlStaleError("Goal revoke queue owner changed")
	}
	if err := validateActiveGoalPromptBinding(
		ctx,
		exec,
		request.Key,
		checkpoint.BindingHandle,
		request.ExpectedBindingEpoch,
		checkpoint.SessionID,
	); err != nil {
		return goal.Checkpoint{}, goalPromptRow{}, err
	}
	return checkpoint, row, nil
}

func revocableGoalCheckpointPhase(phase string) bool {
	switch phase {
	case goalCheckpointPhaseQueued,
		goalCheckpointPhasePrompting,
		goalCheckpointPhaseCompacting,
		goalCheckpointPhaseJudging,
		goalCheckpointPhasePersisting:
		return true
	default:
		return false
	}
}

func closeRevokedGoalBinding(
	ctx context.Context,
	exec taskSQLExecutor,
	checkpoint goal.Checkpoint,
	request goal.RevokePromptRequest,
) error {
	return closeGoalBindingWithCleanup(
		ctx,
		exec,
		goal.BindingKey{
			WorkspaceID: request.Key.WorkspaceID,
			LoopRunID:   request.Key.LoopRunID,
			Handle:      checkpoint.BindingHandle,
		},
		request.ExpectedBindingEpoch,
		checkpoint.SessionID,
		goal.SessionCleanupCauseControlRevoked,
		request.RevokedAt,
	)
}

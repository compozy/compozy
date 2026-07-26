package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
)

var _ looppkg.GoalRunStopStore = (*GoalRepo)(nil)

// StopGoalRun atomically revokes every live Goal checkpoint before failing its owning Run.
func (g *GoalRepo) StopGoalRun(
	ctx context.Context,
	request looppkg.GoalRunStopRequest,
) (looppkg.GoalRunStopResult, error) {
	if err := g.checkReady(ctx, "stop Goal run"); err != nil {
		return looppkg.GoalRunStopResult{}, err
	}
	if request.StoppedAt.IsZero() {
		request.StoppedAt = g.now().UTC()
	} else {
		request.StoppedAt = request.StoppedAt.UTC()
	}
	if err := validateGoalRunStopRequest(request); err != nil {
		return looppkg.GoalRunStopResult{}, err
	}

	result := looppkg.GoalRunStopResult{RevokedPromptLeases: make([]looppkg.GoalPromptLease, 0)}
	err := g.withTaskImmediateTransaction(ctx, "stop Goal run", func(exec taskSQLExecutor) error {
		run, err := getLoopRunByIDWithExecutor(ctx, exec, request.RunID)
		if err != nil {
			return err
		}
		if run.WorkspaceID != request.WorkspaceID {
			return fmt.Errorf("%w: %s", looppkg.ErrRunNotFound, request.RunID)
		}
		if run.Status != request.ExpectedStatus {
			return fmt.Errorf(
				"%w: run_id=%s from=%s to=%s",
				looppkg.ErrTransitionConflict,
				request.RunID,
				request.ExpectedStatus,
				looppkg.StatusFailed,
			)
		}
		leases, err := stopGoalCheckpointsWithExecutor(ctx, exec, request, true)
		if err != nil {
			return err
		}
		if err := closeStoppedGoalBindings(ctx, exec, request); err != nil {
			return err
		}
		if err := compareAndSwapLoopRunStatusWithExecutor(
			ctx,
			exec,
			request.RunID,
			request.ExpectedStatus,
			looppkg.StatusFailed,
			looppkg.TransitionCauseOperatorStop,
			request.StoppedAt,
		); err != nil {
			return err
		}
		result.RevokedPromptLeases = leases
		return nil
	})
	if err != nil {
		return looppkg.GoalRunStopResult{}, err
	}
	return result, nil
}

func validateGoalRunStopRequest(request looppkg.GoalRunStopRequest) error {
	if strings.TrimSpace(string(request.WorkspaceID)) == "" || strings.TrimSpace(string(request.RunID)) == "" ||
		!request.ExpectedStatus.Valid() || request.ExpectedStatus.Terminal() || request.StoppedAt.IsZero() {
		return fmt.Errorf("%w: Goal Run stop identity is incomplete", looppkg.ErrValidation)
	}
	if err := request.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: Goal Run stop actor: %w", looppkg.ErrValidation, err)
	}
	return nil
}

func stoppedGoalRevokeRequest(
	request looppkg.GoalRunStopRequest,
	checkpoint goal.Checkpoint,
) goal.RevokePromptRequest {
	return goal.RevokePromptRequest{
		Key:                  checkpoint.Key,
		ExpectedControlEpoch: checkpoint.ControlEpoch,
		ExpectedBindingEpoch: checkpoint.BindingEpoch,
		TaskRunID:            checkpoint.TaskRunID,
		QueueEntryID:         checkpoint.QueueEntryID,
		PromptID:             checkpoint.PromptID,
		Disposition:          looppkg.ActionDispositionPaused,
		Status:               goalStatusPaused,
		Cause:                looppkg.ReasonCodeGoalControlRevokedInFlight,
		ActorKind:            string(request.Actor.Actor.Kind.Normalize()),
		ActorID:              strings.TrimSpace(request.Actor.Actor.Ref),
		ProjectionCause:      goal.SessionOutboxCauseStatus,
		RevokedAt:            request.StoppedAt,
	}
}

func goalPromptLease(checkpoint goal.Checkpoint) looppkg.GoalPromptLease {
	return looppkg.GoalPromptLease{
		QueueEntryID:   checkpoint.QueueEntryID,
		SessionID:      checkpoint.SessionID,
		OwnerKind:      "goal",
		LoopRunID:      string(checkpoint.Key.LoopRunID),
		TaskRunID:      checkpoint.TaskRunID,
		RunGeneration:  checkpoint.Key.Generation,
		PromptAttempt:  checkpoint.PromptAttempt,
		ControlEpoch:   checkpoint.ControlEpoch,
		BindingEpoch:   checkpoint.BindingEpoch,
		PromptID:       checkpoint.PromptID,
		PromptKind:     checkpoint.PromptKind,
		JudgeAttemptID: checkpoint.JudgeAttemptID,
	}
}

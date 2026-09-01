package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type stoppableGoalCheckpointKey struct {
	generation int
	nodeID     looppkg.NodeID
	itemIndex  int
}

type goalCancellationRequest struct {
	WorkspaceID looppkg.WorkspaceID
	RunID       looppkg.RunID
	Actor       taskpkg.ActorContext
	CanceledAt  time.Time
}

func cancelGoalCheckpointsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	request goalCancellationRequest,
	enqueueProjection bool,
) ([]looppkg.GoalPromptLease, error) {
	keys, err := loadStoppableGoalCheckpointKeys(ctx, exec, request)
	if err != nil {
		return nil, err
	}
	leases := make([]looppkg.GoalPromptLease, 0, len(keys))
	for _, key := range keys {
		checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, goal.TurnKey{
			WorkspaceID: request.WorkspaceID,
			LoopRunID:   request.RunID,
			Generation:  key.generation,
			NodeID:      key.nodeID,
			ItemIndex:   key.itemIndex,
		})
		if err != nil {
			return nil, err
		}
		if revocableGoalCheckpointPhase(checkpoint.Phase) {
			revokeRequest := canceledGoalRevokeRequest(request, checkpoint)
			if _, err := revokeGoalPromptWithExecutorOptions(
				ctx,
				exec,
				revokeRequest,
				enqueueProjection,
			); err != nil {
				return nil, err
			}
			lease := goalPromptLease(checkpoint)
			if !validGoalPromptLease(lease) {
				return nil, fmt.Errorf("%w: stopped Goal prompt lease is incomplete", looppkg.ErrTransitionConflict)
			}
			leases = append(leases, lease)
			continue
		}
		if err := terminalizeCanceledGoalCheckpoint(
			ctx,
			exec,
			request,
			checkpoint,
			enqueueProjection,
		); err != nil {
			return nil, err
		}
	}
	return leases, nil
}

func loadStoppableGoalCheckpointKeys(
	ctx context.Context,
	exec taskSQLExecutor,
	request goalCancellationRequest,
) ([]stoppableGoalCheckpointKey, error) {
	rows, err := sqlcgen.New(exec).ListStoppableGoalCheckpointKeys(ctx, string(request.RunID))
	if err != nil {
		return nil, fmt.Errorf("store: list stoppable Goal checkpoints: %w", err)
	}
	keys := make([]stoppableGoalCheckpointKey, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, stoppableGoalCheckpointKey{
			generation: int(row.Generation), nodeID: looppkg.NodeID(row.NodeID), itemIndex: int(row.ItemIndex),
		})
	}
	return keys, nil
}

func terminalizeCanceledGoalCheckpoint(
	ctx context.Context,
	exec taskSQLExecutor,
	request goalCancellationRequest,
	checkpoint goal.Checkpoint,
	enqueueProjection bool,
) error {
	revoke := canceledGoalRevokeRequest(request, checkpoint)
	affected, err := sqlcgen.New(exec).
		TerminalizeStoppedGoalCheckpoint(ctx, sqlcgen.TerminalizeStoppedGoalCheckpointParams{
			ControlCause:     goalNullableString(string(revoke.Cause)),
			ControlActorKind: goalNullableString(revoke.ActorKind),
			ControlActorID: goalNullableString(
				revoke.ActorID,
			),
			ControlRequestedAt: store.FormatTimestamp(request.CanceledAt),
			UpdatedAt:          store.FormatTimestamp(request.CanceledAt),
			LoopRunID:          string(request.RunID),
			Generation:         int64(checkpoint.Key.Generation),
			NodeID:             string(checkpoint.Key.NodeID),
			ItemIndex:          int64(checkpoint.Key.ItemIndex),
			ControlEpoch:       checkpoint.ControlEpoch,
			ExpectedPhase:      checkpoint.Phase,
		})
	if err != nil {
		return fmt.Errorf("store: terminalize stopped Goal checkpoint: %w", err)
	}
	if err := requireGoalAffectedCount(affected, "terminalize stopped Goal checkpoint"); err != nil {
		return err
	}
	if err := projectGoalCheckpointCounts(
		ctx,
		exec,
		checkpoint.Key,
		goalStatusPaused,
		checkpoint.TurnsUsed,
		checkpoint.TurnLimit,
	); err != nil {
		return err
	}
	if _, _, err := appendGoalStatusChangedRunEvent(
		ctx,
		exec,
		checkpoint.Key,
		checkpoint.Status,
		goalStatusPaused,
		revoke.Cause,
		revoke.ActorKind,
		revoke.ActorID,
		request.CanceledAt,
	); err != nil {
		return err
	}
	if !enqueueProjection {
		return nil
	}
	return enqueueRevokedGoalProjection(ctx, exec, checkpoint, revoke)
}

func closeCanceledGoalBindings(
	ctx context.Context,
	exec taskSQLExecutor,
	request goalCancellationRequest,
) error {
	rows, err := sqlcgen.New(exec).ListActiveGoalBindingsForStop(ctx, sqlcgen.ListActiveGoalBindingsForStopParams{
		LoopRunID: string(request.RunID), WorkspaceID: string(request.WorkspaceID),
	})
	if err != nil {
		return fmt.Errorf("store: list stopped Goal bindings: %w", err)
	}
	for _, binding := range rows {
		if err := closeGoalBindingWithCleanup(
			ctx,
			exec,
			goal.BindingKey{
				WorkspaceID: request.WorkspaceID,
				LoopRunID:   request.RunID,
				Handle:      binding.Handle,
			},
			binding.BindingEpoch,
			binding.SessionID,
			goal.SessionCleanupCauseStop,
			request.CanceledAt,
		); err != nil {
			return err
		}
	}
	return failCanceledCreatingGoalBindings(ctx, exec, request)
}

func failCanceledCreatingGoalBindings(
	ctx context.Context,
	exec taskSQLExecutor,
	request goalCancellationRequest,
) error {
	rows, err := sqlcgen.New(exec).ListCreatingGoalBindingsForStop(ctx, sqlcgen.ListCreatingGoalBindingsForStopParams{
		LoopRunID: string(request.RunID), WorkspaceID: string(request.WorkspaceID),
	})
	if err != nil {
		return fmt.Errorf("store: list stopped creating Goal bindings: %w", err)
	}
	for _, candidate := range rows {
		key := goal.BindingKey{WorkspaceID: request.WorkspaceID, LoopRunID: request.RunID, Handle: candidate.Handle}
		binding, err := getSessionBindingAttemptWithExecutor(ctx, exec, key, candidate.BindingEpoch)
		if err != nil {
			return err
		}
		terminalAt := request.CanceledAt
		if terminalAt.Before(binding.CreatedAt) {
			terminalAt = binding.CreatedAt
		}
		affected, err := sqlcgen.New(exec).
			FailGoalBindingCreationAttempt(ctx, sqlcgen.FailGoalBindingCreationAttemptParams{
				FailureCode: goalNullableString(goalBindingFailureStopCreationUnsettled),
				FailedAt:    store.FormatTimestamp(terminalAt), LoopRunID: string(request.RunID),
				Handle: candidate.Handle, BindingEpoch: candidate.BindingEpoch,
			})
		if err != nil {
			return fmt.Errorf("store: fail stopped creating Goal binding: %w", err)
		}
		if err := requireGoalAffectedCount(affected, "fail stopped creating Goal binding"); err != nil {
			return err
		}
		if err := enqueueGoalSessionCleanupWithExecutor(
			ctx, exec, binding, goal.SessionCleanupCauseStop, terminalAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func canceledGoalRevokeRequest(
	request goalCancellationRequest,
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
		RevokedAt:            request.CanceledAt,
	}
}

func goalPromptLease(checkpoint goal.Checkpoint) looppkg.GoalPromptLease {
	return looppkg.GoalPromptLease{
		QueueEntryID: checkpoint.QueueEntryID, SessionID: checkpoint.SessionID, OwnerKind: "goal",
		LoopRunID: string(checkpoint.Key.LoopRunID), TaskRunID: checkpoint.TaskRunID,
		RunGeneration: int64(checkpoint.Key.Generation), PromptAttempt: checkpoint.PromptAttempt,
		ControlEpoch: checkpoint.ControlEpoch, BindingEpoch: checkpoint.BindingEpoch,
		PromptID: checkpoint.PromptID, PromptKind: checkpoint.PromptKind, JudgeAttemptID: checkpoint.JudgeAttemptID,
	}
}

func validGoalPromptLease(lease looppkg.GoalPromptLease) bool {
	return strings.TrimSpace(lease.QueueEntryID) != "" && strings.TrimSpace(lease.SessionID) != "" &&
		lease.OwnerKind == "goal" && strings.TrimSpace(lease.LoopRunID) != "" &&
		strings.TrimSpace(lease.TaskRunID) != "" && lease.RunGeneration > 0 &&
		lease.PromptAttempt >= 0 && lease.ControlEpoch > 0 && lease.BindingEpoch > 0 &&
		strings.TrimSpace(lease.PromptID) != "" && strings.TrimSpace(lease.PromptKind) != ""
}

package globaldb

import (
	"context"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

type stoppableGoalCheckpointKey struct {
	generation int
	nodeID     looppkg.NodeID
	itemIndex  int
}

func stopGoalCheckpointsWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.GoalRunStopRequest,
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
			revokeRequest := stoppedGoalRevokeRequest(request, checkpoint)
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
		if err := terminalizeStoppedGoalCheckpoint(
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
	request looppkg.GoalRunStopRequest,
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

func terminalizeStoppedGoalCheckpoint(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.GoalRunStopRequest,
	checkpoint goal.Checkpoint,
	enqueueProjection bool,
) error {
	revoke := stoppedGoalRevokeRequest(request, checkpoint)
	affected, err := sqlcgen.New(exec).
		TerminalizeStoppedGoalCheckpoint(ctx, sqlcgen.TerminalizeStoppedGoalCheckpointParams{
			ControlCause:     goalNullableString(string(revoke.Cause)),
			ControlActorKind: goalNullableString(revoke.ActorKind),
			ControlActorID: goalNullableString(
				revoke.ActorID,
			),
			ControlRequestedAt: store.FormatTimestamp(request.StoppedAt),
			UpdatedAt:          store.FormatTimestamp(request.StoppedAt),
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
		request.StoppedAt,
	); err != nil {
		return err
	}
	if !enqueueProjection {
		return nil
	}
	return enqueueRevokedGoalProjection(ctx, exec, checkpoint, revoke)
}

func closeStoppedGoalBindings(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.GoalRunStopRequest,
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
			request.StoppedAt,
		); err != nil {
			return err
		}
	}
	return failStoppedCreatingGoalBindings(ctx, exec, request)
}

func failStoppedCreatingGoalBindings(
	ctx context.Context,
	exec taskSQLExecutor,
	request looppkg.GoalRunStopRequest,
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
		affected, err := sqlcgen.New(exec).
			FailGoalBindingCreationAttempt(ctx, sqlcgen.FailGoalBindingCreationAttemptParams{
				FailureCode: goalNullableString(goalBindingFailureStopCreationUnsettled),
				FailedAt:    store.FormatTimestamp(request.StoppedAt), LoopRunID: string(request.RunID),
				Handle: candidate.Handle, BindingEpoch: candidate.BindingEpoch,
			})
		if err != nil {
			return fmt.Errorf("store: fail stopped creating Goal binding: %w", err)
		}
		if err := requireGoalAffectedCount(affected, "fail stopped creating Goal binding"); err != nil {
			return err
		}
		if err := enqueueGoalSessionCleanupWithExecutor(
			ctx, exec, binding, goal.SessionCleanupCauseStop, request.StoppedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func validGoalPromptLease(lease looppkg.GoalPromptLease) bool {
	return strings.TrimSpace(lease.QueueEntryID) != "" && strings.TrimSpace(lease.SessionID) != "" &&
		lease.OwnerKind == "goal" && strings.TrimSpace(lease.LoopRunID) != "" &&
		strings.TrimSpace(lease.TaskRunID) != "" && lease.RunGeneration > 0 &&
		lease.PromptAttempt >= 0 && lease.ControlEpoch > 0 && lease.BindingEpoch > 0 &&
		strings.TrimSpace(lease.PromptID) != "" && strings.TrimSpace(lease.PromptKind) != ""
}

package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func (g *TaskRunRepo) recoverNetworkWakeOnBootWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation taskpkg.NetworkWakeBootRecoveryMutation,
) (taskpkg.NominalRunMutationResult, error) {
	actor := mutation.Actor()
	if err := actor.Validate(); err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	if actor.Actor.Kind.Normalize() != taskpkg.ActorKindDaemon {
		return taskpkg.NominalRunMutationResult{}, taskpkg.ErrPermissionDenied
	}
	recovery := mutation.Recovery()
	if err := recovery.Validate("network_wake_boot_recovery"); err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	now := mutation.Now().UTC()
	if now.IsZero() {
		return taskpkg.NominalRunMutationResult{}, fmt.Errorf(
			"%w: network wake recovery timestamp is required",
			taskpkg.ErrValidation,
		)
	}
	previous, err := g.requireFencedTaskRun(ctx, exec, mutation.Fence(), false)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	result, err := g.applyNetworkWakeBootRecovery(ctx, exec, previous, mutation, now)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	wakeID, targetSessionID, ownerKey := result.Run.NetworkWakeCorrelation()
	if err := appendNetworkWakeEventWithExecutor(ctx, exec, networkWakeEvent{
		workspaceID: result.Run.WorkspaceID, wakeID: wakeID, taskRunID: result.Run.ID,
		ownerKey: ownerKey, targetSessionID: targetSessionID,
		eventType: networkWakeEventRecovered, state: result.Run.Status.String(),
		claimTokenHash: previous.ClaimTokenHash, reason: recovery.Reason,
		actor: actor.Actor, at: now,
	}); err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	return result, nil
}

func (g *TaskRunRepo) applyNetworkWakeBootRecovery(
	ctx context.Context,
	exec taskSQLExecutor,
	previous taskpkg.Run,
	mutation taskpkg.NetworkWakeBootRecoveryMutation,
	now time.Time,
) (taskpkg.NominalRunMutationResult, error) {
	action := mutation.Recovery().Action.Normalize()
	if action == taskpkg.RunBootRecoveryFail {
		if previous.Status.Normalize() != taskpkg.TaskRunStatusStarting &&
			previous.Status.Normalize() != taskpkg.TaskRunStatusRunning {
			return taskpkg.NominalRunMutationResult{}, invalidNominalRunTransition(previous, "boot failure")
		}
		next := previous
		next.Status = taskpkg.TaskRunStatusFailed
		next.Error = strings.TrimSpace(mutation.FailureText())
		next.Result = nil
		next.LeaseUntil = time.Time{}
		next.HeartbeatAt = time.Time{}
		next.EndedAt = now
		updated, err := g.tasks.transitionTerminalRunWithExecutor(
			ctx,
			exec,
			taskpkg.NewTerminalRunMutation(previous, next),
		)
		if err != nil {
			return taskpkg.NominalRunMutationResult{}, err
		}
		return taskpkg.NominalRunMutationResult{Previous: previous, Run: updated}, nil
	}
	startedAt := previous.StartedAt
	if startedAt.IsZero() {
		startedAt = now
	}
	affected, err := applyBootRecoveryNominalWrite(ctx, exec, mutation.Fence(), action, startedAt)
	if err != nil {
		return taskpkg.NominalRunMutationResult{}, err
	}
	return g.finishNominalTaskRunMutation(ctx, exec, previous, affected)
}

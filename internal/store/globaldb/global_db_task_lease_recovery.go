package globaldb

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
	taskpkg "github.com/compozy/agh/internal/task"
)

const expiredLeaseRecoveryActorRef = "lease-recovery"

func (g *TaskRunRepo) recoverExpiredLeaseWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	snapshot taskRunLeaseSnapshot,
	recovery taskpkg.ExpiredLeaseRecovery,
) (bool, error) {
	if run.IsNetworkWake() {
		return false, requeueExpiredLease(ctx, exec, run, snapshot, 0)
	}
	taskRecord, err := g.tasks.getTaskWithExecutor(ctx, exec, run.TaskID)
	if err != nil {
		return false, err
	}
	maxAttempts := normalizeStoredTaskMaxAttempts(taskRecord.MaxAttempts)
	if int(run.Attempt)+int(run.RecoveryCount) < maxAttempts {
		return false, requeueExpiredLease(ctx, exec, run, snapshot, 1)
	}
	if err := exhaustExpiredLease(ctx, exec, run, snapshot, recovery.Now); err != nil {
		return false, err
	}
	actor := expiredLeaseRecoveryActor(recovery.Actor)
	escalated, changed, err := g.tasks.markTaskNeedsAttentionIfClearWithExecutor(
		ctx,
		exec,
		taskpkg.NeedsAttentionMutation{
			TaskID:   run.TaskID,
			Reason:   taskpkg.LeaseRecoveryExhaustedReason,
			Actor:    actor.Actor,
			MarkedAt: recovery.Now,
			Origin:   actor.Origin,
		},
	)
	if err != nil {
		return false, err
	}
	if changed && escalated.NeedsAttention != nil {
		if err := appendTaskEventPayloadWithExecutor(
			ctx,
			exec,
			run.TaskID,
			run.ID,
			string(hookspkg.HookTaskNeedsAttention),
			actor,
			recovery.Now,
			taskAttentionWatchEventPayload{
				Status: taskpkg.TaskStatusNeedsAttention,
				Reason: taskpkg.LeaseRecoveryExhaustedReason,
				At:     recovery.Now,
			},
		); err != nil {
			return false, err
		}
	}
	return true, nil
}

func expiredLeaseRecoveryActor(actor taskpkg.ActorContext) taskpkg.ActorContext {
	if actor.Validate() == nil {
		return actor
	}
	return taskpkg.ActorContext{
		Actor: taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindDaemon,
			Ref:  expiredLeaseRecoveryActorRef,
		},
		Origin: taskpkg.Origin{
			Kind: taskpkg.OriginKindDaemon,
			Ref:  expiredLeaseRecoveryActorRef,
		},
		Scope: taskpkg.CallerScope{},
	}
}

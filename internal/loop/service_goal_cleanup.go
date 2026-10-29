package loop

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/task"
)

func (s *service) revokeGoalPromptLeases(
	ctx context.Context,
	leases []GoalPromptLease,
	cause TransitionCause,
) {
	if s.goalLeaseRevoker == nil {
		return
	}
	revocationCtx, cancel := loopPostCommitContext(ctx)
	defer cancel()
	for _, lease := range leases {
		if err := s.goalLeaseRevoker.RevokeGoalPromptLease(
			revocationCtx,
			lease,
			string(cause),
		); err != nil {
			s.logger.ErrorContext(
				revocationCtx,
				"post-commit Goal runtime revocation failed",
				"cause", cause,
				"loop_run_id", lease.LoopRunID,
				"task_run_id", lease.TaskRunID,
				"judge_attempt_id", lease.JudgeAttemptID,
				"error", err,
			)
		}
	}
}

func (s *service) pauseSessionGoal(ctx context.Context, run Run, actor task.ActorContext) error {
	if run.Status == StatusPaused || run.Status == StatusNeedsApproval {
		return nil
	}
	if run.Status != StatusRunning {
		return reasonError(ReasonCodeInvalidStatusTransition, ErrInvalidTransition,
			map[string]string{reasonMetaFrom: string(run.Status), reasonMetaTo: string(StatusPaused)})
	}
	controls, ok := s.store.(GoalPauseStore)
	if !ok {
		return fmt.Errorf("%w: Goal pause store is unavailable", ErrActionDependencyMissing)
	}
	leases, err := controls.RequestGoalPause(ctx, run.WorkspaceID, run.ID, actor)
	if err != nil {
		return err
	}
	if len(leases) == 0 {
		return nil
	}
	if s.goalLeaseRevoker == nil {
		return fmt.Errorf("%w: Goal prompt interrupter is unavailable", ErrActionDependencyMissing)
	}
	interruptCtx, cancel := loopPostCommitContext(ctx)
	defer cancel()
	var errs []error
	for _, lease := range leases {
		errs = append(
			errs,
			s.goalLeaseRevoker.RevokeGoalPromptLease(interruptCtx, lease, string(TransitionCausePauseBoundary)),
		)
	}
	return errors.Join(errs...)
}

package loop

import "context"

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

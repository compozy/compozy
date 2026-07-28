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
	for _, lease := range leases {
		if err := s.goalLeaseRevoker.RevokeGoalPromptLease(ctx, lease, string(cause)); err != nil {
			s.logger.ErrorContext(
				ctx,
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

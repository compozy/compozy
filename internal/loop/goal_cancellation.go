package loop

import "context"

// GoalPromptLease is the neutral identity required to revoke one managed Goal prompt lease.
type GoalPromptLease struct {
	QueueEntryID   string
	SessionID      string
	OwnerKind      string
	LoopRunID      string
	TaskRunID      string
	RunGeneration  int64
	PromptAttempt  int
	ControlEpoch   int64
	BindingEpoch   int64
	PromptID       string
	PromptKind     string
	JudgeAttemptID string
}

// GoalPromptLeaseRevoker cancels one exact in-memory prompt lease after durable revocation commits.
type GoalPromptLeaseRevoker interface {
	RevokeGoalPromptLease(context.Context, GoalPromptLease, string) error
}

// GoalPromptLeaseRevokerFunc adapts a function to GoalPromptLeaseRevoker.
type GoalPromptLeaseRevokerFunc func(context.Context, GoalPromptLease, string) error

// RevokeGoalPromptLease implements GoalPromptLeaseRevoker.
func (f GoalPromptLeaseRevokerFunc) RevokeGoalPromptLease(
	ctx context.Context,
	lease GoalPromptLease,
	reason string,
) error {
	if f == nil {
		return nil
	}
	return f(ctx, lease, reason)
}

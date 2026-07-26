package loop

import (
	"context"
	"time"

	"github.com/compozy/agh/internal/task"
)

// GoalPromptLease is the neutral identity required to revoke one managed Goal prompt lease.
type GoalPromptLease struct {
	QueueEntryID   string
	SessionID      string
	OwnerKind      string
	LoopRunID      string
	TaskRunID      string
	RunGeneration  int
	PromptAttempt  int
	ControlEpoch   int64
	BindingEpoch   int64
	PromptID       string
	PromptKind     string
	JudgeAttemptID string
}

// GoalRunStopRequest atomically revokes Goal state and transitions its owning Run.
type GoalRunStopRequest struct {
	WorkspaceID    WorkspaceID
	RunID          RunID
	ExpectedStatus Status
	Actor          task.ActorContext
	StoppedAt      time.Time
}

// GoalRunStopResult carries exact in-memory leases that may be canceled after commit.
type GoalRunStopResult struct {
	RevokedPromptLeases []GoalPromptLease
}

// GoalRunStopStore is the optional atomic Goal-aware Run stop extension.
type GoalRunStopStore interface {
	StopGoalRun(context.Context, GoalRunStopRequest) (GoalRunStopResult, error)
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

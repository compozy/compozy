package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/session"
)

type loopManagedInputLeaseRevoker interface {
	RevokeManagedInput(session.ManagedInputOwner, string)
}

type loopGoalPromptLeaseRevoker struct {
	sessions loopManagedInputLeaseRevoker
	judges   *loopGateJudgeRunner
}

var _ looppkg.GoalPromptLeaseRevoker = loopGoalPromptLeaseRevoker{}

func (r loopGoalPromptLeaseRevoker) RevokeGoalPromptLease(
	ctx context.Context,
	lease looppkg.GoalPromptLease,
	reason string,
) error {
	if ctx == nil {
		return errors.New("daemon: loop Goal revoke context is required")
	}
	if r.sessions != nil {
		r.sessions.RevokeManagedInput(session.ManagedInputOwner{
			QueueEntryID:  lease.QueueEntryID,
			SessionID:     lease.SessionID,
			OwnerKind:     lease.OwnerKind,
			LoopRunID:     lease.LoopRunID,
			TaskRunID:     lease.TaskRunID,
			RunGeneration: lease.RunGeneration,
			PromptAttempt: lease.PromptAttempt,
			ControlEpoch:  lease.ControlEpoch,
			BindingEpoch:  lease.BindingEpoch,
			PromptID:      lease.PromptID,
			PromptKind:    lease.PromptKind,
		}, reason)
	}
	if strings.TrimSpace(lease.JudgeAttemptID) == "" {
		return nil
	}
	if r.judges == nil {
		return fmt.Errorf("daemon: loop judge revoker is unavailable for attempt %q", lease.JudgeAttemptID)
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
	defer cancel()
	return r.judges.revokeExecution(cleanupCtx, lease.JudgeAttemptID)
}

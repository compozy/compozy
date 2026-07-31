package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	goalpkg "github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (l *loopGoalManagedInputLifecycle) Revoke(
	ctx context.Context,
	owner session.ManagedInputOwner,
	reason string,
) error {
	operation, err := l.loadOperation(ctx, owner)
	if err != nil {
		entry, lookupErr := l.store.GetSessionInputQueueEntryByID(ctx, owner.QueueEntryID)
		if lookupErr == nil && managedGoalEntrySettled(&entry) {
			l.clearUsageReporter(owner.QueueEntryID)
			return nil
		}
		return errors.Join(err, lookupErr)
	}
	_, err = l.store.RevokeGoalPrompt(ctx, goalpkg.RevokePromptRequest{
		Key:                  operation.key,
		ExpectedControlEpoch: owner.ControlEpoch,
		ExpectedBindingEpoch: owner.BindingEpoch,
		TaskRunID:            owner.TaskRunID,
		QueueEntryID:         owner.QueueEntryID,
		PromptID:             owner.PromptID,
		Disposition:          looppkg.ActionDispositionPaused,
		Status:               string(looppkg.StatusPaused),
		Cause:                looppkg.ReasonCodeGoalControlRevokedInFlight,
		ActorKind:            string(taskpkg.ActorKindDaemon),
		ActorID:              firstNonEmptyManagedGoal(strings.TrimSpace(reason), "managed-input-revoke"),
		ProjectionCause:      goalpkg.SessionOutboxCauseStatus,
		RevokedAt:            l.currentTime(),
	})
	if err == nil {
		l.clearUsageReporter(owner.QueueEntryID)
	}
	if err != nil {
		return fmt.Errorf("daemon: revoke managed Goal prompt %q: %w", owner.PromptID, err)
	}
	return nil
}

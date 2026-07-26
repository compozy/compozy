package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (l *loopGoalManagedInputLifecycle) Revoke(
	ctx context.Context,
	owner session.ManagedInputOwner,
	reason string,
) error {
	operation, err := l.loadOperation(ctx, owner)
	if err != nil {
		entry, lookupErr := l.store.GetSessionInputQueueEntryByID(ctx, owner.QueueEntryID)
		if lookupErr == nil && managedGoalEntrySettled(entry) {
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

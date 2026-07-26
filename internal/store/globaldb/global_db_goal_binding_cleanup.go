package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const (
	goalBindingFailureStopCreationUnsettled = "goal_stop_creation_unsettled"
	goalBindingFailureStopCreationSettled   = "goal_control_revoked_in_flight"
)

func closeGoalBindingWithCleanup(
	ctx context.Context,
	exec taskSQLExecutor,
	key goal.BindingKey,
	bindingEpoch int64,
	sessionID string,
	cause goal.SessionCleanupCause,
	closedAt time.Time,
) error {
	binding, err := getSessionBindingAttemptWithExecutor(ctx, exec, key, bindingEpoch)
	if err != nil {
		return err
	}
	if binding.SessionID != strings.TrimSpace(sessionID) {
		return goalControlStaleError("Goal cleanup binding session changed")
	}
	switch binding.State {
	case goal.BindingStateClosed, goal.BindingStateReseeded:
		if binding.ClosedAt != nil {
			closedAt = *binding.ClosedAt
		}
		return enqueueGoalSessionCleanupWithExecutor(ctx, exec, binding, cause, closedAt)
	case goal.BindingStateActive:
	default:
		return goalControlStaleError("Goal cleanup binding is not active")
	}
	affected, err := sqlcgen.New(exec).CloseGoalBindingWithCleanup(ctx, sqlcgen.CloseGoalBindingWithCleanupParams{
		ClosedAt: store.FormatTimestamp(closedAt), LoopRunID: string(key.LoopRunID), Handle: key.Handle,
		BindingEpoch: bindingEpoch, SessionID: strings.TrimSpace(sessionID),
	})
	if err != nil {
		return fmt.Errorf("store: close Goal binding for cleanup: %w", err)
	}
	if err := requireGoalAffectedCount(affected, "close Goal binding for cleanup"); err != nil {
		return err
	}
	return enqueueGoalSessionCleanupWithExecutor(ctx, exec, binding, cause, closedAt)
}

func closeTerminalGoalCheckpointBinding(
	ctx context.Context,
	exec taskSQLExecutor,
	checkpoint goal.Checkpoint,
	at time.Time,
) error {
	if checkpoint.BindingEpoch < 1 || strings.TrimSpace(checkpoint.BindingHandle) == "" ||
		strings.TrimSpace(checkpoint.SessionID) == "" {
		return nil
	}
	return closeGoalBindingWithCleanup(
		ctx,
		exec,
		goal.BindingKey{
			WorkspaceID: checkpoint.Key.WorkspaceID,
			LoopRunID:   checkpoint.Key.LoopRunID,
			Handle:      checkpoint.BindingHandle,
		},
		checkpoint.BindingEpoch,
		checkpoint.SessionID,
		goal.SessionCleanupCauseTerminal,
		at,
	)
}

// SettleStoppedSessionBindingCreation releases cleanup after an in-flight create effect has returned.
func (g *GoalRepo) SettleStoppedSessionBindingCreation(
	ctx context.Context,
	req goal.SettleStoppedBindingCreationRequest,
) (bool, error) {
	if err := g.checkReady(ctx, "settle stopped Goal binding creation"); err != nil {
		return false, err
	}
	if err := req.Key.Validate(); err != nil {
		return false, err
	}
	if req.ExpectedBindingEpoch < 1 || strings.TrimSpace(req.SessionID) == "" {
		return false, fmt.Errorf("%w: stopped Goal binding settlement identity is incomplete", looppkg.ErrValidation)
	}
	stopped := false
	err := g.withTaskImmediateTransaction(
		ctx,
		"settle stopped Goal binding creation",
		func(exec taskSQLExecutor) error {
			binding, err := getSessionBindingAttemptWithExecutor(ctx, exec, req.Key, req.ExpectedBindingEpoch)
			if err != nil {
				return err
			}
			if binding.SessionID != strings.TrimSpace(req.SessionID) {
				return goalControlStaleError("stopped Goal binding session changed")
			}
			if binding.State != goal.BindingStateFailed {
				return nil
			}
			switch binding.FailureCode {
			case goalBindingFailureStopCreationSettled:
				stopped = true
				return nil
			case goalBindingFailureStopCreationUnsettled:
			default:
				return nil
			}
			affected, err := sqlcgen.New(exec).SettleStoppedGoalBindingCreationForSession(
				ctx,
				sqlcgen.SettleStoppedGoalBindingCreationForSessionParams{
					SettledFailureCode: goalNullableString(goalBindingFailureStopCreationSettled),
					LoopRunID:          string(req.Key.LoopRunID), Handle: req.Key.Handle,
					BindingEpoch: req.ExpectedBindingEpoch, SessionID: strings.TrimSpace(req.SessionID),
					UnsettledFailureCode: goalNullableString(goalBindingFailureStopCreationUnsettled),
				},
			)
			if err != nil {
				return fmt.Errorf("store: settle stopped Goal binding creation: %w", err)
			}
			if err := requireGoalAffectedCount(affected, "settle stopped Goal binding creation"); err != nil {
				return err
			}
			stopped = true
			return nil
		},
	)
	return stopped, err
}

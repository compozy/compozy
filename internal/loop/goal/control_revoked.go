package goal

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/loop"
)

const (
	controlRevokedCause    = "Goal control revoked the in-flight turn."
	controlRevokedRecovery = "Start a new Goal to continue the objective."
)

type controlRevokedError struct{}

var _ loop.SafeActionFailureProvider = controlRevokedError{}

func newControlRevokedError() error {
	return controlRevokedError{}
}

func (controlRevokedError) Error() string {
	return loop.ErrTransitionConflict.Error() + ": Goal control revoked the in-flight prompt"
}

func (controlRevokedError) Unwrap() error {
	return loop.ErrTransitionConflict
}

func (controlRevokedError) SafeActionFailure() loop.ActionFailure {
	return loop.NewActionFailure(
		string(loop.ReasonCodeGoalControlRevokedInFlight),
		controlRevokedCause,
		controlRevokedRecovery,
	)
}

func (e *Executor) resolveExecutionError(
	ctx context.Context,
	segment *segmentState,
	executionErr error,
) error {
	checkpoint, err := e.store.LoadCheckpoint(context.WithoutCancel(ctx), segment.key)
	if err != nil {
		return errors.Join(executionErr, fmt.Errorf("load Goal checkpoint after execution error: %w", err))
	}
	if checkpoint.ControlEpoch > segment.checkpoint.ControlEpoch &&
		checkpoint.Phase == checkpointPhaseTerminal &&
		checkpoint.ControlCause == loop.ReasonCodeGoalControlRevokedInFlight {
		return errors.Join(newControlRevokedError(), executionErr)
	}
	return executionErr
}

package goal

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/loop"
)

const (
	defaultWaitInterval = time.Second
	// A timer can run for 24 hours, followed by bounded wake-delivery retries.
	defaultWaitTimeout = 25 * time.Hour
)

// WaitSessionRecovery restores the existing bound runtime so queued wake input can run after restart.
type WaitSessionRecovery interface {
	RecoverWaitSession(context.Context, loop.ActionSessionBinding) error
}

// WaitReader inspects the bound session's daemon-owned monitors and delegated children.
type WaitReader interface {
	HasLiveWait(context.Context, loop.ActionSessionBinding) (bool, error)
}

// WaitCheckpointRequest changes only an idle Goal's wait state under its current owner.
type WaitCheckpointRequest struct {
	Checkpoint Checkpoint
	Waiting    bool
}

// WaitStore persists a wait without allocating a prompt, turn, or judge attempt.
type WaitStore interface {
	SetWaitState(context.Context, WaitCheckpointRequest) (Checkpoint, error)
}

// hasLiveWait reads daemon authority rather than inferring a wait from model output.
func (e *Executor) hasLiveWait(ctx context.Context, segment *segmentState) (bool, error) {
	if e.waits == nil {
		return false, nil
	}
	return e.waits.HasLiveWait(ctx, segment.binding)
}

// waitBeforeWork remains in the executor's cancelable lifetime. Public input and wake
// delivery remain owned by the session queue and run independently while no Goal prompt exists.
func (e *Executor) waitBeforeWork(ctx context.Context, segment *segmentState) (*turnBoundary, error) {
	if err := e.recoverWaitSession(ctx, segment.binding); err != nil {
		return e.waitFailureBoundary(ctx, segment, loop.ReasonCodeGoalWaitUnavailable)
	}
	for {
		waiting, err := e.hasLiveWait(ctx, segment)
		if err != nil {
			return e.waitFailureBoundary(ctx, segment, loop.ReasonCodeGoalWaitUnavailable)
		}
		if !waiting {
			if segment.checkpoint.Phase == checkpointPhaseWaiting || segment.checkpoint.WaitStartedAt != nil {
				segment.checkpoint, err = e.store.SetWaitState(ctx, WaitCheckpointRequest{
					Checkpoint: segment.checkpoint,
				})
			}
			return nil, err
		}
		if segment.checkpoint.Phase != checkpointPhaseWaiting {
			segment.checkpoint, err = e.store.SetWaitState(ctx, WaitCheckpointRequest{
				Checkpoint: segment.checkpoint, Waiting: true,
			})
			if err != nil {
				return nil, err
			}
		}
		if control, paused, err := e.pendingPauseBoundary(ctx, segment); paused || err != nil {
			return &turnBoundary{checkpoint: segment.checkpoint, control: control}, err
		}
		if segment.checkpoint.WaitStartedAt == nil ||
			!e.now().Before(segment.checkpoint.WaitStartedAt.Add(e.waitTimeout)) {
			return e.waitFailureBoundary(ctx, segment, loop.ReasonCodeGoalWaitTimeout)
		}
		decision, err := e.flushBudget(ctx, segment, BudgetBeforeWork, checkpointPhaseWaiting,
			segment.checkpoint.TurnsUsed+1, "goal-wait:"+checkpointIdentity(segment.checkpoint),
			segment.usage.operationBase())
		if err != nil {
			return nil, err
		}
		if !decision.Allowed {
			control, err := e.budgetBoundary(ctx, segment, decision)
			return &turnBoundary{checkpoint: segment.checkpoint, control: control}, err
		}
		remaining := segment.checkpoint.WaitStartedAt.Add(e.waitTimeout).Sub(e.now())
		if err := waitForRecheck(ctx, min(e.waitInterval, remaining)); err != nil {
			return nil, err
		}
		checkpoint, err := e.store.LoadCheckpoint(ctx, segment.key)
		if err != nil {
			return nil, err
		}
		if checkpoint.ControlEpoch != segment.checkpoint.ControlEpoch {
			return nil, loop.ErrTransitionConflict
		}
		segment.checkpoint = checkpoint
		if checkpoint.Phase == checkpointPhaseTerminal || checkpoint.Phase == checkpointPhaseAwaitingControl {
			control, err := e.recoveredControl(ctx, segment)
			return &turnBoundary{checkpoint: checkpoint, control: control}, err
		}
	}
}

func waitForRecheck(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// recoverWaitSession keeps daemon session recovery outside the provider-neutral wait reader.
func (e *Executor) recoverWaitSession(ctx context.Context, binding loop.ActionSessionBinding) error {
	if recovery, ok := e.waits.(WaitSessionRecovery); ok {
		return recovery.RecoverWaitSession(ctx, binding)
	}
	return nil
}

func (e *Executor) waitFailureBoundary(ctx context.Context, segment *segmentState,
	cause loop.ReasonCode) (*turnBoundary, error) {
	control, err := e.checkpointBoundary(ctx, segment, loop.ActionDispositionNeedsApproval,
		goalStatusPaused, cause, automaticActor())
	return &turnBoundary{checkpoint: segment.checkpoint, control: control}, err
}

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

func (e *Executor) hasLiveWait(ctx context.Context, segment *segmentState) (bool, error) {
	if e.waits == nil {
		return false, nil
	}
	return e.waits.HasLiveWait(ctx, segment.binding)
}

// waitBeforeWork remains in the executor's cancelable lifetime. Public input and wake
// delivery remain owned by the session queue and run independently while no Goal prompt exists.
func (e *Executor) waitBeforeWork(ctx context.Context, segment *segmentState) (*turnBoundary, error) {
	for {
		waiting, err := e.hasLiveWait(ctx, segment)
		if err != nil {
			return e.waitFailureBoundary(ctx, segment, loop.ReasonCodeGoalWaitUnavailable)
		}
		if !waiting {
			if segment.checkpoint.Phase == checkpointPhaseWaiting {
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
		timer := time.NewTimer(e.waitInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
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

func (e *Executor) waitFailureBoundary(ctx context.Context, segment *segmentState,
	cause loop.ReasonCode) (*turnBoundary, error) {
	control, err := e.checkpointBoundary(ctx, segment, loop.ActionDispositionNeedsApproval,
		goalStatusPaused, cause, automaticActor())
	return &turnBoundary{checkpoint: segment.checkpoint, control: control}, err
}

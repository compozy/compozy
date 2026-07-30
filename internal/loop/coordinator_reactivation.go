package loop

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/task"
)

// CoordinatorReactivationRequest atomically returns a parked non-Goal Run to execution.
type CoordinatorReactivationRequest struct {
	Run           Run
	Cause         TransitionCause
	Actor         task.ActorContext
	Decisions     []GateDecisionRecord
	ReactivatedAt time.Time
}

// CoordinatorReactivationResult identifies the durable coordinator wake reserved by reactivation.
type CoordinatorReactivationResult struct {
	Run task.Run
}

// CoordinatorControlStore owns atomic non-Goal control reactivation.
type CoordinatorControlStore interface {
	ReactivateLoopCoordinator(
		context.Context,
		*CoordinatorReactivationRequest,
	) (CoordinatorReactivationResult, error)
}

// CoordinatorRunActivator starts a committed coordinator wake.
type CoordinatorRunActivator interface {
	ActivateCoordinatorRun(context.Context, task.Run)
}

// CoordinatorRunActivatorFunc adapts a function to CoordinatorRunActivator.
type CoordinatorRunActivatorFunc func(context.Context, task.Run)

// ActivateCoordinatorRun implements CoordinatorRunActivator.
func (f CoordinatorRunActivatorFunc) ActivateCoordinatorRun(ctx context.Context, run task.Run) {
	if f != nil {
		f(ctx, run)
	}
}

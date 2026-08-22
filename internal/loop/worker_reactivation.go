package loop

import (
	"context"

	"github.com/compozy/compozy/internal/task"
)

// WorkerRunActivator hands a committed Loop worker run to the daemon runtime.
type WorkerRunActivator interface {
	ActivateWorkerRun(context.Context, task.Run)
}

// WorkerRunActivatorFunc adapts a function to WorkerRunActivator.
type WorkerRunActivatorFunc func(context.Context, task.Run)

// ActivateWorkerRun implements WorkerRunActivator.
func (f WorkerRunActivatorFunc) ActivateWorkerRun(ctx context.Context, run task.Run) {
	if f != nil {
		f(ctx, run)
	}
}

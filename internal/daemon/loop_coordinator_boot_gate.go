package daemon

import (
	"context"
	"sync/atomic"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

// loopCoordinatorBootGate keeps every coordinator activation source inert
// until boot reconciliation finishes.
type loopCoordinatorBootGate struct {
	backstop loopCoordinatorBackstopRunner
	ready    atomic.Bool
}

var _ loopCoordinatorBackstopRunner = (*loopCoordinatorBootGate)(nil)

func newLoopCoordinatorBootGate(backstop loopCoordinatorBackstopRunner) *loopCoordinatorBootGate {
	return &loopCoordinatorBootGate{backstop: backstop}
}

func (g *loopCoordinatorBootGate) Activate() {
	g.ready.Store(true)
}

func (g *loopCoordinatorBootGate) RunLoopCoordinatorBackstop(
	ctx context.Context,
	now time.Time,
	actor taskpkg.ActorContext,
) (int, error) {
	if !g.ready.Load() {
		return 0, nil
	}
	return g.backstop.RunLoopCoordinatorBackstop(ctx, now, actor)
}

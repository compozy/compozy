package daemon

import (
	looppkg "github.com/compozy/compozy/internal/loop"
)

// loopCoordinatorInterrupter resolves the task manager lazily because Loop services are built
// before the task runtime that owns in-flight coordinator evaluations.
type loopCoordinatorInterrupter struct {
	state *bootState
}

var _ looppkg.CoordinatorInterrupter = loopCoordinatorInterrupter{}

// InterruptLoopCoordinatorRuns stops every coordinator evaluation the daemon is running for one
// Loop run. A canceled Run reaches its in-flight gate command through this seam; without it the
// command outlives the Run that owns it.
func (i loopCoordinatorInterrupter) InterruptLoopCoordinatorRuns(loopRunID string, reason string) int {
	if i.state == nil || i.state.tasks == nil || i.state.tasks.manager == nil {
		return 0
	}
	return i.state.tasks.manager.InterruptLoopCoordinatorRuns(loopRunID, reason)
}

package daemon

import (
	schedulerpkg "github.com/compozy/compozy/internal/scheduler"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// The production global store MUST satisfy the scheduler's StarvationStore so
// the convergence backstop is statically wired, never silently disabled.
var _ schedulerpkg.StarvationStore = (*globaldb.GlobalDB)(nil)
var _ schedulerpkg.TaskSource = schedulerTaskSource{}
var _ schedulerpkg.LoopCoordinatorBackstop = schedulerTaskSource{}

type schedulerTaskSource struct {
	manager             *taskpkg.Service
	store               taskStore
	watchEventsGapScan  *loopWatchEventsGapScanState
	coordinatorBackstop loopCoordinatorBackstopRunner
}

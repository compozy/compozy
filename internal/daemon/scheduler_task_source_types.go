package daemon

import (
	looppkg "github.com/compozy/compozy/internal/loop"
	schedulerpkg "github.com/compozy/compozy/internal/scheduler"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// The production global store MUST satisfy the scheduler's StarvationStore so
// the convergence backstop is statically wired, never silently disabled.
var _ schedulerpkg.StarvationStore = (*globaldb.GlobalDB)(nil)
var _ schedulerpkg.TaskSource = schedulerTaskSource{}
var _ schedulerpkg.LoopCoordinatorBackstop = schedulerTaskSource{}
var _ schedulerpkg.LoopCancellationBackstop = schedulerTaskSource{}

type schedulerTaskSource struct {
	manager                *taskpkg.Service
	store                  taskStore
	watchEventsGapScan     *loopWatchEventsGapScanState
	loopRetryDueScan       *loopRetryDueScanState
	loopWaitDueScan        *loopWaitDueScanState
	loopWaitEscalationScan *loopWaitEscalationScanState
	coordinatorBackstop    loopCoordinatorBackstopRunner
	cancellationReconciler looppkg.CancellationReconciler
}

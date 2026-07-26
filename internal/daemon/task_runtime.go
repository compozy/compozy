package daemon

import (
	"context"
	"sync/atomic"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	defaultTaskCancelGrace     = 5 * time.Second
	taskRecoveryReasonBoot     = "orphaned_on_boot"
	taskStopDetailShutdown     = "task shutdown"
	taskStopDetailOrphaned     = "task run orphaned"
	taskStopDetailCancellation = "task cancellation"

	taskRecoveryClassificationLive     = "live"
	taskRecoveryClassificationMissing  = "missing"
	taskRecoveryClassificationCrashed  = "crashed"
	taskRecoveryClassificationOrphaned = "orphaned"
	taskRecoveryClassificationStalled  = "stalled"
)

type taskStore interface {
	taskpkg.Store
}

type loopCoordinatorBootReconciler interface {
	ReconcileLoopCoordinatorsOnBoot(
		ctx context.Context,
		origin taskpkg.Origin,
		now time.Time,
	) ([]taskpkg.Run, error)
}

type taskRuntime struct {
	manager              *taskpkg.Service
	store                taskStore
	detached             *harnessDetachedWorkBridge
	reentry              *harnessReentryBridge
	wakeBridge           *taskWakeBridge
	bridgeNotifications  *bridgeTerminalTaskNotificationObserver
	taskStatusProjection *taskStatusProjectionObserver
	loopActions          *loopActionRuntime
	coordinatorBackstop  *loopCoordinatorBootGate
	loopJudges           *loopGateJudgeRunner
	activation           atomic.Pointer[taskRunActivationDispatcher]
	roles                atomic.Pointer[taskRoleRuntime]
}

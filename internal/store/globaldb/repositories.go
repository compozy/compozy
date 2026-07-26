package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

type repoBase struct {
	db       *sql.DB
	queries  *sqlcgen.Queries
	path     string
	now      func() time.Time
	closed   *atomic.Int32
	tasks    *TaskRepo
	sessions *SessionRepo
}

func newRepoBase(db *sql.DB, now func() time.Time, closed *atomic.Int32) *repoBase {
	return &repoBase{
		db:      db,
		queries: sqlcgen.New(db),
		now:     now,
		closed:  closed,
	}
}

func (r *repoBase) checkReady(ctx context.Context, action string) error {
	if r == nil || r.db == nil || r.queries == nil || r.closed == nil {
		return errors.New("store: repository is required")
	}
	if r.closed.Load() != 0 {
		return store.ErrClosed
	}
	if ctx == nil {
		return fmt.Errorf("store: %s context is required", action)
	}
	return nil
}

type WorkspaceRepo struct{ *repoBase }
type AppMetadataRepo struct{ *repoBase }
type BundleRepo struct{ *repoBase }
type PermissionRepo struct{ *repoBase }
type SessionRepo struct{ *repoBase }
type TaskRepo struct {
	*repoBase
	runs                       *TaskRunRepo
	enqueueLoopCoordinatorWake func(context.Context, string, string, taskpkg.Origin, time.Time) (taskpkg.Run, bool, error)
	taskEventObserverMu        sync.RWMutex
	taskEventObserver          taskpkg.EventObserver
}

type TaskRunRepo struct {
	*repoBase
	tasks *TaskRepo
}
type AutomationRepo struct{ *repoBase }
type BridgeRepo struct{ *repoBase }
type NetworkRepo struct{ *repoBase }

type LoopRepo struct {
	*repoBase
	watchEvents *WatchEventsRepo
	observe     *ObserveRepo
}
type GoalRepo struct{ *repoBase }
type HeartbeatRepo struct{ *repoBase }
type SoulRepo struct{ *repoBase }
type ModelCatalogRepo struct{ *repoBase }
type MarketplaceRepo struct{ *repoBase }
type ObserveRepo struct{ *repoBase }
type ToolRuntimeRepo struct{ *repoBase }
type VaultRepo struct{ *repoBase }
type WatchEventsRepo struct {
	*repoBase
	openSessionEventMetadata store.SessionEventMetadataOpener
}
type DeadEntityRepo struct{ *repoBase }
type ApprovalGrantRepo struct{ *repoBase }

func (g *GlobalDB) initializeRepositories(config openConfig) {
	base := newRepoBase(g.db, func() time.Time { return g.now() }, &g.closed)
	base.path = g.path
	g.WorkspaceRepo = &WorkspaceRepo{repoBase: base}
	g.AppMetadataRepo = &AppMetadataRepo{repoBase: base}
	g.BundleRepo = &BundleRepo{repoBase: base}
	g.PermissionRepo = &PermissionRepo{repoBase: base}
	g.SessionRepo = &SessionRepo{repoBase: base}
	base.sessions = g.SessionRepo
	taskRepo := &TaskRepo{repoBase: base}
	taskRunRepo := &TaskRunRepo{repoBase: base}
	g.TaskRepo = taskRepo
	g.TaskRunRepo = taskRunRepo
	base.tasks = taskRepo
	taskRepo.runs = taskRunRepo
	taskRunRepo.tasks = taskRepo
	g.AutomationRepo = &AutomationRepo{repoBase: base}
	g.BridgeRepo = &BridgeRepo{repoBase: base}
	g.NetworkRepo = &NetworkRepo{repoBase: base}
	loopRepo := &LoopRepo{repoBase: base}
	g.LoopRepo = loopRepo
	g.GoalRepo = &GoalRepo{repoBase: base}
	g.HeartbeatRepo = &HeartbeatRepo{repoBase: base}
	g.SoulRepo = &SoulRepo{repoBase: base}
	g.ModelCatalogRepo = &ModelCatalogRepo{repoBase: base}
	g.MarketplaceRepo = &MarketplaceRepo{repoBase: base}
	g.ObserveRepo = &ObserveRepo{repoBase: base}
	g.NotificationRepo = &NotificationRepo{repoBase: base}
	g.ToolRuntimeRepo = &ToolRuntimeRepo{repoBase: base}
	g.VaultRepo = &VaultRepo{repoBase: base}
	g.WatchEventsRepo = &WatchEventsRepo{
		repoBase:                 base,
		openSessionEventMetadata: config.openSessionEventMetadata,
	}
	g.DeadEntityRepo = &DeadEntityRepo{repoBase: base}
	g.ApprovalGrantRepo = &ApprovalGrantRepo{repoBase: base}
	loopRepo.watchEvents = g.WatchEventsRepo
	loopRepo.observe = g.ObserveRepo
	taskRepo.enqueueLoopCoordinatorWake = g.EnqueueLoopCoordinatorWake
}

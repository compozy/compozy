package daemon

import (
	"context"
	"time"

	"github.com/compozy/agh/internal/clientstate"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/memory/consolidation"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"
)

type shutdownTargets struct {
	scheduler           *schedulerRuntime
	coordinator         *coordinatorRuntime
	spawnReaper         *spawnReaper
	tasks               *taskRuntime
	sessions            SessionManager
	network             networkRuntime
	networkWakeRunner   *networkWakeRunner
	hooks               hookRuntime
	extensions          extensionRuntime
	automation          automationRuntime
	resourceReconcile   resources.ReconcileDriver
	bridges             *bridgeRuntime
	httpServer          Server
	udsServer           Server
	registry            Registry
	lock                *Lock
	closeLogger         func() error
	infoPath            string
	dreamRuntime        *consolidation.Runtime
	memoryExtractor     *daemonMemoryExtractor
	runtimeWorkers      daemonRuntimeWorkers
	memoryStore         *memory.Store
	localMemoryProvider memoryProviderShutdowner
	modelCatalog        *modelCatalogRuntime
	marketplace         *marketplaceRuntime
	clarify             *clarifyBridge
	skillsCancel        context.CancelFunc
	skillsDone          chan struct{}
	loopsCancel         context.CancelFunc
	loopsDone           chan struct{}
	goalOutboxCancel    context.CancelFunc
	goalOutboxDone      chan struct{}
	retention           observerRetentionStopper
	windowManagerStore  *clientstate.Engine
	windowManager       *windowmanager.Manager
}

func (d *Daemon) detachShutdownTargets() shutdownTargets {
	d.mu.Lock()
	defer d.mu.Unlock()

	targets := shutdownTargets{
		scheduler:           d.scheduler,
		coordinator:         d.coordinator,
		spawnReaper:         d.spawnReaper,
		tasks:               d.tasks,
		sessions:            d.sessions,
		network:             d.network,
		networkWakeRunner:   d.networkWakeRunner,
		hooks:               d.hooks,
		extensions:          d.extensions,
		automation:          d.automation,
		resourceReconcile:   d.resourceReconcile,
		bridges:             d.bridges,
		httpServer:          d.httpServer,
		udsServer:           d.udsServer,
		registry:            d.registry,
		lock:                d.lock,
		closeLogger:         d.closeLogger,
		infoPath:            d.homePaths.DaemonInfo,
		dreamRuntime:        d.dreamRuntime,
		memoryExtractor:     d.memoryExtractor,
		runtimeWorkers:      d.runtimeWorkers,
		memoryStore:         d.memoryStore,
		localMemoryProvider: d.localMemoryProvider,
		modelCatalog:        d.modelCatalog,
		marketplace:         d.marketplace,
		clarify:             d.clarify,
		skillsCancel:        d.skillsCancel,
		skillsDone:          d.skillsDone,
		loopsCancel:         d.loopsCancel,
		loopsDone:           d.loopsDone,
		goalOutboxCancel:    d.goalOutboxCancel,
		goalOutboxDone:      d.goalOutboxDone,
		windowManagerStore:  d.windowManagerStore,
		windowManager:       d.windowManager,
	}
	if stopper, ok := d.observer.(observerRetentionStopper); ok {
		targets.retention = stopper
	}

	d.resetRuntimeStateLocked()
	return targets
}

func (d *Daemon) resetRuntimeStateLocked() {
	d.sessions = nil
	d.tasks = nil
	d.coordinator = nil
	d.spawnReaper = nil
	d.scheduler = nil
	d.hooks = nil
	d.extensions = nil
	d.automation = nil
	d.resourceReconcile = nil
	d.httpServer = nil
	d.udsServer = nil
	d.observer = nil
	d.registry = nil
	d.harnessResolver = nil
	d.memoryStore = nil
	d.memoryProviderRegistry = nil
	d.memoryExtractor = nil
	d.runtimeWorkers = daemonRuntimeWorkers{}
	d.localMemoryProvider = nil
	d.modelCatalog = nil
	d.marketplace = nil
	d.clarify = nil
	d.skillsRegistry = nil
	d.loopCatalog = nil
	d.lock = nil
	d.booting = false
	d.info = Info{}
	d.startedAt = time.Time{}
	d.closeLogger = func() error { return nil }
	d.dreamRuntime = nil
	d.workspaceResolver = nil
	d.windowManagerStore = nil
	d.windowManager = nil
	d.sandboxRegistry = nil
	d.skillsCancel = nil
	d.skillsDone = nil
	d.loopsCancel = nil
	d.loopsDone = nil
	d.goalOutboxCancel = nil
	d.goalOutboxDone = nil
	d.bridges = nil
	d.network = nil
	d.networkWakeRunner = nil
	d.toolRegistry = nil
}

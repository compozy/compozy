package daemon

func (d *Daemon) publishBootState(state *bootState) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.config = state.cfg
	d.logger = state.logger
	d.closeLogger = state.closeLogger
	d.booting = false
	d.lock = state.lock
	d.harnessResolver = state.harnessResolver
	d.registry = state.registry
	d.memoryStore = state.memoryStore
	d.memoryProviderRegistry = state.memoryProviderRegistry
	d.memoryExtractor = state.memoryExtractor
	d.runtimeWorkers = state.runtimeWorkers
	d.localMemoryProvider = nil
	if state.localMemoryProvider != nil {
		d.localMemoryProvider = checkpointMemoryShutdowner{
			runtime:  state.checkpointRuntime,
			provider: state.localMemoryProvider,
		}
	}
	d.modelCatalog = state.modelCatalog
	d.marketplace = state.marketplace
	d.situationContext = state.situationContext
	d.sessions = state.sessions
	d.tasks = state.tasks
	d.coordinator = state.coordinator
	d.spawnReaper = state.spawnReaper
	d.scheduler = state.scheduler
	d.network = state.network
	d.networkWakeRunner = state.networkWakeRunner
	d.toolRegistry = state.toolRegistry
	d.clarify = state.clarify
	d.hooks = state.hooks
	d.extensions = state.currentExtensionRuntime()
	d.bridges = state.bridges
	d.observer = state.observer
	d.resourceReconcile = state.resourceReconcile
	d.agentCatalog = state.agentCatalog
	d.soulCatalog = state.soulCatalog
	d.heartbeatCatalog = state.heartbeatCatalog
	d.toolCatalog = state.toolCatalog
	d.mcpServerCatalog = state.mcpServerCatalog
	d.loopCatalog = state.loopCatalog
	d.automation = state.automation
	d.httpServer = state.httpServer
	d.udsServer = state.udsServer
	d.dreamRuntime = state.dreamRuntime
	d.workspaceResolver = state.workspaceResolver
	d.windowManagerStore = state.windowManagerStore
	d.windowManager = state.windowManager
	d.sandboxRegistry = state.sandboxRegistry
	d.skillsRegistry = state.skillsRegistry
	d.skillsCancel = state.skillsCancel
	d.skillsDone = state.skillsDone
	d.loopsCancel = state.loopsCancel
	d.loopsDone = state.loopsDone
	d.goalOutboxCancel = state.goalOutboxCancel
	d.goalOutboxDone = state.goalOutboxDone
	d.startedAt = state.startedAt
	d.info = state.info
	if !d.readyClosed {
		close(d.readyCh)
		d.readyClosed = true
	}
}

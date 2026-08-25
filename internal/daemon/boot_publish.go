package daemon

func (d *Daemon) publishBootState(state *bootState) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.config = state.cfg
	d.logger = state.logger
	d.closeLogger = state.closeLogger
	d.booting = false
	var localMemoryProvider memoryProviderShutdowner
	if state.localMemoryProvider != nil {
		localMemoryProvider = checkpointMemoryShutdowner{
			runtime:  state.checkpointRuntime,
			provider: state.localMemoryProvider,
		}
	}
	d.daemonRuntimeState = daemonRuntimeState{
		lock:                   state.lock,
		harnessResolver:        state.harnessResolver,
		registry:               state.registry,
		profiles:               state.profiles,
		memoryStore:            state.memoryStore,
		memoryProviderRegistry: state.memoryProviderRegistry,
		memoryExtractor:        state.memoryExtractor,
		runtimeWorkers:         state.runtimeWorkers,
		terminals:              state.terminals,
		localMemoryProvider:    localMemoryProvider,
		situationContext:       state.situationContext,
		sessions:               state.sessions,
		sessionWakeBridge:      state.sessionWakeBridge,
		tasks:                  state.tasks,
		coordinator:            state.coordinator,
		spawnReaper:            state.spawnReaper,
		scheduler:              state.scheduler,
		network:                state.network,
		gateway:                state.gateway,
		networkWakeRunner:      state.networkWakeRunner,
		toolRegistry:           state.toolRegistry,
		clarify:                state.clarify,
		hooks:                  state.hooks,
		extensions:             state.currentExtensionRuntime(),
		observer:               state.observer,
		resourceReconcile:      state.resourceReconcile,
		supportBundles:         state.supportBundles,
		backgroundUpdates:      state.backgroundUpdates,
		agentCatalog:           state.agentCatalog,
		soulCatalog:            state.soulCatalog,
		heartbeatCatalog:       state.heartbeatCatalog,
		toolCatalog:            state.toolCatalog,
		mcpServerCatalog:       state.mcpServerCatalog,
		loopCatalog:            state.loopCatalog,
		automation:             state.automation,
		bridges:                state.bridges,
		httpServer:             state.httpServer,
		udsServer:              state.udsServer,
		dreamRuntime:           state.dreamRuntime,
		workspaceResolver:      state.workspaceResolver,
		worktrees:              state.worktrees,
		sandboxRegistry:        state.sandboxRegistry,
		windowManagerRuntime: windowManagerRuntime{
			windowManagerStore: state.windowManagerStore,
			windowManagers:     state.windowManagers,
		},
		skillsRegistry:    state.skillsRegistry,
		modelCatalog:      state.modelCatalog,
		marketplace:       state.marketplace,
		skillsCancel:      state.skillsCancel,
		skillsDone:        state.skillsDone,
		loopsCancel:       state.loopsCancel,
		loopsDone:         state.loopsDone,
		goalOutboxCancel:  state.goalOutboxCancel,
		goalOutboxDone:    state.goalOutboxDone,
		effectRelayCancel: state.effectRelayCancel,
		effectRelayDone:   state.effectRelayDone,
		viewPatches:       state.viewPatches,
		startedAt:         state.startedAt,
		info:              state.info,
	}
	if !d.readyClosed {
		close(d.readyCh)
		d.readyClosed = true
	}
}

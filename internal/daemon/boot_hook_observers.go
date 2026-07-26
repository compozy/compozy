package daemon

import hookspkg "github.com/compozy/agh/internal/hooks"

func (d *Daemon) initializeHookObservers(
	state *bootState,
) ([]hookspkg.HookDecl, map[string]hookspkg.Executor) {
	state.lifecycleObservers = newSessionLifecycleFanout()
	if state.observer != nil {
		state.lifecycleObservers.Add(state.observer)
	}
	if state.harnessRecorder != nil {
		state.lifecycleObservers.Add(state.harnessRecorder)
	}
	if state.checkpointRuntime != nil {
		state.lifecycleObservers.Add(state.checkpointRuntime)
	}
	if state.clarify != nil {
		state.lifecycleObservers.Add(state.clarify)
	}
	state.hookTelemetrySinks = newHookTelemetryFanout()
	if sink, ok := state.observer.(hookspkg.TelemetrySink); ok {
		state.hookTelemetrySinks.Add(sink)
	}
	return daemonNativeHooks(
		state.lifecycleObservers,
		state.dreamRuntime,
		state.memoryExtractor,
		state.runtimeWorkers.autoTitle,
	)
}

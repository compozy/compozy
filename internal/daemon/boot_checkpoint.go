package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/memory"
)

func (d *Daemon) bootCheckpointSummaryRuntime(
	ctx context.Context,
	state *bootState,
	sessions SessionManager,
	cleanup *bootCleanup,
) error {
	if state == nil || (!state.cfg.Memory.Enabled && !state.cfg.Session.Compaction.Enabled) {
		return nil
	}
	checkpointSessions, ok := sessions.(checkpointSummarySessionManager)
	if !ok {
		return errors.New("daemon: session manager does not implement checkpoint summary lifecycle")
	}
	checkpointStore := state.memoryStore
	if checkpointStore == nil {
		checkpointStore = state.checkpointStore
		if checkpointStore == nil {
			return errors.New("daemon: compaction checkpoint store is unavailable")
		}
		// Only degraded replay reads this coverage; normal prompts still have no memory provider.
		assembler, ok := state.promptAssembler.(*ComposedAssembler)
		if !ok {
			return errors.New("daemon: compaction requires the composed prompt assembler")
		}
		assembler.resumeOnlyProvider = memory.NewAssembler(checkpointStore)
	}
	summarizer := newDaemonCheckpointSummarizer(
		checkpointSessions,
		roleResolverForState(state),
	)
	service := memory.NewCheckpointSummaryService(
		checkpointStore,
		state.workspaceResolver,
		summarizer,
		memory.WithCheckpointSummaryClock(d.now),
	)
	if state.localMemoryProvider != nil {
		state.localMemoryProvider.SetSessionEndHandler(service)
		state.localMemoryProvider.SetPreCompressHandler(service)
	}
	runtime := newCheckpointSummaryRuntime(
		sessions,
		state.memoryProviderRegistry,
		state.cfg.Memory.Provider.Timeout,
		state.cfg.Memory.Extractor.Deadline,
		state.logger,
		service,
	)
	if state.cfg.Memory.Enabled {
		if err := runtime.Start(ctx); err != nil {
			return fmt.Errorf("daemon: start checkpoint summary runtime: %w", err)
		}
	}
	cleanup.add(runtime.Shutdown)
	if binder, ok := sessions.(sessionCompactionBinder); ok {
		binder.SetCompactionHandler(runtime)
	}
	state.checkpointRuntime = runtime
	return nil
}

package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (d *Daemon) bootMemorySessionRuntime(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	ledgerMaterializer, err := d.newSessionLedgerMaterializer(state)
	if err != nil {
		return err
	}
	state.ledgerMaterializer = ledgerMaterializer
	resolver, err := ensureDaemonParticipationResolver(state, state.registry)
	if err != nil {
		return err
	}
	state.participationResolver = resolver
	sessionWakeBridge, err := newSessionWakeBridge(ctx, func() sessionWakeSessionManager {
		if state == nil || state.sessions == nil {
			return nil
		}
		return state.sessions
	}, state.logger)
	if err != nil {
		return fmt.Errorf("daemon: create session wake bridge: %w", err)
	}
	state.sessionWakeBridge = sessionWakeBridge
	cleanup.add(sessionWakeBridge.shutdown)

	sessions, err := d.newSessionManager(ctx, d.sessionManagerDeps(state))
	if err != nil {
		return fmt.Errorf("daemon: create session manager: %w", err)
	}
	cleanup.add(func(cleanupCtx context.Context) error {
		return d.shutdownSessionManager(cleanupCtx, sessions)
	})
	state.sessions = sessions
	if state.notifier != nil {
		state.notifier.setSessionProfileResolver(
			daemonSessionProfileResolver(sessions, "daemon: hook session profile is unavailable"),
		)
	}
	if state.harnessRecorder != nil {
		state.harnessRecorder.SetProfileResolver(
			daemonSessionProfileResolver(sessions, "daemon: harness session profile is unavailable"),
		)
	}
	if err := d.bootWorkspaceAccess(state, sessions); err != nil {
		return err
	}
	d.configureMemoryController(state, sessions)
	if err := d.bootClarifyBridge(state, cleanup); err != nil {
		return err
	}
	if err := installWorkspaceRemovalPreparer(state, sessions); err != nil {
		return err
	}
	memoryExtractor, err := newDaemonMemoryExtractor(ctx, state, sessions, d.now)
	if err != nil {
		return err
	}
	state.memoryExtractor = memoryExtractor
	if err := d.bootAutoTitleRuntime(ctx, state, sessions, cleanup); err != nil {
		return err
	}
	if err := d.bootCheckpointSummaryRuntime(ctx, state, sessions, cleanup); err != nil {
		return err
	}
	state.deps = d.runtimeDeps(ctx, state, sessions)
	resourceService, err := d.buildResourceService(state)
	if err != nil {
		return err
	}
	state.deps.Resources = resourceService
	return nil
}

func daemonSessionProfileResolver(
	sessions SessionManager,
	unavailableMessage string,
) func(context.Context, string) (string, error) {
	return func(ctx context.Context, sessionID string) (string, error) {
		info, err := sessions.Status(ctx, sessionID)
		if err != nil {
			return "", err
		}
		if info == nil {
			return "", errors.New(unavailableMessage)
		}
		return info.ProfileID, nil
	}
}

func (d *Daemon) bootClarifyBridge(state *bootState, cleanup *bootCleanup) error {
	if state == nil || state.sessions == nil {
		return errors.New("daemon: session manager is required before clarification broker")
	}
	publisher, ok := state.sessions.(clarifyEventPublisher)
	if !ok {
		return errors.New("daemon: session manager does not implement clarification event publication")
	}
	bridge, err := newClarifyBridge(
		state.cfg.Tools.Clarify.Timeout,
		publisher,
		extensionEventSummaryStore(state.registry),
		state.logger,
		withClarifyClock(d.now),
		withClarifySessionProfileResolver(clarifySessionProfileResolver(state.sessions)),
	)
	if err != nil {
		return fmt.Errorf("daemon: create clarification broker: %w", err)
	}
	cleanup.add(bridge.Close)
	state.clarify = bridge
	return nil
}

func (d *Daemon) bootAutoTitleRuntime(
	ctx context.Context,
	state *bootState,
	sessions SessionManager,
	cleanup *bootCleanup,
) error {
	if state == nil {
		return nil
	}
	titleSessions, ok := sessions.(autoTitleSessionManager)
	if !ok {
		return errors.New("daemon: session manager does not implement automatic title lifecycle")
	}
	deadline := state.cfg.Memory.Extractor.Deadline
	generator := newForkedAutoTitleGenerator(titleSessions, roleResolverForState(state), deadline, state.logger)
	runtime := newAutoTitleRuntime(titleSessions, generator, deadline, state.logger)
	if err := runtime.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start automatic title runtime: %w", err)
	}
	cleanup.add(runtime.Shutdown)
	state.runtimeWorkers.autoTitle = runtime
	return nil
}

func (d *Daemon) bootCheckpointSummaryRuntime(
	ctx context.Context,
	state *bootState,
	sessions SessionManager,
	cleanup *bootCleanup,
) error {
	if state == nil || state.localMemoryProvider == nil || state.memoryProviderRegistry == nil {
		return nil
	}
	checkpointSessions, ok := sessions.(checkpointSummarySessionManager)
	if !ok {
		return errors.New("daemon: session manager does not implement checkpoint summary lifecycle")
	}
	summarizer := newDaemonCheckpointSummarizer(
		checkpointSessions,
		roleResolverForState(state),
	)
	service := memory.NewCheckpointSummaryService(
		state.memoryStore,
		state.workspaceResolver,
		summarizer,
		memory.WithCheckpointSummaryClock(d.now),
	)
	state.localMemoryProvider.SetSessionEndHandler(service)
	state.localMemoryProvider.SetPreCompressHandler(service)
	runtime := newCheckpointSummaryRuntime(
		sessions,
		state.memoryProviderRegistry,
		state.cfg.Memory.Provider.Timeout,
		state.cfg.Memory.Extractor.Deadline,
		state.logger,
		service,
	)
	if err := runtime.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start checkpoint summary runtime: %w", err)
	}
	cleanup.add(runtime.Shutdown)
	if binder, ok := sessions.(sessionCompactionBinder); ok {
		binder.SetCompactionHandler(runtime)
	}
	state.checkpointRuntime = runtime
	return nil
}

type workspaceRemovalPreparer interface {
	PrepareWorkspaceRemoval(context.Context, string) (workspacepkg.UnregisterPreparation, error)
}

type sessionCompactionBinder interface {
	SetCompactionHandler(session.CompactionHandler)
}

type autoTitleSessionManager interface {
	autoTitleSessions
	autoTitleSpawnSessions
}

var _ autoTitleSessionManager = (*session.Manager)(nil)

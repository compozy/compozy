package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	loopdsl "github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func bootHarnessReentryBridge(
	ctx context.Context,
	state *bootState,
) (*harnessReentryBridge, error) {
	reentrySessions, ok := state.sessions.(harnessReentrySessionManager)
	if !ok {
		if state != nil && state.logger != nil {
			state.logger.Warn(
				"daemon: synthetic reentry bridge disabled because session manager support is unavailable",
			)
		}
		return nil, nil
	}
	reentryStore, ok := state.registry.(harnessReentryStore)
	if !ok {
		if state != nil && state.logger != nil {
			state.logger.Warn(
				"daemon: synthetic reentry bridge disabled because registry support is unavailable",
			)
		}
		return nil, nil
	}
	return newHarnessReentryBridge(
		ctx,
		state.harnessResolver,
		state.harnessRecorder,
		reentryStore,
		reentrySessions,
		state.logger,
		withHarnessHeartbeatWake(state.registry, reentrySessions, state.cfg.Agents.Heartbeat),
	)
}

func (d *Daemon) bootSpawnReaper(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	if state == nil || state.sessions == nil || state.tasks == nil || state.tasks.manager == nil {
		return nil
	}
	logger := state.logger
	if logger == nil {
		logger = slog.Default()
	}
	reaper, err := newSpawnReaper(
		ctx,
		state.sessions,
		state.tasks.manager,
		state.notifier,
		logger.With("component", "spawn_reaper"),
		d.now,
		defaultSpawnReaperInterval,
	)
	if err != nil {
		return err
	}
	report, err := reaper.Sweep(ctx)
	if err != nil {
		return err
	}
	if report.Reaped > 0 {
		logger.Info(
			"daemon: spawn reaper boot sweep complete",
			"reaped", report.Reaped,
			"released_leases", report.ReleasedLeases,
			"ttl_expired", report.TTLExpired,
			"parent_stopped", report.ParentStopped,
			"orphaned", report.Orphaned,
		)
	}
	reaper.start()
	state.spawnReaper = reaper
	if cleanup != nil {
		cleanup.add(func(cleanupCtx context.Context) error {
			return reaper.shutdown(cleanupCtx)
		})
	}
	return nil
}

func (d *Daemon) bootTaskRoles(ctx context.Context, state *bootState) error {
	if state == nil || state.tasks == nil || state.tasks.store == nil || state.sessions == nil {
		return nil
	}
	runtime, err := newTaskRoleRuntime(
		state.tasks.store,
		state.sessions,
		d.homePaths.HomeDir,
		state.logger,
		d.now,
	)
	if err != nil {
		return err
	}
	dispatcher, err := newTaskRunActivationDispatcher(
		state.tasks.store,
		runtime,
		state.tasks.loopActions,
		state.logger,
	)
	if err != nil {
		return err
	}
	if state.notifier != nil {
		state.notifier.AddTaskRunEnqueuedObserver(dispatcher)
	}
	state.tasks.activation.Store(dispatcher)
	dispatcher.Recover(ctx)
	state.tasks.roles.Store(runtime)
	return nil
}

func taskManagerOptions(
	store taskStore,
	bridge taskpkg.SessionExecutor,
	wakeNotifier taskpkg.WakeNotifier,
	events taskpkg.EventObserver,
	hooks *hooksNotifier,
	reviewRequests taskpkg.RunReviewRequestedObserver,
	coordinatorRunner taskpkg.CoordinatorRunner,
	generationFinalizer taskpkg.GenerationStateFinalizer,
	recovery aghconfig.TaskRecoveryConfig,
	scheduler aghconfig.SchedulerConfig,
	blockRecurrenceLimit int,
	workspaceActiveRunCap int,
) []taskpkg.Option {
	options := []taskpkg.Option{
		taskpkg.WithStore(store),
		taskpkg.WithSessionExecutor(bridge),
		taskpkg.WithWakeNotifier(wakeNotifier),
		taskpkg.WithEventObserver(events),
		taskpkg.WithRunReviewRequestedObserver(reviewRequests),
		taskpkg.WithCancelGracePeriod(defaultTaskCancelGrace),
		taskpkg.WithForceRecoveryOptions(taskpkg.ForceRecoveryOptions{
			AllowAgentForce: recovery.AllowAgentForce,
		}),
		taskpkg.WithStarvationAge(scheduler.MinQueuedAge),
		taskpkg.WithBlockRecurrenceLimit(blockRecurrenceLimit),
		taskpkg.WithWorkspaceActiveRunCap(workspaceActiveRunCap),
		taskpkg.WithCoordinatorTerminalStatusValidator(func(status string) bool {
			return looppkg.Status(strings.TrimSpace(status)).Valid()
		}),
		taskpkg.WithCoordinatorTerminalHookStatusValidator(func(status string) bool {
			return looppkg.Status(strings.TrimSpace(status)).Terminal()
		}),
	}
	if coordinatorRunner != nil {
		options = append(options, taskpkg.WithCoordinatorRunner(coordinatorRunner))
	}
	if generationFinalizer != nil {
		options = append(options, taskpkg.WithGenerationStateFinalizer(generationFinalizer))
	}
	if hooks != nil {
		options = append(options, taskpkg.WithTaskRunHooks(hooks))
	}
	if reader, ok := store.(taskpkg.InspectStateReader); ok {
		options = append(options, taskpkg.WithInspectStateReader(reader))
	}
	return options
}

func newLoopCoordinatorRunner(
	store taskStore,
	hooks looppkg.HookDispatcher,
	watchPoller watchpkg.Poller,
	gateEvaluator gate.GateEvaluator,
	actions *looppkg.ActionRegistry,
	logger *slog.Logger,
) (*looppkg.CoordinatorRunner, error) {
	loopStore, ok := store.(looppkg.Store)
	if !ok {
		if logger != nil {
			logger.Warn(
				"daemon: loop coordinator disabled because registry does not implement loop store",
			)
		}
		return nil, nil
	}
	outputs, ok := store.(looppkg.GenerationOutputReader)
	if !ok {
		if logger != nil {
			logger.Warn(
				"daemon: loop coordinator disabled because registry does not implement generation output reader",
			)
		}
		return nil, nil
	}
	options := []looppkg.CoordinatorRunnerOption{
		looppkg.WithCoordinatorHookDispatcher(hooks),
	}
	if watchPoller != nil {
		options = append(options, looppkg.WithCoordinatorWatchPoller(watchPoller))
	}
	if watchEventsLedger, ledgerOK := store.(looppkg.WatchEventsLedger); ledgerOK {
		options = append(options, looppkg.WithCoordinatorWatchEventsLedger(watchEventsLedger))
	} else if logger != nil {
		logger.Warn("daemon: loop coordinator watch-events ledger unavailable")
	}
	if gateEvaluator != nil {
		options = append(options, looppkg.WithCoordinatorGateEvaluator(gateEvaluator))
	}
	if actions != nil {
		options = append(options, looppkg.WithCoordinatorActionRegistry(actions))
	}
	return looppkg.NewCoordinatorRunner(
		store,
		loopStore,
		outputs,
		logger,
		options...,
	)
}

func newBootLoopCoordinatorRunner(
	store taskStore,
	state *bootState,
	homePaths aghconfig.HomePaths,
) (*looppkg.CoordinatorRunner, error) {
	runner, _, err := newBootLoopCoordinatorRuntime(store, state, homePaths)
	return runner, err
}

func newBootLoopCoordinatorRuntime(
	store taskStore,
	state *bootState,
	homePaths aghconfig.HomePaths,
) (*looppkg.CoordinatorRunner, *loopGateJudgeRunner, error) {
	var hooks looppkg.HookDispatcher
	if state.notifier != nil {
		hooks = state.notifier
	}
	var workspaceResolver workspacepkg.RuntimeResolver
	if state.workspaceResolver != nil {
		workspaceResolver = state.workspaceResolver
	}
	toolRegistry := lazyToolRegistry{current: func() toolspkg.Registry {
		if state == nil {
			return nil
		}
		return state.deps.ToolRegistry
	}}
	policyGate := &loopSessionPolicyGate{
		workspaceResolver: workspaceResolver,
		agentResolver: agentCatalogDependency(state.agentCatalog, agentSidecarCatalogs{
			soul:      state.soulCatalog,
			heartbeat: state.heartbeatCatalog,
		}),
	}
	judgeRunner := &loopGateJudgeRunner{
		sessions:            state.sessions,
		globalWorkspacePath: homePaths.HomeDir,
		policyGate:          policyGate,
		executions:          newLoopJudgeExecutionRegistry(),
	}
	gateEvaluator := gate.NewEvaluator(
		gate.WithCommandRunner(loopGateCommandRunner{}),
		gate.WithJudgeRunner(judgeRunner),
		gate.WithToolCaller(toolRegistry),
	)
	actions, err := newBootLoopActionRegistry(
		store,
		state,
		homePaths.HomeDir,
		toolRegistry,
		policyGate,
		gateEvaluator,
		judgeRunner.executions,
	)
	if err != nil {
		return nil, nil, err
	}
	runner, err := newLoopCoordinatorRunner(
		store,
		hooks,
		daemonExtensionWatchPoller{runtime: state.currentExtensionRuntime},
		gateEvaluator,
		actions,
		state.logger,
	)
	if err != nil {
		return nil, nil, err
	}
	return runner, judgeRunner, nil
}

type daemonExtensionWatchPoller struct {
	runtime func() extensionRuntime
}

var _ watchpkg.Poller = daemonExtensionWatchPoller{}

func (p daemonExtensionWatchPoller) Poll(
	ctx context.Context,
	req watchpkg.PollRequest,
) (watchpkg.PollResponse, error) {
	if p.runtime == nil {
		return watchpkg.PollResponse{}, fmt.Errorf(
			"%w: extension runtime resolver is unavailable",
			watchpkg.ErrPollerRequired,
		)
	}
	runtime := p.runtime()
	poller, ok := runtime.(watchpkg.Poller)
	if !ok || poller == nil {
		return watchpkg.PollResponse{}, fmt.Errorf(
			"%w: extension runtime does not provide watch poller",
			watchpkg.ErrPollerRequired,
		)
	}
	return poller.Poll(ctx, req)
}

type daemonLoopDefinitionResolver struct {
	catalog         *resourceCatalog[looppkg.ResourceSpec]
	compilerFactory func(context.Context) *looppkg.Compiler
}

var _ looppkg.DefinitionResolver = (*daemonLoopDefinitionResolver)(nil)

func (r *daemonLoopDefinitionResolver) ResolveLoop(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	name string,
) (*looppkg.ResolvedDefinition, error) {
	if r == nil || r.catalog == nil || r.compilerFactory == nil {
		return nil, fmt.Errorf(
			"%w: loop definition resolver is not initialized",
			looppkg.ErrValidation,
		)
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, fmt.Errorf("%w: loop name is required", looppkg.ErrValidation)
	}
	records := looppkg.ResolveEffectiveResources(r.catalog.Snapshot(), string(ws))
	for _, record := range records {
		if strings.TrimSpace(record.Spec.Name) != trimmedName {
			continue
		}
		definition, err := daemonLoopDefinitionFromSpec(record.Spec)
		if err != nil {
			return nil, err
		}
		return r.compilerFactory(ctx).Compile(definition)
	}
	return nil, fmt.Errorf("%w: loop definition %q not found", looppkg.ErrDefinitionNotFound, trimmedName)
}

func daemonLoopDefinitionFromSpec(spec looppkg.ResourceSpec) (loopdsl.Definition, error) {
	source := spec.Source.Normalize()
	filePath := strings.TrimSpace(spec.FilePath)
	if filePath == "" {
		return loopdsl.Definition{}, fmt.Errorf(
			"%w: loop %q has no file_path",
			looppkg.ErrValidation,
			spec.Name,
		)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return loopdsl.Definition{}, fmt.Errorf(
			"daemon: read loop definition %q: %w",
			filePath,
			err,
		)
	}
	_, definition, err := looppkg.ParseResource(data, looppkg.ResourceParseOptions{
		Source:                 source,
		Dir:                    strings.TrimSpace(spec.Dir),
		FilePath:               filePath,
		InstalledFromExtension: strings.TrimSpace(spec.InstalledFromExtension),
	})
	if err != nil {
		return loopdsl.Definition{}, fmt.Errorf(
			"daemon: parse loop definition %q: %w",
			filePath,
			err,
		)
	}
	return definition, nil
}

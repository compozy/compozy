package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/agh/internal/admission"
	looppkg "github.com/compozy/agh/internal/loop"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (d *Daemon) bootTasks(ctx context.Context, state *bootState) error {
	if state == nil || state.registry == nil || state.sessions == nil {
		return nil
	}

	store, ok := taskStoreForBoot(state)
	if !ok {
		return nil
	}

	bridge, err := newTaskSessionBridge(
		state.sessions,
		d.homePaths.HomeDir,
		state.logger,
		withTaskSessionContextOverlay(state.situationContext),
	)
	if err != nil {
		return err
	}
	reentry, err := bootHarnessReentryBridge(ctx, state)
	if err != nil {
		return fmt.Errorf("daemon: create harness reentry bridge: %w", err)
	}
	wakeBridge, err := newTaskWakeBridge(ctx, state.sessions, state.logger)
	if err != nil {
		return fmt.Errorf("daemon: create task wake bridge: %w", err)
	}
	reviewRequests := newRunReviewRequestedForwarder()
	eventObserver, bridgeNotifications, taskStatusProjection := d.composeTaskEventObserver(
		state,
		store,
		reentry,
	)
	coordinatorRunner, loopJudges, err := newBootLoopCoordinatorRuntime(store, state, d.homePaths)
	if err != nil {
		return fmt.Errorf("daemon: create loop coordinator runner: %w", err)
	}
	manager, err := newTaskRuntimeManager(
		state,
		store,
		bridge,
		wakeBridge,
		eventObserver,
		reviewRequests,
		coordinatorRunner,
		&d.admission,
	)
	if err != nil {
		return fmt.Errorf("daemon: create task manager: %w", err)
	}
	if err := bootSubprocessHealthEscalator(state, store, manager); err != nil {
		return err
	}
	coordinatorBackstop := newLoopCoordinatorBootGate(schedulerTaskSource{manager: manager, store: store})
	if err := installLoopTaskObservers(ctx, state, manager, store, coordinatorBackstop, d.now); err != nil {
		return err
	}
	loopActions, err := installLoopActionRuntime(state, manager, store, coordinatorRunner, d.now)
	if err != nil {
		return err
	}
	detached, err := newHarnessDetachedWorkBridge(manager, store, state.sessions)
	if err != nil {
		return fmt.Errorf("daemon: create detached harness bridge: %w", err)
	}

	installTaskRuntime(
		state,
		manager,
		store,
		detached,
		reentry,
		wakeBridge,
		bridgeNotifications,
		taskStatusProjection,
		loopActions,
		reviewRequests,
		coordinatorBackstop,
		loopJudges,
	)
	return recoverInstalledTaskRuntime(ctx, state, manager, store, reentry)
}

func taskStoreForBoot(state *bootState) (taskStore, bool) {
	store, ok := state.registry.(taskStore)
	if !ok {
		state.logger.Warn(
			"daemon: task runtime skipped because registry does not implement task store",
		)
	}
	return store, ok
}

func recoverInstalledTaskRuntime(
	ctx context.Context,
	state *bootState,
	manager *taskpkg.Service,
	store taskStore,
	reentry *harnessReentryBridge,
) error {
	if err := recoverBootTaskRuns(ctx, state, manager, store); err != nil {
		return err
	}
	return recoverDetachedHarnessReentry(ctx, reentry)
}

func installLoopTaskObservers(
	ctx context.Context,
	state *bootState,
	manager *taskpkg.Service,
	store taskStore,
	backstop loopCoordinatorBackstopRunner,
	now func() time.Time,
) error {
	if err := installLoopNativeHookObserver(state, manager, store, backstop, now); err != nil {
		return err
	}
	return installLoopWatchEventsObserver(ctx, state, manager, store, backstop, now)
}

func installLoopActionRuntime(
	state *bootState,
	manager *taskpkg.Service,
	store taskStore,
	coordinatorRunner *looppkg.CoordinatorRunner,
	now func() time.Time,
) (*loopActionRuntime, error) {
	if coordinatorRunner == nil {
		return nil, nil
	}
	loopActions, err := newLoopActionRuntime(
		manager,
		store,
		coordinatorRunner,
		state.sessions,
		state.logger,
		now,
		state.cfg.Task.Orchestration.ActionRunTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create loop action runtime: %w", err)
	}
	return loopActions, nil
}

func installLoopNativeHookObserver(
	state *bootState,
	manager *taskpkg.Service,
	store taskStore,
	backstop loopCoordinatorBackstopRunner,
	now func() time.Time,
) error {
	if state == nil || state.notifier == nil || manager == nil {
		return nil
	}
	loopStore, ok := store.(loopHookCoordinatorStore)
	if !ok {
		if state.logger != nil {
			state.logger.Warn("daemon: loop native hook observer skipped because task store lacks loop callbacks")
		}
		return nil
	}
	observer, err := newLoopNativeHookObserver(
		loopStore,
		state.notifier,
		backstop,
		now,
	)
	if err != nil {
		return err
	}
	state.notifier.AddLoopStartedObserver(observer)
	state.notifier.AddTaskRunTerminalObserver(observer)
	state.notifier.AddLoopTerminalObserver(observer)
	return nil
}

func installLoopWatchEventsObserver(
	ctx context.Context,
	state *bootState,
	manager *taskpkg.Service,
	store taskStore,
	backstop loopCoordinatorBackstopRunner,
	now func() time.Time,
) error {
	if state == nil || state.notifier == nil || manager == nil {
		return nil
	}
	watchStore, ok := store.(loopWatchEventsStore)
	if !ok {
		if state.logger != nil {
			state.logger.Warn(
				"daemon: loop watch-events observer skipped because task store lacks watch-events callbacks",
			)
		}
		return nil
	}
	observer, err := newLoopWatchEventsObserver(
		watchStore,
		backstop,
		now,
	)
	if err != nil {
		return err
	}
	if err := observer.Hydrate(ctx); err != nil {
		return err
	}
	state.notifier.AddTaskStatusChangedObserver(observer)
	state.notifier.AddTaskLifecycleWatchObserver(observer)
	state.notifier.AddTaskRunTerminalObserver(observer)
	state.notifier.AddLoopTerminalObserver(observer)
	state.notifier.AddLoopNodeTerminalObserver(observer)
	state.notifier.AddAutomationRunWatchObserver(observer)
	state.notifier.AddNetworkWatchObserver(observer)
	state.notifier.AddCoordinatorWatchObserver(observer)
	state.notifier.AddEventRecordWatchObserver(observer)
	return nil
}

func installTaskRuntime(
	state *bootState,
	manager *taskpkg.Service,
	store taskStore,
	detached *harnessDetachedWorkBridge,
	reentry *harnessReentryBridge,
	wakeBridge *taskWakeBridge,
	bridgeNotifications *bridgeTerminalTaskNotificationObserver,
	taskStatusProjection *taskStatusProjectionObserver,
	loopActions *loopActionRuntime,
	reviewRequests *runReviewRequestedForwarder,
	coordinatorBackstop *loopCoordinatorBootGate,
	loopJudges *loopGateJudgeRunner,
) {
	state.tasks = &taskRuntime{
		manager:              manager,
		store:                store,
		detached:             detached,
		reentry:              reentry,
		wakeBridge:           wakeBridge,
		bridgeNotifications:  bridgeNotifications,
		taskStatusProjection: taskStatusProjection,
		loopActions:          loopActions,
		coordinatorBackstop:  coordinatorBackstop,
		loopJudges:           loopJudges,
	}
	state.reviewRequests = reviewRequests
	state.deps.Tasks = manager
}

func newTaskRuntimeManager(
	state *bootState,
	store taskStore,
	bridge taskpkg.SessionExecutor,
	wakeBridge taskpkg.WakeNotifier,
	eventObserver taskpkg.EventObserver,
	reviewRequests taskpkg.RunReviewRequestedObserver,
	coordinatorRunner taskpkg.CoordinatorRunner,
	workAdmission admission.Checker,
) (*taskpkg.Service, error) {
	resolver, err := ensureDaemonParticipationResolver(state, store)
	if err != nil {
		return nil, err
	}
	options := taskManagerOptions(
		store,
		bridge,
		wakeBridge,
		eventObserver,
		state.notifier,
		reviewRequests,
		coordinatorRunner,
		looppkg.NewStoreFinalizer(),
		state.cfg.Task.Recovery,
		state.cfg.Autonomy.Scheduler,
		state.cfg.Autonomy.BlockRecurrenceLimit,
		state.cfg.Task.Orchestration.MaxActiveRunsPerWorkspace,
	)
	options = append(
		options,
		taskpkg.WithParticipationResolver(resolver),
		taskpkg.WithWorkAdmissionChecker(workAdmission),
	)
	return taskpkg.NewManager(options...)
}

func recoverDetachedHarnessReentry(ctx context.Context, reentry *harnessReentryBridge) error {
	if reentry == nil {
		return nil
	}
	if err := reentry.recover(ctx); err != nil {
		return fmt.Errorf("daemon: recover detached harness reentry bridge: %w", err)
	}
	return nil
}

func recoverBootTaskRuns(
	ctx context.Context,
	state *bootState,
	manager *taskpkg.Service,
	store taskStore,
) error {
	actor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "daemon.boot")
	if err != nil {
		return fmt.Errorf("daemon: derive task boot recovery actor: %w", err)
	}
	stats, err := recoverTaskRunsOnBoot(ctx, manager, store, state.sessions, actor)
	if err != nil {
		return err
	}
	if stats.requeued+stats.markedRunning+stats.failed > 0 {
		state.logger.Info(
			"daemon: task boot recovery complete",
			"requeued_runs", stats.requeued,
			"resumed_running_runs", stats.markedRunning,
			"failed_runs", stats.failed,
		)
	}
	if reconciler, ok := store.(loopCoordinatorBootReconciler); ok {
		runs, err := reconciler.ReconcileLoopCoordinatorsOnBoot(ctx, actor.Origin, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("daemon: reconcile loop coordinators on boot: %w", err)
		}
		if len(runs) > 0 {
			state.logger.Info(
				"daemon: loop coordinator boot reconcile complete",
				"enqueued_runs",
				len(runs),
			)
		}
	}
	return nil
}

func startBootLoopCoordinators(ctx context.Context, state *bootState) error {
	if state == nil || state.tasks == nil || state.tasks.coordinatorBackstop == nil {
		return nil
	}
	actor, err := taskpkg.DeriveDaemonActorContext("boot-recovery", "daemon.boot")
	if err != nil {
		return fmt.Errorf("daemon: derive loop coordinator boot actor: %w", err)
	}
	state.tasks.coordinatorBackstop.Activate()
	started, err := state.tasks.coordinatorBackstop.RunLoopCoordinatorBackstop(
		ctx,
		time.Now().UTC(),
		actor,
	)
	if err != nil {
		if errors.Is(err, looppkg.ErrDefinitionNotFound) {
			state.logger.Warn(
				"daemon: loop coordinator boot start deferred until loop definitions are reconciled",
				"error",
				err,
			)
			return nil
		}
		return fmt.Errorf("daemon: start loop coordinators on boot: %w", err)
	}
	if started > 0 {
		state.logger.Info("daemon: loop coordinator boot start complete", "started_runs", started)
	}
	return nil
}

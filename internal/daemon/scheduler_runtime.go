package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	schedulerpkg "github.com/compozy/agh/internal/scheduler"
	"github.com/compozy/agh/internal/situation"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	schedulerRuntimeTaskKey = "task"
)

const (
	defaultMechanicalSchedulerInterval   = 15 * time.Second
	defaultMechanicalSchedulerSweepLimit = 100
	defaultLoopCoordinatorBackstopLimit  = 16
	mechanicalSchedulerSweepReason       = "scheduler_sweep"
	mechanicalSchedulerWakeReason        = "pending_task_run"
)

type schedulerRuntime struct {
	scheduler *schedulerpkg.Scheduler
	waker     *schedulerSessionWaker
}

type effectiveTaskPauseStore interface {
	IsTaskEffectivelyPaused(ctx context.Context, taskID string) (bool, string, error)
}

func (d *Daemon) bootScheduler(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil || state.tasks == nil || state.tasks.manager == nil || state.tasks.store == nil {
		return nil
	}
	if state.sessions == nil {
		return errors.New("daemon: scheduler requires session manager")
	}
	logger := state.logger
	if logger == nil {
		logger = slog.Default()
	}
	logSchedulerPauseState(ctx, state.tasks.store, logger)

	waker := newSchedulerSessionWaker(ctx, state.sessions, logger)
	if err := waker.configureHeartbeatWake(state.registry, state.sessions, state.cfg.Agents.Heartbeat); err != nil {
		return err
	}
	spawner := starvationSpawner{
		workspaces: state.workspaceResolver,
		agents:     agentCatalogDependency(state.agentCatalog),
	}
	runtime, err := newSchedulerRuntime(
		ctx,
		state.tasks,
		state.sessions,
		state.situationContext,
		waker,
		spawner,
		starvationThresholdsFromConfig(state.cfg.Autonomy.Scheduler),
		logger,
	)
	if err != nil {
		if shutdownErr := waker.shutdown(context.Background()); shutdownErr != nil {
			logger.Warn("daemon: cleanup scheduler waker after create failure", "error", shutdownErr)
		}
		return err
	}
	if _, err := runtime.scheduler.Rebuild(ctx); err != nil {
		if shutdownErr := runtime.shutdown(context.Background()); shutdownErr != nil {
			logger.Warn("daemon: cleanup scheduler after rebuild failure", "error", shutdownErr)
		}
		return fmt.Errorf("daemon: rebuild scheduler state: %w", err)
	}
	if err := runtime.scheduler.Start(ctx); err != nil {
		if shutdownErr := runtime.shutdown(context.Background()); shutdownErr != nil {
			logger.Warn("daemon: cleanup scheduler after start failure", "error", shutdownErr)
		}
		return fmt.Errorf("daemon: start scheduler: %w", err)
	}
	state.scheduler = runtime
	if cleanup != nil {
		cleanup.add(func(cleanupCtx context.Context) error {
			return runtime.shutdown(cleanupCtx)
		})
	}
	return nil
}

func logSchedulerPauseState(ctx context.Context, store taskStore, logger *slog.Logger) {
	if store == nil || logger == nil {
		return
	}
	if pauseReader, ok := store.(interface {
		GetSchedulerPause(context.Context) (taskpkg.SchedulerPauseState, error)
	}); ok {
		state, err := pauseReader.GetSchedulerPause(ctx)
		if err != nil {
			logger.Warn("daemon: scheduler pause state unavailable", "error", err)
		} else if state.Paused {
			logger.Warn(
				"daemon: scheduler booting paused",
				"paused_by", state.PausedBy,
				"reason", state.Reason,
			)
		}
	}
	if counter, ok := store.(interface {
		CountPausedTasks(context.Context) (int, error)
	}); ok {
		count, err := counter.CountPausedTasks(ctx)
		if err != nil {
			logger.Warn("daemon: paused task count unavailable", "error", err)
		} else if count > 0 {
			logger.Info("daemon: scheduler found paused tasks", "paused_task_count", count)
		}
	}
}

func newSchedulerRuntime(
	ctx context.Context,
	tasks *taskRuntime,
	sessions SessionManager,
	situation *situation.Service,
	waker *schedulerSessionWaker,
	spawner starvationSpawner,
	thresholds schedulerpkg.StarvationThresholds,
	logger *slog.Logger,
) (*schedulerRuntime, error) {
	if ctx == nil {
		return nil, errors.New("daemon: scheduler context is required")
	}
	if tasks == nil || tasks.manager == nil || tasks.store == nil {
		return nil, errors.New("daemon: scheduler task runtime is required")
	}
	if sessions == nil {
		return nil, errors.New("daemon: scheduler session manager is required")
	}
	if waker == nil {
		return nil, errors.New("daemon: scheduler waker is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	manager := tasks.manager
	store := tasks.store
	var pauseStore schedulerpkg.PauseStore
	if candidate, ok := store.(schedulerpkg.PauseStore); ok {
		pauseStore = candidate
	}
	var starvationStore schedulerpkg.StarvationStore
	if candidate, ok := store.(schedulerpkg.StarvationStore); ok {
		starvationStore = candidate
	}
	escalationActor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
	if err != nil {
		return nil, fmt.Errorf("daemon: derive scheduler escalation actor: %w", err)
	}

	scheduler, err := schedulerpkg.New(
		schedulerTaskSource{
			manager:             manager,
			store:               store,
			watchEventsGapScan:  newLoopWatchEventsGapScanState(),
			coordinatorBackstop: tasks.coordinatorBackstop,
		},
		schedulerSessionSource{sessions: sessions, situation: situation, logger: logger},
		waker,
		schedulerpkg.WithLogger(logger),
		schedulerpkg.WithInterval(defaultMechanicalSchedulerInterval),
		schedulerpkg.WithSweepReason(mechanicalSchedulerSweepReason),
		schedulerpkg.WithWakeReason(mechanicalSchedulerWakeReason),
		schedulerpkg.WithSweepLimit(defaultMechanicalSchedulerSweepLimit),
		schedulerpkg.WithPauseStore(pauseStore),
		schedulerpkg.WithStarvationAge(thresholds.MinQueuedAge),
		schedulerpkg.WithStarvationThresholds(thresholds),
		schedulerpkg.WithStarvationStore(starvationStore),
		schedulerpkg.WithEscalationActor(escalationActorAdapter{
			manager: manager,
			actor:   escalationActor,
			tasks:   tasks,
			spawner: spawner,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create scheduler: %w", err)
	}
	return &schedulerRuntime{scheduler: scheduler, waker: waker}, nil
}

func starvationThresholdsFromConfig(cfg aghconfig.SchedulerConfig) schedulerpkg.StarvationThresholds {
	return schedulerpkg.StarvationThresholds{
		FanOutAfter:         cfg.FanOutAfter,
		SpawnAfter:          cfg.SpawnAfter,
		EventAfter:          cfg.EventAfter,
		NeedsAttentionAfter: cfg.NeedsAttentionAfter,
		MinQueuedAge:        cfg.MinQueuedAge,
	}
}

// escalationActorAdapter bridges the scheduler's escalation seam to the task service. It
// never claims work: it records observability events, marks needs_attention, and (in a
// later increment) requests a capability-matched worker spawn that self-claims.
type escalationActorAdapter struct {
	manager *taskpkg.Service
	actor   taskpkg.ActorContext
	tasks   *taskRuntime
	spawner starvationSpawner
}

func (a escalationActorAdapter) EmitRunStarved(
	ctx context.Context,
	work *schedulerpkg.RunSnapshot,
	age time.Duration,
) error {
	return a.manager.RecordRunStarved(ctx, work.Run.ID, work.Run.QueuedAt, age, a.actor)
}

func (a escalationActorAdapter) RequestWorkerSpawn(ctx context.Context, work *schedulerpkg.RunSnapshot) error {
	// roles is populated after bootScheduler (bootTaskRoles); convergence runs long after boot, so a
	// momentary nil here only means "spawn not requested this cycle" — the budget retries next cycle.
	if a.tasks == nil {
		return schedulerpkg.ErrSpawnUnresolvable
	}
	roles := a.tasks.roles.Load()
	if roles == nil {
		return schedulerpkg.ErrSpawnUnresolvable
	}
	err := roles.activateForStarvation(ctx, work.Task, work.Run, a.spawner)
	if errors.Is(err, errStarvationSpawnUnresolvable) {
		return schedulerpkg.ErrSpawnUnresolvable
	}
	return err
}

func (a escalationActorAdapter) MarkRunNeedsAttention(
	ctx context.Context,
	runID string,
	diagnostic string,
) (taskpkg.Run, error) {
	return a.manager.MarkRunNeedsAttention(ctx, runID, diagnostic, a.actor)
}

func (r *schedulerRuntime) shutdown(ctx context.Context) error {
	return errors.Join(r.stopLoop(ctx), r.shutdownWaker(ctx))
}

func (r *schedulerRuntime) stopLoop(ctx context.Context) error {
	if r == nil || r.scheduler == nil {
		return nil
	}
	return r.scheduler.Shutdown(ctx)
}

func (r *schedulerRuntime) shutdownWaker(ctx context.Context) error {
	if r == nil || r.waker == nil {
		return nil
	}
	return r.waker.shutdown(ctx)
}

func (s schedulerTaskSource) RecoverExpiredRunLeases(
	ctx context.Context,
	recovery taskpkg.ExpiredLeaseRecovery,
	actor taskpkg.ActorContext,
) ([]taskpkg.ExpiredLeaseRecoveryResult, error) {
	return s.manager.RecoverExpiredRunLeases(ctx, recovery, actor)
}

func (s schedulerTaskSource) ExpireTaskBlocks(
	ctx context.Context,
	now time.Time,
	actor taskpkg.ActorContext,
) (taskpkg.ExpireTaskBlocksResult, error) {
	return s.manager.ExpireTaskBlocks(ctx, now, actor)
}

func (s schedulerTaskSource) GetRunStatus(
	ctx context.Context,
	runID string,
) (taskpkg.RunStatus, bool, error) {
	run, err := s.store.GetTaskRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		if errors.Is(err, taskpkg.ErrTaskRunNotFound) {
			return taskpkg.TaskRunStatusUnknown, false, nil
		}
		return taskpkg.TaskRunStatusUnknown, false, fmt.Errorf("daemon: scheduler read run status %q: %w", runID, err)
	}
	return run.Status, true, nil
}

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/jonboulle/clockwork"
)

// Scheduler owns one mechanical sweep/notify loop.
type Scheduler struct {
	tasks      TaskSource
	sessions   SessionSource
	waker      Waker
	pauseStore PauseStore
	escalator  EscalationActor
	starvation StarvationStore

	logger           *slog.Logger
	clock            clockwork.Clock
	interval         time.Duration
	wakeCooldown     time.Duration
	starvationAge    time.Duration
	starveThresholds StarvationThresholds
	sweepReason      string
	wakeReason       string
	sweepLimit       int
	actor            taskpkg.ActorContext

	mu            sync.Mutex
	runtimeCancel context.CancelFunc
	runtimeDone   chan struct{}
	started       bool
	stopping      bool
	stopped       bool
	wakeState     map[wakeKey]time.Time
	stats         Stats
	wg            sync.WaitGroup
}

// New constructs a mechanical scheduler over durable task and session sources.
func New(tasks TaskSource, sessions SessionSource, waker Waker, opts ...Option) (*Scheduler, error) {
	if tasks == nil {
		return nil, errors.New("scheduler: task source is required")
	}
	if sessions == nil {
		return nil, errors.New("scheduler: session source is required")
	}
	if waker == nil {
		return nil, errors.New("scheduler: waker is required")
	}

	actor, err := taskpkg.DeriveDaemonActorContext("scheduler", "daemon.scheduler")
	if err != nil {
		return nil, fmt.Errorf("scheduler: derive daemon actor: %w", err)
	}
	s := &Scheduler{
		tasks:            tasks,
		sessions:         sessions,
		waker:            waker,
		logger:           slog.Default(),
		clock:            clockwork.NewRealClock(),
		interval:         defaultInterval,
		wakeCooldown:     defaultWakeCooldown,
		starvationAge:    defaultStarvationAge,
		starveThresholds: DefaultStarvationThresholds(),
		sweepReason:      defaultSweepReason,
		wakeReason:       defaultWakeReason,
		sweepLimit:       defaultSweepLimit,
		actor:            actor,
		wakeState:        make(map[wakeKey]time.Time),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	if s.logger == nil {
		s.logger = slog.Default()
	}
	if s.clock == nil {
		s.clock = clockwork.NewRealClock()
	}
	if s.interval <= 0 {
		s.interval = defaultInterval
	}
	if s.wakeCooldown < 0 {
		s.wakeCooldown = defaultWakeCooldown
	}
	if s.starvationAge < 0 {
		s.starvationAge = 0
	}
	if strings.TrimSpace(s.sweepReason) == "" {
		s.sweepReason = defaultSweepReason
	}
	if strings.TrimSpace(s.wakeReason) == "" {
		s.wakeReason = defaultWakeReason
	}
	if s.sweepLimit < 0 {
		s.sweepLimit = defaultSweepLimit
	}
	if err := s.actor.Validate(); err != nil {
		return nil, fmt.Errorf("scheduler: validate daemon actor: %w", err)
	}
	return s, nil
}

// Start begins the context-bound background scheduler loop.
func (s *Scheduler) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("scheduler: start context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || s.stopping {
		return ErrStopped
	}
	if s.started {
		return nil
	}

	runtimeCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	s.runtimeCancel = cancel
	s.runtimeDone = done
	s.started = true
	s.wg.Go(func() {
		defer func() {
			close(done)
			s.finishRuntime(done)
		}()
		s.loop(runtimeCtx)
	})
	s.logger.Info("scheduler.started", "interval_ms", s.interval.Milliseconds())
	return nil
}

// Shutdown cancels the scheduler loop and waits for owned goroutines to exit.
func (s *Scheduler) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("scheduler: shutdown context is required")
	}

	s.mu.Lock()
	if s.stopped && !s.stopping {
		s.mu.Unlock()
		return nil
	}
	if s.runtimeDone == nil {
		s.stopped = true
		s.stopping = false
		s.started = false
		s.runtimeCancel = nil
		s.mu.Unlock()
		s.logger.Info("scheduler.shutdown")
		return nil
	}
	s.stopping = true
	s.started = false
	cancel := s.runtimeCancel
	done := s.runtimeDone
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return fmt.Errorf("scheduler: shutdown runtime: %w", ctx.Err())
		}
	}
	s.wg.Wait()

	s.mu.Lock()
	if s.runtimeDone == done {
		s.runtimeCancel = nil
		s.runtimeDone = nil
	}
	s.stopped = true
	s.stopping = false
	s.started = false
	s.mu.Unlock()
	s.logger.Info("scheduler.shutdown")
	return nil
}

func (s *Scheduler) finishRuntime(done chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runtimeDone != done {
		return
	}
	s.runtimeCancel = nil
	s.runtimeDone = nil
	s.started = false
	s.stopping = false
	s.stopped = true
}

// Stats returns a consistent snapshot of scheduler counters.
func (s *Scheduler) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

// Rebuild clears scheduler-owned ephemeral wake state after reading durable
// task/session state. The returned counts are observability only; durable
// recovery remains in RunOnce through the task service.
func (s *Scheduler) Rebuild(ctx context.Context) (RebuildResult, error) {
	if ctx == nil {
		return RebuildResult{}, errors.New("scheduler: rebuild context is required")
	}
	now := s.clock.Now().UTC()

	pending, err := s.tasks.PendingRuns(ctx)
	if err != nil {
		return RebuildResult{}, fmt.Errorf("scheduler: rebuild pending runs: %w", err)
	}
	active, err := s.tasks.ActiveRuns(ctx)
	if err != nil {
		return RebuildResult{}, fmt.Errorf("scheduler: rebuild active runs: %w", err)
	}
	sessions, err := s.sessions.Sessions(ctx)
	if err != nil {
		return RebuildResult{}, fmt.Errorf("scheduler: rebuild sessions: %w", err)
	}

	s.mu.Lock()
	cleared := len(s.wakeState)
	s.wakeState = make(map[wakeKey]time.Time)
	s.stats.Rebuilds++
	s.stats.LastRebuildAt = now
	s.mu.Unlock()

	result := RebuildResult{
		PendingRuns:     len(pending),
		ActiveRuns:      len(active),
		SessionsScanned: len(sessions),
		ClearedWakeKeys: cleared,
		RebuiltAt:       now,
	}
	s.logger.Info(
		"scheduler.rebuild",
		"pending_runs", result.PendingRuns,
		"active_runs", result.ActiveRuns,
		"sessions_scanned", result.SessionsScanned,
		"cleared_wake_keys", result.ClearedWakeKeys,
	)
	return result, nil
}

// RunOnce executes one sweep/notify pass.
func (s *Scheduler) RunOnce(ctx context.Context) (CycleResult, error) {
	if ctx == nil {
		return CycleResult{}, errors.New("scheduler: run context is required")
	}
	now := s.clock.Now().UTC()
	result := CycleResult{}
	errs := s.sweepExpiredLeases(ctx, now, &result)
	errs = append(errs, s.sweepExpiredTaskBlocks(ctx, now, &result)...)
	if backstop, ok := s.tasks.(LoopCoordinatorBackstop); ok {
		count, err := backstop.RunLoopCoordinatorBackstop(ctx, now, s.actor)
		if err != nil {
			errs = append(errs, fmt.Errorf("scheduler: loop coordinator backstop: %w", err))
		}
		if count > 0 {
			s.logger.Info("scheduler.loop_coordinator_backstop", "started_runs", count)
		}
	}

	pending, active, sessions, err := s.loadCycleSnapshots(ctx, &result)
	if err != nil {
		return result, errors.Join(append(errs, err)...)
	}
	pauseState, err := s.schedulerPause(ctx)
	if err != nil {
		errs = append(errs, err)
	}
	if pauseState.Paused {
		result.Paused = true
		s.recordCycle(now, result)
		s.logger.Warn(
			"scheduler.paused",
			"pending_runs",
			result.PendingRuns,
			"active_runs",
			result.ActiveRuns,
			"sessions_scanned",
			result.SessionsScanned,
		)
		return result, errors.Join(errs...)
	}

	selection := s.selectWakeTargets(now, pauseState.UpdatedAt, pending, sessions, active)
	applySelection(&result, selection)
	errs = append(errs, s.dispatchWakeTargets(ctx, now, selection.targets, &result)...)
	if s.starvation != nil && s.escalator != nil {
		errs = append(
			errs,
			s.runConvergence(ctx, now, pauseState.UpdatedAt, selection.convergenceCandidates, &result)...,
		)
	}

	s.recordCycle(now, result)
	if result.NoMatchRuns > 0 {
		s.logger.Info("scheduler.wake.no_match", "runs", result.NoMatchRunIDs)
	}
	if result.CapacityWaitingRuns > 0 {
		s.logger.Info(
			"scheduler.capacity_waiting",
			"count", result.CapacityWaitingRuns,
			"run_ids", result.CapacityWaitingRunIDs,
			"reasons", selection.capacityWaitingReasons,
		)
	}
	if result.StarvedRuns > 0 {
		s.logger.Warn(
			"scheduler.wake.starved",
			"runs", result.StarvedRunIDs,
			"min_queued_age_ms", s.starveThresholds.MinQueuedAge.Milliseconds(),
		)
	}
	return result, errors.Join(errs...)
}

func (s *Scheduler) schedulerPause(ctx context.Context) (taskpkg.SchedulerPauseState, error) {
	if s.pauseStore == nil {
		return taskpkg.SchedulerPauseState{}, nil
	}
	state, err := s.pauseStore.GetSchedulerPause(ctx)
	if err != nil {
		return taskpkg.SchedulerPauseState{}, fmt.Errorf("scheduler: read pause state: %w", err)
	}
	return state, nil
}

func (s *Scheduler) sweepExpiredLeases(ctx context.Context, now time.Time, result *CycleResult) []error {
	recovered, err := s.tasks.RecoverExpiredRunLeases(ctx, taskpkg.ExpiredLeaseRecovery{
		Now:    now,
		Reason: s.sweepReason,
		Limit:  s.sweepLimit,
	}, s.actor)
	if err != nil {
		s.recordRecoveryError(err)
		s.logger.Warn("scheduler.lease_sweep.error", "error", err)
		return []error{fmt.Errorf("scheduler: recover expired leases: %w", err)}
	}

	result.RecoveredLeases = len(recovered)
	result.RecoveredRunIDs = recoveredRunIDs(recovered)
	s.recordRecovered(len(recovered), now)
	s.logger.Info("scheduler.lease_sweep", "recovered_leases", len(recovered))
	return nil
}

func (s *Scheduler) sweepExpiredTaskBlocks(ctx context.Context, now time.Time, result *CycleResult) []error {
	expired, err := s.tasks.ExpireTaskBlocks(ctx, now, s.actor)
	if err != nil {
		s.recordExpiryError(err)
		s.logger.Warn("scheduler.task_block_sweep.error", "error", err)
		return []error{fmt.Errorf("scheduler: expire task blocks: %w", err)}
	}
	result.ExpiredBlocks = len(expired.Blocks)
	result.ExpiredBlockIDs = expiredBlockIDs(expired.Blocks)
	s.recordExpiredBlocks(len(expired.Blocks), now)
	s.logger.Info("scheduler.task_block_sweep", "expired_blocks", len(expired.Blocks))
	return nil
}

func (s *Scheduler) loadCycleSnapshots(
	ctx context.Context,
	result *CycleResult,
) ([]RunSnapshot, []taskpkg.Run, []SessionSnapshot, error) {
	pending, err := s.tasks.PendingRuns(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("scheduler: list pending runs: %w", err)
	}
	active, err := s.tasks.ActiveRuns(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("scheduler: list active runs: %w", err)
	}
	sessions, err := s.sessions.Sessions(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("scheduler: list sessions: %w", err)
	}

	result.PendingRuns = len(pending)
	result.ActiveRuns = len(active)
	result.SessionsScanned = len(sessions)
	return pending, active, sessions, nil
}

func (s *Scheduler) loop(ctx context.Context) {
	ticker := s.clock.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
			if _, err := s.RunOnce(ctx); err != nil {
				s.logger.Warn("scheduler.cycle.error", "error", err)
			}
		}
	}
}

func recoveredRunIDs(results []taskpkg.ExpiredLeaseRecoveryResult) []string {
	ids := make([]string, 0, len(results))
	for idx := range results {
		if id := strings.TrimSpace(results[idx].Run.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func expiredBlockIDs(blocks []taskpkg.TaskBlock) []string {
	ids := make([]string, 0, len(blocks))
	for idx := range blocks {
		if id := strings.TrimSpace(blocks[idx].ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/providers"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

var reviewProviderRegistry = providers.DefaultRegistry

const runtimeEventBusBufferSize = 64

// Execute runs the prepared jobs and manages shutdown, retries, and summaries.
func Execute(
	ctx context.Context,
	jobs []model.Job,
	runArtifacts model.RunArtifacts,
	runJournal *journal.Journal,
	bus *events.Bus[events.Event],
	cfg *model.RuntimeConfig,
) (retErr error) {
	internalCfg := newConfig(cfg, runArtifacts)
	internalJobs := newJobs(jobs)
	bus = ensureRuntimeEventBus(internalCfg, runJournal, bus)
	startedAt := time.Now().UTC()
	defer func() {
		if err := closeRunJournal(runJournal); err != nil {
			retErr = errors.Join(retErr, err)
		}
	}()

	if err := submitRunEvent(
		ctx,
		runJournal,
		runArtifacts.RunID,
		events.EventKindRunStarted,
		kinds.RunStartedPayload{
			Mode:            string(internalCfg.mode),
			Name:            internalCfg.name,
			WorkspaceRoot:   internalCfg.workspaceRoot,
			IDE:             internalCfg.ide,
			Model:           internalCfg.model,
			ReasoningEffort: internalCfg.reasoningEffort,
			AccessMode:      internalCfg.accessMode,
			ArtifactsDir:    runArtifacts.RunDir,
			JobsTotal:       len(internalJobs),
		},
	); err != nil {
		return err
	}

	failed, failures, total, shutdownErr := executeJobsWithGracefulShutdown(
		ctx,
		internalJobs,
		internalCfg,
		runJournal,
		bus,
	)
	result := buildExecutionResult(internalCfg, internalJobs, failures, shutdownErr)
	if err := emitExecutionResult(internalCfg, result); err != nil {
		return err
	}
	if internalCfg.humanOutputEnabled() {
		summarizeResults(failed, failures, total)
	}
	refreshTaskMetaOnExit(internalCfg)
	if err := emitRunTerminalEvent(ctx, runJournal, result, internalJobs, startedAt); err != nil {
		return err
	}

	if shutdownErr != nil {
		if internalCfg.humanOutputEnabled() {
			fmt.Fprintf(os.Stderr, "\nShutdown interrupted: %v\n", shutdownErr)
		}
		return shutdownErr
	}
	if len(failures) > 0 {
		return errors.New("one or more groups failed; see logs above")
	}
	return nil
}

func ensureRuntimeEventBus(
	cfg *config,
	runJournal *journal.Journal,
	bus *events.Bus[events.Event],
) *events.Bus[events.Event] {
	if cfg != nil && cfg.uiEnabled() && bus == nil {
		bus = events.New[events.Event](runtimeEventBusBufferSize)
	}
	if runJournal != nil && bus != nil {
		runJournal.SetBus(bus)
	}
	return bus
}

type jobExecutionContext struct {
	ctx            context.Context
	cfg            *config
	jobs           []job
	total          int
	cwd            string
	logger         *slog.Logger
	journal        *journal.Journal
	bus            *events.Bus[events.Event]
	ui             uiSession
	sem            chan struct{}
	aggregateUsage model.Usage
	aggregateMu    sync.Mutex
	failed         int32
	failures       []failInfo
	failuresMu     sync.Mutex
	completed      int32
	wg             sync.WaitGroup
	clientsMu      sync.Mutex
	activeClients  map[agent.Client]struct{}
}

type executorState string

const (
	executorStateInitializing executorState = "initializing"
	executorStateRunning      executorState = "running"
	executorStateDraining     executorState = "draining"
	executorStateForcing      executorState = "forcing"
	executorStateShutdown     executorState = "shutdown"
	executorStateTerminated   executorState = "terminated"
)

type shutdownRequest struct {
	force  bool
	source shutdownSource
}

type executorController struct {
	ctx              context.Context
	execCtx          *jobExecutionContext
	state            executorState
	shutdownState    shutdownState
	cancelJobs       context.CancelCauseFunc
	done             <-chan struct{}
	shutdownRequests chan shutdownRequest
}

func executeJobsWithGracefulShutdown(
	ctx context.Context,
	jobs []job,
	cfg *config,
	runJournal *journal.Journal,
	bus *events.Bus[events.Event],
) (int32, []failInfo, int, error) {
	execCtx, err := newJobExecutionContext(ctx, jobs, cfg, runJournal, bus)
	if err != nil {
		total := len(jobs)
		return 0, []failInfo{{err: err}}, total, nil
	}
	defer execCtx.cleanup()

	jobCtx, cancelJobs := context.WithCancelCause(ctx)
	controller := &executorController{
		ctx:              ctx,
		execCtx:          execCtx,
		state:            executorStateInitializing,
		cancelJobs:       cancelJobs,
		shutdownRequests: make(chan shutdownRequest, 4),
	}
	if execCtx.ui != nil {
		execCtx.ui.setQuitHandler(controller.requestShutdown)
	}
	execCtx.launchWorkers(jobCtx)
	controller.done = execCtx.waitChannel()
	return controller.awaitCompletion()
}

func (c *executorController) awaitCompletion() (int32, []failInfo, int, error) {
	c.state = executorStateRunning
	ctxDone := c.ctx.Done()
	var shutdownTimer *time.Timer
	var shutdownTimerCh <-chan time.Time

	for {
		select {
		case <-c.done:
			return c.handleDone(shutdownTimer)
		case req := <-c.shutdownRequests:
			if req.force {
				c.beginForce(req.source)
				continue
			}
			if c.beginDrain(req.source) {
				shutdownTimer, shutdownTimerCh = resetShutdownTimer(shutdownTimer)
			}
		case <-ctxDone:
			ctxDone = nil
			if c.beginDrain(shutdownSourceSignal) {
				shutdownTimer, shutdownTimerCh = resetShutdownTimer(shutdownTimer)
			}
		case <-shutdownTimerCh:
			shutdownTimerCh = nil
			c.beginForce(shutdownSourceTimer)
		}
	}
}

func (c *executorController) handleDone(shutdownTimer *time.Timer) (int32, []failInfo, int, error) {
	if shutdownTimer != nil {
		shutdownTimer.Stop()
	}
	if c.state == executorStateRunning {
		c.state = executorStateShutdown
		if err := c.execCtx.awaitUIAfterCompletion(); err != nil {
			c.state = executorStateTerminated
			return c.result(err)
		}
		c.execCtx.reportAggregateUsage()
		c.state = executorStateTerminated
		return c.result(nil)
	}
	c.emitShutdownFallback(
		"Controller shutdown complete after shutdown grace period (%v)\n",
		gracefulShutdownTimeout,
	)
	forced := c.state == executorStateForcing
	c.state = executorStateShutdown
	if err := c.execCtx.shutdownUI(); err != nil {
		c.state = executorStateTerminated
		return c.result(err)
	}
	c.execCtx.reportAggregateUsage()
	c.execCtx.emitShutdownTerminated(c.shutdownState, forced)
	c.state = executorStateTerminated
	return c.result(nil)
}

func resetShutdownTimer(current *time.Timer) (*time.Timer, <-chan time.Time) {
	if current != nil {
		current.Stop()
	}
	next := time.NewTimer(gracefulShutdownTimeout)
	return next, next.C
}

func (c *executorController) result(err error) (int32, []failInfo, int, error) {
	failed := atomic.LoadInt32(&c.execCtx.failed)
	return failed, c.execCtx.failures, c.execCtx.total, err
}

func (c *executorController) emitShutdownFallback(format string, args ...any) {
	if c == nil || c.execCtx == nil || c.execCtx.cfg == nil {
		return
	}
	if !c.execCtx.cfg.humanOutputEnabled() || c.execCtx.ui != nil {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
}

func (c *executorController) requestShutdown(req uiQuitRequest) {
	force := req == uiQuitRequestForce
	select {
	case c.shutdownRequests <- shutdownRequest{force: force, source: shutdownSourceUI}:
	default:
	}
}

func (c *executorController) beginDrain(source shutdownSource) bool {
	if c.state != executorStateRunning && c.state != executorStateInitializing {
		return false
	}
	c.emitShutdownFallback(
		"\nReceived shutdown request (%s) while executor in %s state; requesting drain...\n",
		source,
		c.state,
	)
	c.state = executorStateDraining
	if c.cancelJobs != nil {
		c.cancelJobs(context.Canceled)
	}
	now := time.Now()
	state := shutdownState{
		Phase:       shutdownPhaseDraining,
		Source:      source,
		RequestedAt: now,
		DeadlineAt:  now.Add(gracefulShutdownTimeout),
	}
	c.shutdownState = state
	c.execCtx.emitShutdownRequested(state)
	c.execCtx.publishShutdownStatus(state)
	return true
}

func (c *executorController) beginForce(source shutdownSource) {
	if c.state == executorStateForcing || c.state == executorStateShutdown || c.state == executorStateTerminated {
		return
	}
	if c.state == executorStateRunning || c.state == executorStateInitializing {
		if !c.beginDrain(source) {
			return
		}
	}
	c.emitShutdownFallback("Escalating shutdown via %s; forcing exit\n", source)
	c.state = executorStateForcing
	if c.cancelJobs != nil {
		c.cancelJobs(context.Canceled)
	}
	c.execCtx.forceActiveClients()
	requestedAt := c.shutdownState.RequestedAt
	if requestedAt.IsZero() {
		requestedAt = time.Now()
	}
	c.shutdownState = shutdownState{
		Phase:       shutdownPhaseForcing,
		Source:      source,
		RequestedAt: requestedAt,
		DeadlineAt:  c.shutdownState.DeadlineAt,
	}
}

type jobLifecycle struct {
	index          int
	job            *job
	execCtx        *jobExecutionContext
	state          jobPhase
	attempt        int
	startedAt      time.Time
	currentTimeout time.Duration
	lastExitCode   int
	lastFailure    *failInfo
}

func newJobLifecycle(index int, jb *job, execCtx *jobExecutionContext) *jobLifecycle {
	return &jobLifecycle{
		index:   index,
		job:     jb,
		execCtx: execCtx,
		state:   jobPhaseQueued,
	}
}

func (l *jobLifecycle) schedule() {
	l.state = jobPhaseScheduled
}

func (l *jobLifecycle) startAttempt(attempt int, maxAttempts int, timeout time.Duration) {
	l.attempt = attempt
	l.currentTimeout = timeout
	l.state = jobPhaseRunning
	if l.startedAt.IsZero() {
		l.startedAt = time.Now().UTC()
	}
	cfg := l.execConfig()
	if l.attempt == 1 {
		l.execCtx.submitEventOrWarn(
			events.EventKindJobStarted,
			kinds.JobStartedPayload{
				Index:       l.index,
				Attempt:     attempt,
				MaxAttempts: maxAttempts,
			},
		)
		notifyJobStart(
			false,
			cfg != nil && cfg.humanOutputEnabled() && l.execCtx.ui == nil,
			l.index,
			attempt,
			maxAttempts,
			l.job,
			l.configIDE(),
			l.configModel(),
			l.configAddDirs(),
			l.configReasoningEffort(),
			l.configAccessMode(),
		)
	}
}

func (l *jobLifecycle) markRetry(failure failInfo, nextAttempt int, maxAttempts int) {
	l.lastFailure = &failure
	l.lastExitCode = failure.exitCode
	l.state = jobPhaseRetrying
	l.attempt = nextAttempt
	l.job.exitCode = failure.exitCode
	l.job.failure = failure.err.Error()
	l.execCtx.submitEventOrWarn(
		events.EventKindJobRetryScheduled,
		kinds.JobRetryScheduledPayload{
			Index:       l.index,
			Attempt:     nextAttempt,
			MaxAttempts: maxAttempts,
			Reason:      failure.err.Error(),
		},
	)
}

func (l *jobLifecycle) markGiveUp(failure failInfo) {
	l.lastFailure = &failure
	l.lastExitCode = failure.exitCode
	l.state = jobPhaseFailed
	l.job.status = runStatusFailed
	l.job.exitCode = failure.exitCode
	l.job.failure = failure.err.Error()
	if l.lastFailure != nil {
		recordFailure(&l.execCtx.failuresMu, &l.execCtx.failures, *l.lastFailure)
	}
	atomic.AddInt32(&l.execCtx.failed, 1)
	l.execCtx.submitEventOrWarn(
		events.EventKindJobFailed,
		kinds.JobFailedPayload{
			Index:       l.index,
			Attempt:     l.attempt,
			MaxAttempts: l.maxAttempts(),
			CodeFile:    strings.Join(l.job.codeFiles, ", "),
			ExitCode:    l.lastExitCode,
			OutLog:      l.job.outLog,
			ErrLog:      l.job.errLog,
			Error:       l.job.failure,
		},
	)
	if l.lastFailure != nil && l.humanOutputEnabled() {
		fmt.Fprintf(
			os.Stderr,
			"\n❌ Job %d (%s) failed with exit code %d: %v\n",
			l.index+1,
			strings.Join(l.job.codeFiles, ", "),
			l.lastExitCode,
			l.lastFailure.err,
		)
	}
}

func (l *jobLifecycle) markSuccess() {
	if l.startedAt.IsZero() {
		l.startedAt = time.Now().UTC()
	}
	l.lastFailure = nil
	l.lastExitCode = 0
	l.state = jobPhaseSucceeded
	l.job.status = runStatusSucceeded
	l.job.exitCode = 0
	l.job.failure = ""
	l.execCtx.submitEventOrWarn(
		events.EventKindJobCompleted,
		kinds.JobCompletedPayload{
			Index:       l.index,
			Attempt:     l.attempt,
			MaxAttempts: l.maxAttempts(),
			ExitCode:    0,
			DurationMs:  time.Since(l.startedAt).Milliseconds(),
		},
	)
}

func (l *jobLifecycle) markCanceled(exitCode int) {
	l.lastExitCode = exitCode
	l.state = jobPhaseCanceled
	l.job.status = runStatusCanceled
	l.job.exitCode = exitCode
	if exitCode == exitCodeCanceled {
		l.lastFailure = &failInfo{
			codeFile: strings.Join(l.job.codeFiles, ", "),
			exitCode: exitCodeCanceled,
			outLog:   l.job.outLog,
			errLog:   l.job.errLog,
			err:      fmt.Errorf("job canceled by shutdown"),
		}
	} else {
		l.lastFailure = nil
	}
	if l.lastFailure != nil {
		l.job.failure = l.lastFailure.err.Error()
	}

	if l.lastFailure != nil {
		recordFailure(&l.execCtx.failuresMu, &l.execCtx.failures, *l.lastFailure)
	}
	atomic.AddInt32(&l.execCtx.failed, 1)
	reason := ""
	if l.lastFailure != nil && l.lastFailure.err != nil {
		reason = l.lastFailure.err.Error()
	}
	l.execCtx.submitEventOrWarn(
		events.EventKindJobCancelled,
		kinds.JobCancelledPayload{
			Index:       l.index,
			Attempt:     l.attempt,
			MaxAttempts: l.maxAttempts(),
			Reason:      reason,
		},
	)
	if l.lastFailure != nil && l.humanOutputEnabled() {
		fmt.Fprintf(
			os.Stderr,
			"\n⚠️ Job %d (%s) canceled: %v\n",
			l.index+1,
			strings.Join(l.job.codeFiles, ", "),
			l.lastFailure.err,
		)
	}
}

func (l *jobLifecycle) execConfig() *config {
	if l == nil || l.execCtx == nil {
		return nil
	}
	return l.execCtx.cfg
}

func (l *jobLifecycle) maxAttempts() int {
	cfg := l.execConfig()
	if cfg == nil {
		return 1
	}
	return atLeastOne(cfg.maxRetries + 1)
}

func (l *jobLifecycle) humanOutputEnabled() bool {
	cfg := l.execConfig()
	return cfg != nil && cfg.humanOutputEnabled()
}

func (l *jobLifecycle) configIDE() string {
	cfg := l.execConfig()
	if cfg == nil {
		return ""
	}
	return cfg.ide
}

func (l *jobLifecycle) configModel() string {
	cfg := l.execConfig()
	if cfg == nil {
		return ""
	}
	return cfg.model
}

func (l *jobLifecycle) configAddDirs() []string {
	cfg := l.execConfig()
	if cfg == nil {
		return nil
	}
	return cfg.addDirs
}

func (l *jobLifecycle) configReasoningEffort() string {
	cfg := l.execConfig()
	if cfg == nil {
		return ""
	}
	return cfg.reasoningEffort
}

func (l *jobLifecycle) configAccessMode() string {
	cfg := l.execConfig()
	if cfg == nil {
		return ""
	}
	return cfg.accessMode
}

type jobRunner struct {
	index     int
	job       *job
	execCtx   *jobExecutionContext
	lifecycle *jobLifecycle
}

func newJobRunner(index int, jb *job, execCtx *jobExecutionContext) *jobRunner {
	return &jobRunner{
		index:     index,
		job:       jb,
		execCtx:   execCtx,
		lifecycle: newJobLifecycle(index, jb, execCtx),
	}
}

func (r *jobRunner) run(ctx context.Context) {
	r.lifecycle.schedule()
	if r.execCtx.cfg.dryRun {
		r.lifecycle.markSuccess()
		return
	}

	maxAttempts := atLeastOne(r.execCtx.cfg.maxRetries + 1)
	timeout := r.execCtx.cfg.timeout
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			r.lifecycle.markCanceled(exitCodeCanceled)
			return
		}

		r.lifecycle.startAttempt(attempt, maxAttempts, timeout)
		result := r.executeAttempt(ctx, timeout)
		if result.Successful() {
			if err := r.runPostSuccessHook(ctx); err != nil {
				r.lifecycle.markGiveUp(failInfo{
					codeFile: strings.Join(r.job.codeFiles, ", "),
					exitCode: -1,
					outLog:   r.job.outLog,
					errLog:   r.job.errLog,
					err:      err,
				})
				return
			}
			r.lifecycle.markSuccess()
			return
		}
		nextTimeout, continueLoop := r.handleResult(attempt, maxAttempts, timeout, result)
		if !continueLoop {
			return
		}
		timeout = nextTimeout
	}
}

func (r *jobRunner) runPostSuccessHook(ctx context.Context) error {
	return r.execCtx.afterJobSuccess(ctx, r.job)
}

func (r *jobRunner) handleResult(
	attempt int,
	attempts int,
	timeout time.Duration,
	result jobAttemptResult,
) (time.Duration, bool) {
	if result.Successful() {
		r.lifecycle.markSuccess()
		return timeout, false
	}
	if result.IsCanceled() {
		r.lifecycle.markCanceled(result.exitCode)
		return timeout, false
	}
	if !result.NeedsRetry() || attempt == attempts {
		r.lifecycle.markGiveUp(r.ensureFailure(result, "job failed"))
		return timeout, false
	}
	nextTimeout := r.nextTimeout(timeout)
	nextAttempt := attempt + 1
	r.lifecycle.markRetry(r.ensureFailure(result, "retrying job"), nextAttempt, attempts)
	r.logRetry(nextAttempt, attempts, nextTimeout)
	return nextTimeout, true
}

func (r *jobRunner) ensureFailure(result jobAttemptResult, fallback string) failInfo {
	if result.failure != nil {
		return *result.failure
	}
	return failInfo{
		codeFile: strings.Join(r.job.codeFiles, ", "),
		exitCode: result.exitCode,
		outLog:   r.job.outLog,
		errLog:   r.job.errLog,
		err:      errors.New(fallback),
	}
}

func (r *jobRunner) executeAttempt(ctx context.Context, timeout time.Duration) jobAttemptResult {
	return executeJobWithTimeout(
		ctx,
		r.execCtx.cfg,
		r.job,
		r.execCtx.cwd,
		r.execCtx.ui != nil,
		r.index,
		timeout,
		r.execCtx.journal,
		&r.execCtx.aggregateUsage,
		&r.execCtx.aggregateMu,
		r.execCtx.trackClient,
	)
}

func (r *jobRunner) nextTimeout(current time.Duration) time.Duration {
	if current <= 0 {
		return current
	}
	next := time.Duration(float64(current) * r.execCtx.cfg.retryBackoffMultiplier)
	const maxTimeout = 30 * time.Minute
	if next > maxTimeout {
		return maxTimeout
	}
	return next
}

func (r *jobRunner) logRetry(attempt int, maxAttempts int, timeout time.Duration) {
	if r.execCtx.ui != nil {
		return
	}
	if !r.execCtx.cfg.humanOutputEnabled() {
		return
	}
	fmt.Fprintf(
		os.Stderr,
		"\n🔄 [%s] Job %d (%s) retry attempt %d/%d with timeout %v\n",
		time.Now().Format("15:04:05"),
		r.index+1,
		strings.Join(r.job.codeFiles, ", "),
		attempt,
		maxAttempts,
		timeout,
	)
}

func newJobExecutionContext(
	ctx context.Context,
	jobs []job,
	cfg *config,
	runJournal *journal.Journal,
	bus *events.Bus[events.Event],
) (*jobExecutionContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	execCtx := &jobExecutionContext{
		ctx:           ctx,
		cfg:           cfg,
		jobs:          jobs,
		total:         len(jobs),
		cwd:           cwd,
		logger:        runtimeLoggerFor(cfg, cfg.uiEnabled()),
		journal:       runJournal,
		bus:           bus,
		sem:           make(chan struct{}, atLeastOne(cfg.concurrent)),
		activeClients: make(map[agent.Client]struct{}),
	}
	for idx := range execCtx.jobs {
		execCtx.jobs[idx].outBuffer = newLineBuffer(cfg.tailLines)
		execCtx.jobs[idx].errBuffer = newLineBuffer(cfg.tailLines)
	}
	execCtx.ui = setupUI(ctx, execCtx.jobs, cfg, bus, cfg.uiEnabled())
	return execCtx, nil
}

func (j *jobExecutionContext) cleanup() {
	if err := j.shutdownUI(); err != nil {
		if j != nil && j.cfg.humanOutputEnabled() {
			fmt.Fprintf(os.Stderr, "UI shutdown error: %v\n", err)
		}
	}
}

func (j *jobExecutionContext) runtimeLogger() *slog.Logger {
	if j != nil && j.logger != nil {
		return j.logger
	}
	if j != nil {
		return runtimeLoggerFor(j.cfg, j.cfg != nil && j.cfg.uiEnabled())
	}
	return runtimeLogger(false)
}

func (j *jobExecutionContext) awaitUIAfterCompletion() error {
	if j.ui == nil {
		return nil
	}
	j.ui.closeEvents()
	return j.ui.wait()
}

func (j *jobExecutionContext) shutdownUI() error {
	if j.ui == nil {
		return nil
	}
	j.ui.closeEvents()
	j.ui.shutdown()
	return j.ui.wait()
}

func (j *jobExecutionContext) publishShutdownStatus(state shutdownState) {
	if state.Phase != shutdownPhaseDraining {
		return
	}
	j.submitEventOrWarn(
		events.EventKindShutdownDraining,
		kinds.ShutdownDrainingPayload{
			Source:      string(state.Source),
			RequestedAt: state.RequestedAt,
			DeadlineAt:  state.DeadlineAt,
		},
	)
}

func (j *jobExecutionContext) launchWorkers(jobCtx context.Context) {
	if j.cfg.mode == model.ExecutionModePRDTasks {
		j.launchSequentialTaskWorkers(jobCtx)
		return
	}
	for idx := range j.jobs {
		jb := &j.jobs[idx]
		j.wg.Add(1)
		go j.executeJob(jobCtx, idx, jb)
	}
}

func (j *jobExecutionContext) launchSequentialTaskWorkers(jobCtx context.Context) {
	if len(j.jobs) == 0 {
		return
	}
	j.wg.Add(len(j.jobs))
	go func() {
		for idx := range j.jobs {
			j.executeSequentialJob(jobCtx, idx, &j.jobs[idx])
		}
	}()
}

func (j *jobExecutionContext) executeSequentialJob(jobCtx context.Context, index int, jb *job) {
	defer func() {
		j.wg.Done()
		atomic.AddInt32(&j.completed, 1)
	}()

	newJobRunner(index, jb, j).run(jobCtx)
}

func (j *jobExecutionContext) executeJob(jobCtx context.Context, index int, jb *job) {
	defer func() {
		j.wg.Done()
		atomic.AddInt32(&j.completed, 1)
	}()

	if !j.acquireWorkerSlot(jobCtx) {
		newJobRunner(index, jb, j).run(jobCtx)
		return
	}
	defer j.releaseWorkerSlot()

	newJobRunner(index, jb, j).run(jobCtx)
}

func (j *jobExecutionContext) trackClient(client agent.Client) func() {
	if client == nil {
		return func() {}
	}
	j.clientsMu.Lock()
	if j.activeClients == nil {
		j.activeClients = make(map[agent.Client]struct{})
	}
	j.activeClients[client] = struct{}{}
	j.clientsMu.Unlock()
	return func() {
		j.clientsMu.Lock()
		delete(j.activeClients, client)
		j.clientsMu.Unlock()
	}
}

func (j *jobExecutionContext) forceActiveClients() {
	j.clientsMu.Lock()
	clients := make([]agent.Client, 0, len(j.activeClients))
	for client := range j.activeClients {
		clients = append(clients, client)
	}
	j.clientsMu.Unlock()

	for _, client := range clients {
		if err := client.Kill(); err != nil {
			j.runtimeLogger().Warn("failed to force-kill ACP client", "error", err)
		}
	}
}

func (j *jobExecutionContext) acquireWorkerSlot(ctx context.Context) bool {
	if j.sem == nil {
		return true
	}
	select {
	case j.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (j *jobExecutionContext) releaseWorkerSlot() {
	if j.sem == nil {
		return
	}
	<-j.sem
}

func (j *jobExecutionContext) waitChannel() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		j.wg.Wait()
		close(done)
	}()
	return done
}

func (j *jobExecutionContext) reportAggregateUsage() {
	if j == nil || !j.cfg.humanOutputEnabled() {
		return
	}
	j.aggregateMu.Lock()
	defer j.aggregateMu.Unlock()
	printAggregateUsage(&j.aggregateUsage)
}

func (j *jobExecutionContext) submitEvent(kind events.EventKind, payload any) error {
	if j == nil || j.journal == nil || j.cfg == nil {
		return nil
	}
	ctx := j.ctx
	if ctx == nil {
		return errors.New("job execution context missing context")
	}
	event, err := newRuntimeEvent(j.cfg.runArtifacts.RunID, kind, payload)
	if err != nil {
		return err
	}
	return j.journal.Submit(ctx, event)
}

func (j *jobExecutionContext) submitEventOrWarn(kind events.EventKind, payload any) {
	if err := j.submitEvent(kind, payload); err != nil {
		j.runtimeLogger().Warn("failed to submit runtime event", "kind", kind, "error", err)
	}
}

func (j *jobExecutionContext) emitShutdownRequested(state shutdownState) {
	j.submitEventOrWarn(
		events.EventKindShutdownRequested,
		kinds.ShutdownRequestedPayload{
			Source:      string(state.Source),
			RequestedAt: state.RequestedAt,
			DeadlineAt:  state.DeadlineAt,
		},
	)
}

func (j *jobExecutionContext) emitShutdownTerminated(state shutdownState, forced bool) {
	if !state.active() {
		return
	}
	j.submitEventOrWarn(
		events.EventKindShutdownTerminated,
		kinds.ShutdownTerminatedPayload{
			Source:      string(state.Source),
			RequestedAt: state.RequestedAt,
			DeadlineAt:  state.DeadlineAt,
			Forced:      forced,
		},
	)
}

func (j *jobExecutionContext) afterJobSuccess(ctx context.Context, jb *job) error {
	if j.cfg.mode == model.ExecutionModePRDTasks {
		return j.afterTaskJobSuccess(jb)
	}

	if j.cfg.mode != model.ExecutionModePRReview {
		return nil
	}
	return j.afterReviewJobSuccess(ctx, jb)
}

func (j *jobExecutionContext) afterTaskJobSuccess(jb *job) error {
	if strings.TrimSpace(j.cfg.tasksDir) == "" {
		return fmt.Errorf("missing tasks directory for task post-processing")
	}

	entry, err := singleTaskEntry(jb)
	if err != nil {
		return err
	}
	oldTask, err := prompt.ParseTaskFile(entry.Content)
	if err != nil {
		return fmt.Errorf("parse task file %s before completion: %w", entry.AbsPath, err)
	}
	if err := tasks.MarkTaskCompleted(j.cfg.tasksDir, entry.Name); err != nil {
		return err
	}
	j.submitEventOrWarn(
		events.EventKindTaskFileUpdated,
		kinds.TaskFileUpdatedPayload{
			TasksDir:  j.cfg.tasksDir,
			TaskName:  entry.Name,
			FilePath:  entry.AbsPath,
			OldStatus: oldTask.Status,
			NewStatus: "completed",
		},
	)

	meta, err := tasks.RefreshTaskMeta(j.cfg.tasksDir)
	if err != nil {
		return err
	}
	j.submitEventOrWarn(
		events.EventKindTaskMetadataRefreshed,
		kinds.TaskMetadataRefreshedPayload{
			TasksDir:  j.cfg.tasksDir,
			CreatedAt: meta.CreatedAt,
			UpdatedAt: meta.UpdatedAt,
			Total:     meta.Total,
			Completed: meta.Completed,
			Pending:   meta.Pending,
		},
	)
	j.runtimeLogger().Info(
		"updated task workflow metadata",
		"tasks_dir",
		j.cfg.tasksDir,
		"completed",
		meta.Completed,
		"pending",
		meta.Pending,
		"total",
		meta.Total,
	)
	return nil
}

func (j *jobExecutionContext) afterReviewJobSuccess(ctx context.Context, jb *job) error {
	if strings.TrimSpace(j.cfg.reviewsDir) == "" {
		return fmt.Errorf("missing reviews directory for review post-processing")
	}

	batchEntries := prompt.FlattenAndSortIssues(jb.groups, model.ExecutionModePRReview)
	if len(batchEntries) == 0 {
		return errors.New("missing review entries for review post-processing")
	}
	if err := reviews.FinalizeIssueStatuses(j.cfg.reviewsDir, batchEntries); err != nil {
		return err
	}
	issueIDs := make([]string, 0, len(batchEntries))
	for _, entry := range batchEntries {
		issueIDs = append(issueIDs, entry.Name)
	}
	j.submitEventOrWarn(
		events.EventKindReviewStatusFinalized,
		kinds.ReviewStatusFinalizedPayload{
			ReviewsDir: j.cfg.reviewsDir,
			IssueIDs:   issueIDs,
		},
	)

	resolvedIssues, err := collectNewlyResolvedIssues(jb.groups)
	if err != nil {
		return err
	}
	providerBackedIssues := filterResolvedIssuesWithProviderRefs(resolvedIssues)
	if err := j.resolveProviderBackedIssues(ctx, providerBackedIssues); err != nil {
		return err
	}

	meta, err := reviews.RefreshRoundMeta(j.cfg.reviewsDir)
	if err != nil {
		return err
	}
	j.submitEventOrWarn(
		events.EventKindReviewRoundRefreshed,
		kinds.ReviewRoundRefreshedPayload{
			ReviewsDir: j.cfg.reviewsDir,
			Provider:   meta.Provider,
			PR:         meta.PR,
			Round:      meta.Round,
			CreatedAt:  meta.CreatedAt,
			Total:      meta.Total,
			Resolved:   meta.Resolved,
			Unresolved: meta.Unresolved,
		},
	)
	j.runtimeLogger().Info(
		"updated review round metadata",
		"provider",
		meta.Provider,
		"pr",
		meta.PR,
		"round",
		meta.Round,
		"resolved",
		meta.Resolved,
		"unresolved",
		meta.Unresolved,
	)
	return nil
}

func singleTaskEntry(jb *job) (model.IssueEntry, error) {
	if jb == nil {
		return model.IssueEntry{}, errors.New("missing job for task post-processing")
	}

	entries := prompt.FlattenAndSortIssues(jb.groups, model.ExecutionModePRDTasks)
	if len(entries) != 1 {
		return model.IssueEntry{}, fmt.Errorf("expected exactly 1 task entry, got %d", len(entries))
	}
	return entries[0], nil
}

func (j *jobExecutionContext) resolveProviderBackedIssues(
	ctx context.Context,
	providerBackedIssues []provider.ResolvedIssue,
) error {
	if len(providerBackedIssues) == 0 {
		return nil
	}

	startedAt := time.Now().UTC()
	callID := fmt.Sprintf("%s-%d", strings.TrimSpace(j.cfg.provider), startedAt.UnixNano())
	j.emitProviderCallStarted(callID, len(providerBackedIssues))

	reviewProvider, err := j.lookupReviewProvider()
	if err != nil {
		return j.handleProviderResolveFailure(
			callID,
			providerBackedIssues,
			startedAt,
			err,
			"review provider integration unavailable; skipping remote issue resolution",
		)
	}

	if err := reviewProvider.ResolveIssues(ctx, j.cfg.pr, providerBackedIssues); err != nil {
		return j.handleProviderResolveFailure(
			callID,
			providerBackedIssues,
			startedAt,
			err,
			"review provider resolution completed with warnings",
		)
	}

	completedAt := time.Now().UTC()
	j.emitProviderCallCompleted(callID, startedAt, completedAt, 0)
	j.emitReviewIssueResolved(providerBackedIssues, true, completedAt)

	j.runtimeLogger().Info(
		"resolved review provider issues",
		"provider",
		j.cfg.provider,
		"pr",
		j.cfg.pr,
		"resolved_issues",
		len(providerBackedIssues),
	)
	return nil
}

func (j *jobExecutionContext) emitProviderCallStarted(callID string, issueCount int) {
	j.submitEventOrWarn(
		events.EventKindProviderCallStarted,
		kinds.ProviderCallStartedPayload{
			CallID:     callID,
			Provider:   j.cfg.provider,
			Method:     "resolve_issues",
			PR:         j.cfg.pr,
			IssueCount: issueCount,
		},
	)
}

func (j *jobExecutionContext) emitProviderCallCompleted(
	callID string,
	startedAt time.Time,
	completedAt time.Time,
	statusCode int,
) {
	j.submitEventOrWarn(
		events.EventKindProviderCallCompleted,
		kinds.ProviderCallCompletedPayload{
			CallID:     callID,
			Provider:   j.cfg.provider,
			Method:     "resolve_issues",
			StatusCode: statusCode,
			DurationMs: completedAt.Sub(startedAt).Milliseconds(),
		},
	)
}

func (j *jobExecutionContext) lookupReviewProvider() (provider.Provider, error) {
	registry := reviewProviderRegistry()
	return registry.Get(j.cfg.provider)
}

func (j *jobExecutionContext) handleProviderResolveFailure(
	callID string,
	providerBackedIssues []provider.ResolvedIssue,
	startedAt time.Time,
	err error,
	message string,
) error {
	j.emitProviderCallFailed(callID, startedAt, err)
	j.emitReviewIssueResolved(providerBackedIssues, false, time.Time{})
	j.runtimeLogger().Warn(
		message,
		"provider",
		j.cfg.provider,
		"pr",
		j.cfg.pr,
		"resolved_issues",
		len(providerBackedIssues),
		"error",
		err,
	)
	return nil
}

func (j *jobExecutionContext) emitProviderCallFailed(
	callID string,
	startedAt time.Time,
	err error,
) {
	j.submitEventOrWarn(
		events.EventKindProviderCallFailed,
		kinds.ProviderCallFailedPayload{
			CallID:     callID,
			Provider:   j.cfg.provider,
			Method:     "resolve_issues",
			StatusCode: providerStatusCode(err),
			DurationMs: time.Since(startedAt).Milliseconds(),
			Error:      err.Error(),
		},
	)
}

func (j *jobExecutionContext) emitReviewIssueResolved(
	issues []provider.ResolvedIssue,
	providerPosted bool,
	postedAt time.Time,
) {
	for _, issue := range issues {
		payload := kinds.ReviewIssueResolvedPayload{
			ReviewsDir:     j.cfg.reviewsDir,
			IssueID:        issueIDFromPath(issue.FilePath),
			FilePath:       issue.FilePath,
			Provider:       j.cfg.provider,
			PR:             j.cfg.pr,
			ProviderRef:    issue.ProviderRef,
			ProviderPosted: providerPosted,
		}
		if providerPosted {
			payload.PostedAt = postedAt
		}
		j.submitEventOrWarn(events.EventKindReviewIssueResolved, payload)
	}
}

func refreshTaskMetaOnExit(cfg *config) {
	if cfg == nil || cfg.mode != model.ExecutionModePRDTasks || strings.TrimSpace(cfg.tasksDir) == "" {
		return
	}

	meta, err := tasks.RefreshTaskMeta(cfg.tasksDir)
	if err != nil {
		runtimeLoggerFor(cfg, cfg != nil && cfg.uiEnabled()).Warn(
			"failed to refresh task workflow metadata at command exit",
			"tasks_dir",
			cfg.tasksDir,
			"error",
			err,
		)
		return
	}

	runtimeLoggerFor(cfg, cfg != nil && cfg.uiEnabled()).Info(
		"refreshed task workflow metadata at command exit",
		"tasks_dir",
		cfg.tasksDir,
		"completed",
		meta.Completed,
		"pending",
		meta.Pending,
		"total",
		meta.Total,
	)
}

func executeJobWithTimeout(
	ctx context.Context,
	cfg *config,
	j *job,
	cwd string,
	useUI bool,
	index int,
	timeout time.Duration,
	runJournal *journal.Journal,
	aggregateUsage *model.Usage,
	aggregateMu *sync.Mutex,
	trackClient func(agent.Client) func(),
) jobAttemptResult {
	emitHuman := cfg.humanOutputEnabled() && !useUI
	attemptCtx := ctx
	cancel := func(error) {}
	stopActivityWatchdog := func() {}
	var activity *activityMonitor
	if timeout > 0 {
		activity = newActivityMonitor()
		attemptCtx, cancel = context.WithCancelCause(ctx)
		stopActivityWatchdog = startACPActivityWatchdog(attemptCtx, activity, timeout, cancel)
	}
	defer func() {
		stopActivityWatchdog()
		cancel(nil)
	}()

	execution, err := setupSessionExecution(
		attemptCtx,
		cfg,
		j,
		cwd,
		useUI,
		cfg.humanOutputEnabled(),
		index,
		runJournal,
		aggregateUsage,
		aggregateMu,
		activity,
		runtimeLoggerFor(cfg, useUI),
		trackClient,
	)
	if err != nil {
		if timeout > 0 && isActivityTimeout(err) {
			return handleSessionTimeout(resolveTimeoutError(timeout, err), j, index, emitHuman, timeout)
		}
		fail := recordFailureWithContext(nil, j, nil, err, -1)
		return jobAttemptResult{
			status:    attemptStatusSetupFailed,
			exitCode:  -1,
			failure:   &fail,
			retryable: retryableSetupFailure(err),
		}
	}
	return executeSessionAndResolve(attemptCtx, timeout, execution, j, index, emitHuman)
}

type activityTimeoutError struct {
	timeout time.Duration
}

func (e *activityTimeoutError) Error() string {
	return fmt.Sprintf("activity timeout: no output received for %v", e.timeout)
}

func startACPActivityWatchdog(
	ctx context.Context,
	monitor *activityMonitor,
	timeout time.Duration,
	cancel context.CancelCauseFunc,
) func() {
	if monitor == nil || timeout <= 0 || cancel == nil {
		return func() {}
	}

	stopCh := make(chan struct{})
	var stopOnce sync.Once
	interval := timeout / 2
	if interval <= 0 || interval > activityCheckInterval {
		interval = activityCheckInterval
	}
	if interval < time.Millisecond {
		interval = time.Millisecond
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if monitor.timeSinceLastActivity() > timeout {
					cancel(&activityTimeoutError{timeout: timeout})
					return
				}
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			}
		}
	}()

	return func() {
		stopOnce.Do(func() {
			close(stopCh)
		})
	}
}

func createLogFile(path string) (*os.File, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func handleNilExecution(j *job, index int, emitHuman bool) jobAttemptResult {
	codeFileLabel := strings.Join(j.codeFiles, ", ")
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: -1,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      fmt.Errorf("failed to set up ACP session execution"),
	}
	if emitHuman {
		fmt.Fprintf(os.Stderr, "\n❌ Failed to set up job %d (%s): %v\n", index+1, codeFileLabel, failure.err)
	}
	return jobAttemptResult{status: attemptStatusSetupFailed, exitCode: -1, failure: &failure}
}

func executeSessionAndResolve(
	ctx context.Context,
	timeout time.Duration,
	execution *sessionExecution,
	j *job,
	index int,
	emitHuman bool,
) jobAttemptResult {
	if execution == nil || execution.session == nil {
		return handleNilExecution(j, index, emitHuman)
	}
	defer execution.close()

	streamErrCh := make(chan error, 1)
	go func() {
		streamErrCh <- streamSessionUpdates(execution.session, execution.handler)
	}()

	select {
	case <-execution.session.Done():
		streamErr := <-streamErrCh
		if streamErr != nil {
			if err := execution.handler.HandleCompletion(streamErr); err != nil {
				execution.logger.Warn("failed to finalize ACP session handler after stream error", "error", err)
			}
			appendLinesToBuffer(j.errBuffer, []string{"ACP session error: " + streamErr.Error()})
			return buildFailureResult(streamErr, -1, j, index, emitHuman)
		}

		sessionErr := execution.session.Err()
		if err := execution.handler.HandleCompletion(sessionErr); err != nil {
			execution.logger.Warn("failed to finalize ACP session handler", "error", err)
		}
		if sessionErr != nil {
			appendLinesToBuffer(j.errBuffer, []string{"ACP session error: " + sessionErr.Error()})
		}
		return handleSessionCompletion(ctx, sessionErr, timeout, j, index, emitHuman)
	case <-ctx.Done():
		cancelErr := context.Cause(ctx)
		if cancelErr == nil {
			cancelErr = ctx.Err()
		}
		if err := execution.handler.HandleCompletion(cancelErr); err != nil {
			execution.logger.Warn("failed to finalize ACP session handler after context cancellation", "error", err)
		}
		appendLinesToBuffer(j.errBuffer, []string{"ACP session error: " + cancelErr.Error()})
		if isSessionTimeout(ctx, cancelErr) {
			return handleSessionTimeout(
				resolveTimeoutError(timeout, cancelErr, context.Cause(ctx), ctx.Err()),
				j,
				index,
				emitHuman,
				timeout,
			)
		}
		return handleSessionCancellation(cancelErr, j, index, emitHuman)
	}
}

func streamSessionUpdates(session agent.Session, handler *sessionUpdateHandler) error {
	for update := range session.Updates() {
		if err := handler.HandleUpdate(update); err != nil {
			return err
		}
	}
	return nil
}

func handleSessionCompletion(
	ctx context.Context,
	sessionErr error,
	timeout time.Duration,
	j *job,
	index int,
	emitHuman bool,
) jobAttemptResult {
	if sessionErr == nil {
		return jobAttemptResult{status: attemptStatusSuccess, exitCode: 0}
	}

	if isSessionTimeout(ctx, sessionErr) {
		return handleSessionTimeout(
			resolveTimeoutError(timeout, sessionErr, context.Cause(ctx), ctx.Err()),
			j,
			index,
			emitHuman,
			timeout,
		)
	}
	if errors.Is(sessionErr, context.Canceled) {
		return handleSessionCancellation(sessionErr, j, index, emitHuman)
	}

	exitCode := sessionErrorCode(sessionErr)
	return buildFailureResult(sessionErr, exitCode, j, index, emitHuman)
}

func isSessionTimeout(ctx context.Context, err error) bool {
	return isActivityTimeout(err) ||
		isActivityTimeout(context.Cause(ctx)) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(ctx.Err(), context.DeadlineExceeded)
}

func isActivityTimeout(err error) bool {
	var timeoutErr *activityTimeoutError
	return errors.As(err, &timeoutErr)
}

func resolveTimeoutError(timeout time.Duration, errs ...error) error {
	for _, err := range errs {
		var timeoutErr *activityTimeoutError
		if errors.As(err, &timeoutErr) {
			return timeoutErr
		}
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return &activityTimeoutError{timeout: timeout}
}

func handleSessionCancellation(
	cancelErr error,
	j *job,
	index int,
	emitHuman bool,
) jobAttemptResult {
	if emitHuman {
		fmt.Fprintf(
			os.Stderr,
			"\nCanceling job %d (%s) due to shutdown signal\n",
			index+1,
			strings.Join(j.codeFiles, ", "),
		)
	}
	codeFileLabel := strings.Join(j.codeFiles, ", ")
	if cancelErr == nil {
		cancelErr = fmt.Errorf("job canceled by shutdown")
	}
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCodeCanceled,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      cancelErr,
	}
	return jobAttemptResult{status: attemptStatusCanceled, exitCode: exitCodeCanceled, failure: &failure}
}

func handleSessionTimeout(
	timeoutErr error,
	j *job,
	index int,
	emitHuman bool,
	timeout time.Duration,
) jobAttemptResult {
	logTimeoutMessage(index, j, timeout, emitHuman)
	codeFileLabel := strings.Join(j.codeFiles, ", ")
	if timeoutErr == nil {
		timeoutErr = fmt.Errorf("ACP session timeout after %v", timeout)
	}
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCodeTimeout,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      timeoutErr,
	}
	return jobAttemptResult{
		status:    attemptStatusTimeout,
		exitCode:  exitCodeTimeout,
		failure:   &failure,
		retryable: true,
	}
}

func logTimeoutMessage(index int, j *job, timeout time.Duration, emitHuman bool) {
	if emitHuman {
		fmt.Fprintf(
			os.Stderr,
			"\nJob %d (%s) timed out after %v of inactivity\n",
			index+1,
			strings.Join(j.codeFiles, ", "),
			timeout,
		)
	}
}

func buildFailureResult(err error, exitCode int, j *job, index int, emitHuman bool) jobAttemptResult {
	codeFileLabel := strings.Join(j.codeFiles, ", ")
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCode,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      err,
	}
	if emitHuman {
		fmt.Fprintf(os.Stderr, "\n❌ Job %d (%s) failed with code %d: %v\n", index+1, codeFileLabel, exitCode, err)
	}
	return jobAttemptResult{
		status:    attemptStatusFailure,
		exitCode:  exitCode,
		failure:   &failure,
		retryable: true,
	}
}

func retryableSetupFailure(err error) bool {
	var setupErr *agent.SessionSetupError
	if !errors.As(err, &setupErr) {
		return false
	}

	switch setupErr.Stage {
	case agent.SessionSetupStageStartProcess, agent.SessionSetupStageInitialize, agent.SessionSetupStageNewSession:
		return true
	default:
		return false
	}
}

func sessionErrorCode(err error) int {
	var sessionErr *agent.SessionError
	if errors.As(err, &sessionErr) {
		return sessionErr.Code
	}
	return -1
}

func recordFailureWithContext(
	failuresMu *sync.Mutex,
	j *job,
	failures *[]failInfo,
	err error,
	exitCode int,
) failInfo {
	codeFileLabel := strings.Join(j.codeFiles, ", ")
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCode,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      err,
	}
	recordFailure(failuresMu, failures, failure)
	return failure
}

func recordFailure(mu *sync.Mutex, list *[]failInfo, f failInfo) {
	if list == nil {
		return
	}
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	*list = append(*list, f)
}

func printAggregateUsage(usage *model.Usage) {
	if usage == nil || usage.Total() == 0 {
		return
	}
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("ACP Session Token Usage (Aggregate across all jobs)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  Input Tokens:          %s\n", formatNumber(usage.InputTokens))
	if usage.CacheReads > 0 {
		fmt.Printf("  Cache Reads:           %s\n", formatNumber(usage.CacheReads))
	}
	if usage.CacheWrites > 0 {
		fmt.Printf("  Cache Writes:          %s\n", formatNumber(usage.CacheWrites))
	}
	fmt.Printf("  Output Tokens:         %s\n", formatNumber(usage.OutputTokens))
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("  Total Tokens:          %s\n", formatNumber(usage.Total()))
	fmt.Println(strings.Repeat("=", 60))
}

func summarizeResults(failed int32, failures []failInfo, total int) {
	fmt.Printf(
		"\nExecution Summary:\n- Total Groups: %d\n- Success: %d\n- Failed: %d\n",
		total,
		total-int(failed),
		int(failed),
	)
	if len(failures) == 0 {
		return
	}
	fmt.Println("\nFailures:")
	for _, f := range failures {
		fmt.Printf(
			"- Group: %s\n  - Exit Code: %d\n  - Logs: %s (out), %s (err)\n",
			f.codeFile,
			f.exitCode,
			f.outLog,
			f.errLog,
		)
	}
}

func submitRunEvent(
	ctx context.Context,
	runJournal *journal.Journal,
	runID string,
	kind events.EventKind,
	payload any,
) error {
	if runJournal == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	event, err := newRuntimeEvent(runID, kind, payload)
	if err != nil {
		return err
	}
	return runJournal.Submit(ctx, event)
}

func emitRunTerminalEvent(
	ctx context.Context,
	runJournal *journal.Journal,
	result executionResult,
	jobs []job,
	startedAt time.Time,
) error {
	if runJournal == nil {
		return nil
	}

	var (
		succeeded int
		failed    int
		canceled  int
	)
	for idx := range jobs {
		switch jobs[idx].status {
		case runStatusSucceeded:
			succeeded++
		case runStatusCanceled:
			canceled++
		case runStatusFailed:
			failed++
		}
	}

	durationMs := time.Since(startedAt).Milliseconds()
	switch result.Status {
	case runStatusSucceeded:
		return submitRunEvent(
			ctx,
			runJournal,
			result.RunID,
			events.EventKindRunCompleted,
			kinds.RunCompletedPayload{
				ArtifactsDir:   result.ArtifactsDir,
				JobsTotal:      len(jobs),
				JobsSucceeded:  succeeded,
				JobsFailed:     failed,
				JobsCancelled:  canceled,
				DurationMs:     durationMs,
				ResultPath:     result.ResultPath,
				SummaryMessage: "completed",
			},
		)
	case runStatusCanceled:
		reason := strings.TrimSpace(result.Error)
		if reason == "" {
			reason = strings.TrimSpace(result.TeardownError)
		}
		return submitRunEvent(
			ctx,
			runJournal,
			result.RunID,
			events.EventKindRunCancelled,
			kinds.RunCancelledPayload{
				Reason:     reason,
				DurationMs: durationMs,
			},
		)
	default:
		errText := strings.TrimSpace(result.Error)
		if errText == "" {
			errText = strings.TrimSpace(result.TeardownError)
		}
		return submitRunEvent(
			ctx,
			runJournal,
			result.RunID,
			events.EventKindRunFailed,
			kinds.RunFailedPayload{
				ArtifactsDir: result.ArtifactsDir,
				DurationMs:   durationMs,
				Error:        errText,
				ResultPath:   result.ResultPath,
			},
		)
	}
}

func closeRunJournal(runJournal *journal.Journal) error {
	if runJournal == nil {
		return nil
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runJournal.Close(closeCtx); err != nil {
		return fmt.Errorf("close run journal: %w", err)
	}
	return nil
}

func atLeastOne(value int) int {
	if value < 1 {
		return 1
	}
	return value
}

func collectNewlyResolvedIssues(groups map[string][]model.IssueEntry) ([]provider.ResolvedIssue, error) {
	resolved := make([]provider.ResolvedIssue, 0)
	for _, entries := range groups {
		for _, entry := range entries {
			currentBody, err := os.ReadFile(entry.AbsPath)
			if err != nil {
				return nil, fmt.Errorf("read updated issue file %s: %w", entry.AbsPath, err)
			}
			currentContent := string(currentBody)
			currentResolved, err := prompt.IsReviewResolved(currentContent)
			if err != nil {
				return nil, fmt.Errorf("parse updated review issue %s: %w", entry.AbsPath, err)
			}
			previouslyResolved, err := prompt.IsReviewResolved(entry.Content)
			if err != nil {
				return nil, fmt.Errorf("parse original review issue %s: %w", entry.AbsPath, err)
			}
			if !currentResolved || previouslyResolved {
				continue
			}

			reviewContext, err := prompt.ParseReviewContext(currentContent)
			if err != nil {
				return nil, fmt.Errorf("parse review context for %s: %w", entry.AbsPath, err)
			}
			resolved = append(resolved, provider.ResolvedIssue{
				FilePath:    entry.AbsPath,
				ProviderRef: reviewContext.ProviderRef,
			})
		}
	}

	sort.SliceStable(resolved, func(i, j int) bool {
		return resolved[i].FilePath < resolved[j].FilePath
	})
	return resolved, nil
}

func filterResolvedIssuesWithProviderRefs(issues []provider.ResolvedIssue) []provider.ResolvedIssue {
	filtered := make([]provider.ResolvedIssue, 0, len(issues))
	for _, issue := range issues {
		if strings.TrimSpace(issue.ProviderRef) == "" {
			continue
		}
		filtered = append(filtered, issue)
	}
	return filtered
}

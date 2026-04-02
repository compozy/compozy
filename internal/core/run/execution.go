package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/providers"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
)

var reviewProviderRegistry = providers.DefaultRegistry

// Execute runs the prepared jobs and manages shutdown, retries, and summaries.
func Execute(ctx context.Context, jobs []model.Job, cfg *model.RuntimeConfig) error {
	internalCfg := newConfig(cfg)
	internalJobs := newJobs(jobs)

	failed, failures, total, shutdownErr := executeJobsWithGracefulShutdown(ctx, internalJobs, internalCfg)
	summarizeResults(failed, failures, total)
	refreshTaskMetaOnExit(internalCfg)

	if shutdownErr != nil {
		fmt.Fprintf(os.Stderr, "\nShutdown interrupted: %v\n", shutdownErr)
		return shutdownErr
	}
	if len(failures) > 0 {
		return errors.New("one or more groups failed; see logs above")
	}
	return nil
}

type jobExecutionContext struct {
	cfg            *config
	jobs           []job
	total          int
	cwd            string
	uiCh           chan uiMsg
	ui             uiSession
	sem            chan struct{}
	aggregateUsage TokenUsage
	aggregateMu    sync.Mutex
	failed         int32
	failures       []failInfo
	failuresMu     sync.Mutex
	completed      int32
	wg             sync.WaitGroup
}

type executorState string

const (
	executorStateInitializing executorState = "initializing"
	executorStateRunning      executorState = "running"
	executorStateDraining     executorState = "draining"
	executorStateShutdown     executorState = "shutdown"
	executorStateTerminated   executorState = "terminated"
)

type executorController struct {
	ctx        context.Context
	execCtx    *jobExecutionContext
	state      executorState
	cancelJobs context.CancelFunc
	done       <-chan struct{}
}

func executeJobsWithGracefulShutdown(ctx context.Context, jobs []job, cfg *config) (int32, []failInfo, int, error) {
	execCtx, err := newJobExecutionContext(ctx, jobs, cfg)
	if err != nil {
		total := len(jobs)
		return 0, []failInfo{{err: err}}, total, nil
	}
	defer execCtx.cleanup()

	jobCtx, cancelJobs := execCtx.launchWorkers(ctx)
	_ = jobCtx
	controller := &executorController{
		ctx:        ctx,
		execCtx:    execCtx,
		state:      executorStateInitializing,
		cancelJobs: cancelJobs,
		done:       execCtx.waitChannel(),
	}
	if execCtx.ui != nil {
		execCtx.ui.setQuitHandler(cancelJobs)
	}
	return controller.awaitCompletion()
}

func (c *executorController) awaitCompletion() (int32, []failInfo, int, error) {
	c.state = executorStateRunning
	select {
	case <-c.done:
		c.state = executorStateShutdown
		if err := c.execCtx.awaitUIAfterCompletion(); err != nil {
			c.state = executorStateTerminated
			return c.result(err)
		}
		c.execCtx.reportAggregateUsage()
		c.state = executorStateTerminated
		return c.result(nil)
	case <-c.ctx.Done():
		fmt.Fprintf(os.Stderr, "\nReceived shutdown signal while executor in %s state; requesting drain...\n", c.state)
		c.state = executorStateDraining
		if c.cancelJobs != nil {
			c.cancelJobs()
		}
		return c.awaitShutdownAfterCancel()
	}
}

func (c *executorController) awaitShutdownAfterCancel() (int32, []failInfo, int, error) {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(c.ctx), gracefulShutdownTimeout)
	defer shutdownCancel()

	select {
	case <-c.done:
		fmt.Fprintf(os.Stderr, "All jobs completed gracefully within %v while draining\n", gracefulShutdownTimeout)
		c.state = executorStateShutdown
		if err := c.execCtx.shutdownUI(); err != nil {
			c.state = executorStateTerminated
			return c.result(err)
		}
		c.execCtx.reportAggregateUsage()
		c.state = executorStateTerminated
		return c.result(nil)
	case <-shutdownCtx.Done():
		fmt.Fprintf(os.Stderr, "Shutdown timeout exceeded (%v), forcing exit\n", gracefulShutdownTimeout)
		c.state = executorStateTerminated
		if err := c.execCtx.shutdownUI(); err != nil {
			return c.result(fmt.Errorf("shutdown timeout exceeded and ui shutdown failed: %w", err))
		}
		return c.result(fmt.Errorf("shutdown timeout exceeded"))
	}
}

func (c *executorController) result(err error) (int32, []failInfo, int, error) {
	failed := atomic.LoadInt32(&c.execCtx.failed)
	return failed, c.execCtx.failures, c.execCtx.total, err
}

type jobLifecycle struct {
	index          int
	job            *job
	execCtx        *jobExecutionContext
	state          jobPhase
	attempt        int
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

func (l *jobLifecycle) startAttempt(attempt int, timeout time.Duration) {
	l.attempt = attempt
	l.currentTimeout = timeout
	l.state = jobPhaseRunning
	if l.attempt == 1 {
		notifyJobStart(
			l.execCtx.uiCh != nil,
			l.execCtx.uiCh,
			l.index,
			l.job,
			l.execCtx.cfg.ide,
			l.execCtx.cfg.model,
			l.execCtx.cfg.addDirs,
			l.execCtx.cfg.reasoningEffort,
		)
		return
	}
	if l.execCtx.uiCh != nil {
		l.execCtx.uiCh <- jobStartedMsg{Index: l.index}
	}
}

func (l *jobLifecycle) markRetry(failure failInfo) {
	l.lastFailure = &failure
	l.lastExitCode = failure.exitCode
	l.state = jobPhaseRetrying
}

func (l *jobLifecycle) markGiveUp(failure failInfo) {
	l.lastFailure = &failure
	l.lastExitCode = failure.exitCode
	l.state = jobPhaseFailed
	if l.lastFailure != nil {
		recordFailure(&l.execCtx.failuresMu, &l.execCtx.failures, *l.lastFailure)
	}
	atomic.AddInt32(&l.execCtx.failed, 1)
	if l.execCtx.uiCh != nil {
		l.execCtx.uiCh <- jobFinishedMsg{Index: l.index, Success: false, ExitCode: l.lastExitCode}
		if l.lastFailure != nil {
			l.execCtx.uiCh <- jobFailureMsg{Failure: *l.lastFailure}
		}
		return
	}
	if l.lastFailure != nil {
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
	l.lastFailure = nil
	l.lastExitCode = 0
	l.state = jobPhaseSucceeded
	if l.execCtx.uiCh != nil {
		l.execCtx.uiCh <- jobFinishedMsg{Index: l.index, Success: true, ExitCode: 0}
	}
}

func (l *jobLifecycle) markCanceled(exitCode int) {
	l.lastExitCode = exitCode
	l.state = jobPhaseCanceled
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
		recordFailure(&l.execCtx.failuresMu, &l.execCtx.failures, *l.lastFailure)
	}
	atomic.AddInt32(&l.execCtx.failed, 1)
	if l.execCtx.uiCh != nil {
		l.execCtx.uiCh <- jobFinishedMsg{Index: l.index, Success: false, ExitCode: exitCodeCanceled}
		if l.lastFailure != nil {
			l.execCtx.uiCh <- jobFailureMsg{Failure: *l.lastFailure}
		}
		return
	}
	if l.lastFailure != nil {
		fmt.Fprintf(
			os.Stderr,
			"\n⚠️ Job %d (%s) canceled: %v\n",
			l.index+1,
			strings.Join(l.job.codeFiles, ", "),
			l.lastFailure.err,
		)
	}
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

	attempts := maxInt(1, r.execCtx.cfg.maxRetries+1)
	timeout := r.execCtx.cfg.timeout
	for attempt := 1; attempt <= attempts; attempt++ {
		if ctx.Err() != nil {
			r.lifecycle.markCanceled(exitCodeCanceled)
			return
		}

		r.lifecycle.startAttempt(attempt, timeout)
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
		nextTimeout, continueLoop := r.handleResult(attempt, attempts, timeout, result)
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
	r.lifecycle.markRetry(r.ensureFailure(result, "retrying job"))
	r.logRetry(attempt, attempts-1, nextTimeout)
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
		r.execCtx.uiCh != nil,
		r.execCtx.uiCh,
		r.index,
		timeout,
		&r.execCtx.aggregateUsage,
		&r.execCtx.aggregateMu,
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

func (r *jobRunner) logRetry(attempt int, maxRetries int, timeout time.Duration) {
	if r.execCtx.uiCh != nil {
		return
	}
	fmt.Fprintf(
		os.Stderr,
		"\n🔄 [%s] Job %d (%s) retry attempt %d/%d with timeout %v\n",
		time.Now().Format("15:04:05"),
		r.index+1,
		strings.Join(r.job.codeFiles, ", "),
		attempt,
		maxRetries,
		timeout,
	)
}

func newJobExecutionContext(ctx context.Context, jobs []job, cfg *config) (*jobExecutionContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	execCtx := &jobExecutionContext{
		cfg:   cfg,
		jobs:  jobs,
		total: len(jobs),
		cwd:   cwd,
		sem:   make(chan struct{}, maxInt(1, cfg.concurrent)),
	}
	for idx := range execCtx.jobs {
		execCtx.jobs[idx].outBuffer = newLineBuffer(cfg.tailLines)
		execCtx.jobs[idx].errBuffer = newLineBuffer(cfg.tailLines)
	}
	execCtx.ui = setupUI(ctx, execCtx.jobs, cfg, !cfg.dryRun)
	if execCtx.ui != nil {
		execCtx.uiCh = execCtx.ui.events()
	}
	return execCtx, nil
}

func (j *jobExecutionContext) cleanup() {
	if err := j.shutdownUI(); err != nil {
		fmt.Fprintf(os.Stderr, "UI shutdown error: %v\n", err)
	}
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

func (j *jobExecutionContext) launchWorkers(ctx context.Context) (context.Context, context.CancelFunc) {
	jobCtx, cancel := context.WithCancel(ctx)
	for idx := range j.jobs {
		jb := &j.jobs[idx]
		j.wg.Add(1)
		j.sem <- struct{}{}
		go j.executeJob(jobCtx, idx, jb)
	}
	return jobCtx, cancel
}

func (j *jobExecutionContext) executeJob(jobCtx context.Context, index int, jb *job) {
	defer func() {
		<-j.sem
		j.wg.Done()
		atomic.AddInt32(&j.completed, 1)
	}()
	newJobRunner(index, jb, j).run(jobCtx)
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
	if j.cfg.ide != model.IDEClaude && j.cfg.ide != model.IDECursor &&
		j.cfg.ide != model.IDEDroid && j.cfg.ide != model.IDEOpenCode &&
		j.cfg.ide != model.IDEPi {
		return
	}
	j.aggregateMu.Lock()
	defer j.aggregateMu.Unlock()
	printAggregateTokenUsage(&j.aggregateUsage)
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
	if err := tasks.MarkTaskCompleted(j.cfg.tasksDir, entry.Name); err != nil {
		return err
	}

	meta, err := tasks.RefreshTaskMeta(j.cfg.tasksDir)
	if err != nil {
		return err
	}
	slog.Info(
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

	resolvedIssues, err := collectNewlyResolvedIssues(jb.groups)
	if err != nil {
		return err
	}
	providerBackedIssues := filterResolvedIssuesWithProviderRefs(resolvedIssues)
	j.resolveProviderBackedIssues(ctx, providerBackedIssues)

	meta, err := reviews.RefreshRoundMeta(j.cfg.reviewsDir)
	if err != nil {
		return err
	}
	slog.Info(
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
) {
	if len(providerBackedIssues) == 0 {
		return
	}

	registry := reviewProviderRegistry()
	reviewProvider, err := registry.Get(j.cfg.provider)
	if err != nil {
		slog.Warn(
			"review provider integration unavailable; skipping remote issue resolution",
			"provider",
			j.cfg.provider,
			"pr",
			j.cfg.pr,
			"resolved_issues",
			len(providerBackedIssues),
			"error",
			err,
		)
		return
	}

	if err := reviewProvider.ResolveIssues(ctx, j.cfg.pr, providerBackedIssues); err != nil {
		slog.Warn(
			"review provider resolution completed with warnings",
			"provider",
			j.cfg.provider,
			"pr",
			j.cfg.pr,
			"resolved_issues",
			len(providerBackedIssues),
			"error",
			err,
		)
		return
	}

	slog.Info(
		"resolved review provider issues",
		"provider",
		j.cfg.provider,
		"pr",
		j.cfg.pr,
		"resolved_issues",
		len(providerBackedIssues),
	)
}

func refreshTaskMetaOnExit(cfg *config) {
	if cfg == nil || cfg.mode != model.ExecutionModePRDTasks || strings.TrimSpace(cfg.tasksDir) == "" {
		return
	}

	meta, err := tasks.RefreshTaskMeta(cfg.tasksDir)
	if err != nil {
		slog.Warn(
			"failed to refresh task workflow metadata at command exit",
			"tasks_dir",
			cfg.tasksDir,
			"error",
			err,
		)
		return
	}

	slog.Info(
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
	uiCh chan uiMsg,
	index int,
	timeout time.Duration,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
) jobAttemptResult {
	cmd, outF, errF, monitor, err := setupCommandExecution(
		ctx, cfg, j, cwd, useUI, uiCh, index, aggregateUsage, aggregateMu,
	)
	if err != nil {
		fail := recordFailureWithContext(nil, j, nil, err, -1)
		return jobAttemptResult{status: attemptStatusSetupFailed, exitCode: -1, failure: &fail}
	}
	return executeCommandAndResolve(ctx, timeout, monitor, cmd, outF, errF, j, index, useUI)
}

func setupCommandExecution(
	ctx context.Context,
	cfg *config,
	j *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	index int,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
) (*exec.Cmd, *os.File, *os.File, *activityMonitor, error) {
	cmd := createIDECommand(ctx, cfg, j)
	if cmd == nil {
		return nil, nil, nil, nil, fmt.Errorf("create IDE command: unsupported ide %q", cfg.ide)
	}
	outF, errF, monitor, err := setupCommandIO(
		cmd,
		j,
		cwd,
		useUI,
		uiCh,
		index,
		cfg.ide,
		aggregateUsage,
		aggregateMu,
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("setup command IO: %w", err)
	}
	return cmd, outF, errF, monitor, nil
}

func createLogFile(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func handleNilCommand(j *job, index int) jobAttemptResult {
	codeFileLabel := strings.Join(j.codeFiles, ", ")
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: -1,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      fmt.Errorf("failed to set up command (see logs)"),
	}
	fmt.Fprintf(os.Stderr, "\n❌ Failed to set up job %d (%s): %v\n", index+1, codeFileLabel, failure.err)
	return jobAttemptResult{status: attemptStatusSetupFailed, exitCode: -1, failure: &failure}
}

func executeCommandAndResolve(
	ctx context.Context,
	timeout time.Duration,
	monitor *activityMonitor,
	cmd *exec.Cmd,
	outF *os.File,
	errF *os.File,
	j *job,
	index int,
	useUI bool,
) jobAttemptResult {
	if cmd == nil {
		return handleNilCommand(j, index)
	}
	defer func() {
		if outF != nil {
			outF.Close()
		}
		if errF != nil {
			errF.Close()
		}
	}()

	cmdDone := make(chan error, 1)
	cmdDoneSignal := make(chan struct{})
	go func() {
		cmdDone <- cmd.Run()
		close(cmdDoneSignal)
	}()

	activityTimeout := startActivityWatchdog(ctx, monitor, timeout, cmdDoneSignal)
	select {
	case err := <-cmdDone:
		return handleCommandCompletion(err, j, index, useUI)
	case <-activityTimeout:
		return handleActivityTimeout(cmd, cmdDone, j, index, useUI, timeout)
	case <-ctx.Done():
		return handleCommandCancellation(cmd, cmdDone, j, index, useUI)
	}
}

func startActivityWatchdog(
	ctx context.Context,
	monitor *activityMonitor,
	timeout time.Duration,
	cmdDone <-chan struct{},
) <-chan struct{} {
	if monitor == nil || timeout <= 0 {
		return nil
	}
	activityTimeout := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(activityCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if monitor.timeSinceLastActivity() > timeout {
					select {
					case activityTimeout <- struct{}{}:
					default:
					}
					return
				}
			case <-cmdDone:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return activityTimeout
}

func handleCommandCompletion(err error, j *job, index int, useUI bool) jobAttemptResult {
	if err != nil {
		ec := exitCodeOf(err)
		codeFileLabel := strings.Join(j.codeFiles, ", ")
		failure := failInfo{
			codeFile: codeFileLabel,
			exitCode: ec,
			outLog:   j.outLog,
			errLog:   j.errLog,
			err:      err,
		}
		if !useUI {
			fmt.Fprintf(os.Stderr, "\n❌ Job %d (%s) failed with exit code %d: %v\n", index+1, codeFileLabel, ec, err)
		}
		return jobAttemptResult{status: attemptStatusFailure, exitCode: ec, failure: &failure}
	}
	return jobAttemptResult{status: attemptStatusSuccess, exitCode: 0}
}

func handleCommandCancellation(
	cmd *exec.Cmd,
	cmdDone <-chan error,
	j *job,
	index int,
	useUI bool,
) jobAttemptResult {
	if !useUI {
		fmt.Fprintf(
			os.Stderr,
			"\nCanceling job %d (%s) due to shutdown signal\n",
			index+1,
			strings.Join(j.codeFiles, ", "),
		)
	}
	if cmd.Process != nil {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send SIGTERM to process: %v\n", err)
		}
		select {
		case <-cmdDone:
			if !useUI {
				fmt.Fprintf(os.Stderr, "Job %d terminated gracefully\n", index+1)
			}
		case <-time.After(processTerminationGracePeriod):
			if !useUI {
				fmt.Fprintf(os.Stderr, "Job %d did not terminate gracefully, force killing...\n", index+1)
			}
			if err := cmd.Process.Kill(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to kill process: %v\n", err)
			}
		}
	}
	codeFileLabel := strings.Join(j.codeFiles, ", ")
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCodeCanceled,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      fmt.Errorf("job canceled by shutdown"),
	}
	return jobAttemptResult{status: attemptStatusCanceled, exitCode: exitCodeCanceled, failure: &failure}
}

func handleActivityTimeout(
	cmd *exec.Cmd,
	cmdDone <-chan error,
	j *job,
	index int,
	useUI bool,
	timeout time.Duration,
) jobAttemptResult {
	logTimeoutMessage(index, j, timeout, useUI)
	terminateTimedOutProcess(cmd, cmdDone, index, useUI)
	codeFileLabel := strings.Join(j.codeFiles, ", ")
	timeoutErr := fmt.Errorf("activity timeout: no output received for %v", timeout)
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCodeTimeout,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      timeoutErr,
	}
	return jobAttemptResult{status: attemptStatusTimeout, exitCode: exitCodeTimeout, failure: &failure}
}

func logTimeoutMessage(index int, j *job, timeout time.Duration, useUI bool) {
	if !useUI {
		fmt.Fprintf(
			os.Stderr,
			"\nJob %d (%s) timed out after %v of inactivity\n",
			index+1,
			strings.Join(j.codeFiles, ", "),
			timeout,
		)
	}
}

func terminateTimedOutProcess(cmd *exec.Cmd, cmdDone <-chan error, index int, useUI bool) {
	if cmd.Process == nil {
		return
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !useUI {
		fmt.Fprintf(os.Stderr, "Failed to send SIGTERM to process: %v\n", err)
	}
	waitForProcessTermination(cmdDone, cmd, index, useUI)
}

func waitForProcessTermination(cmdDone <-chan error, cmd *exec.Cmd, index int, useUI bool) {
	select {
	case <-cmdDone:
		if !useUI {
			fmt.Fprintf(os.Stderr, "Job %d terminated gracefully after timeout\n", index+1)
		}
	case <-time.After(processTerminationGracePeriod):
		forceKillProcess(cmd, index, useUI)
	}
}

func forceKillProcess(cmd *exec.Cmd, index int, useUI bool) {
	if !useUI {
		fmt.Fprintf(os.Stderr, "Job %d did not terminate gracefully, force killing...\n", index+1)
	}
	if err := cmd.Process.Kill(); err != nil && !useUI {
		fmt.Fprintf(os.Stderr, "Failed to kill process: %v\n", err)
	}
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

func printAggregateTokenUsage(usage *TokenUsage) {
	if usage == nil || usage.Total() == 0 {
		return
	}
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Claude API Token Usage (Aggregate across all jobs)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  Input Tokens:          %s\n", formatNumber(usage.InputTokens))
	if usage.CacheReadTokens > 0 {
		fmt.Printf("  Cache Read Tokens:     %s\n", formatNumber(usage.CacheReadTokens))
	}
	if usage.CacheCreationTokens > 0 {
		fmt.Printf("  Cache Creation Tokens: %s\n", formatNumber(usage.CacheCreationTokens))
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

func exitCodeOf(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(interface{ ExitStatus() int }); ok {
			return status.ExitStatus()
		}
		return exitErr.ExitCode()
	}
	return -1
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

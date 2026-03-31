package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/compozy/looper/internal/looper/agent"
	"github.com/compozy/looper/internal/looper/model"
	"github.com/compozy/looper/internal/looper/prompt"
	"github.com/compozy/looper/internal/looper/provider"
	"github.com/compozy/looper/internal/looper/providers"
	"github.com/compozy/looper/internal/looper/reviews"
)

var reviewProviderRegistry = providers.DefaultRegistry

type signalServer interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Port() int
}

var newSignalServerFunc = func(port int, eventCh chan<- SignalEvent, jobIDs []string) signalServer {
	return NewSignalServer(port, eventCh, jobIDs)
}

var (
	buildJobCommandFunc   = buildJobCommand
	waitForReadyFunc      = waitForReady
	sendComposerInputFunc = sendComposerInput
)

// Execute runs the prepared jobs and manages shutdown, retries, and summaries.
func Execute(ctx context.Context, jobs []model.Job, cfg *model.RuntimeConfig) error {
	internalCfg := newConfig(cfg)
	internalJobs := newJobs(jobs)

	failed, failures, total, shutdownErr := executeJobsWithGracefulShutdown(ctx, internalJobs, internalCfg)
	summarizeResults(failed, failures, total)
	if shutdownErr != nil {
		fmt.Fprintf(os.Stderr, "\nShutdown interrupted: %v\n", shutdownErr)
		return shutdownErr
	}
	if len(failures) > 0 {
		return errors.New("one or more groups failed; see logs above")
	}
	return nil
}

type terminalRuntime struct {
	terminal *Terminal
	outFile  *os.File
	errFile  *os.File
}

type jobExecutionContext struct {
	cfg    *config
	jobs   []job
	total  int
	cwd    string
	uiCh   chan uiMsg
	uiProg *tea.Program
	uiDone <-chan struct{}

	interactive bool
	sem         chan struct{}

	failed     int32
	failures   []failInfo
	failuresMu sync.Mutex
	completed  int32
	wg         sync.WaitGroup

	signalServer       signalServer
	signalEvents       chan SignalEvent
	uiSignalEvents     chan SignalEvent
	jobDoneSignals     map[string]chan SignalEvent
	signalServerErr    chan error
	signalDispatchDone chan struct{}

	terminalMu       sync.Mutex
	terminalRuntimes []*terminalRuntime
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
	return controller.awaitCompletion()
}

func (c *executorController) awaitCompletion() (int32, []failInfo, int, error) {
	c.state = executorStateRunning
	select {
	case <-c.done:
		if c.execCtx.uiDone != nil {
			return c.awaitUIExitAfterJobs()
		}
		c.state = executorStateShutdown
		c.state = executorStateTerminated
		return c.result(nil)
	case <-c.execCtx.uiDone:
		c.state = executorStateDraining
		if c.cancelJobs != nil {
			c.cancelJobs()
		}
		return c.awaitShutdownAfterCancel()
	case <-c.ctx.Done():
		fmt.Fprintf(os.Stderr, "\nReceived shutdown signal while executor in %s state; requesting drain...\n", c.state)
		c.state = executorStateDraining
		if c.cancelJobs != nil {
			c.cancelJobs()
		}
		return c.awaitShutdownAfterCancel()
	}
}

func (c *executorController) awaitUIExitAfterJobs() (int32, []failInfo, int, error) {
	c.state = executorStateShutdown
	select {
	case <-c.execCtx.uiDone:
	case <-c.ctx.Done():
	}
	c.state = executorStateTerminated
	return c.result(nil)
}

func (c *executorController) awaitShutdownAfterCancel() (int32, []failInfo, int, error) {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(c.ctx), gracefulShutdownTimeout)
	defer shutdownCancel()

	select {
	case <-c.done:
		fmt.Fprintf(os.Stderr, "All jobs completed gracefully within %v while draining\n", gracefulShutdownTimeout)
		c.state = executorStateShutdown
		c.state = executorStateTerminated
		return c.result(nil)
	case <-shutdownCtx.Done():
		fmt.Fprintf(os.Stderr, "Shutdown timeout exceeded (%v), forcing exit\n", gracefulShutdownTimeout)
		c.state = executorStateTerminated
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
	if l.execCtx.interactive {
		return
	}
	if l.attempt == 1 {
		notifyJobStart(l.execCtx.uiCh != nil, l.execCtx.uiCh, l.index, l.job, l.execCtx.cfg)
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
			"\nJob %d (%s) failed with exit code %d: %v\n",
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
	if l.execCtx.uiCh != nil && !l.execCtx.interactive {
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

	attempts := maxAtLeastOne(r.execCtx.cfg.maxRetries + 1)
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
	if r.execCtx.interactive {
		return executeInteractiveJobWithTimeout(ctx, r.execCtx, r.job, r.index, timeout)
	}
	return executeLegacyJobWithTimeout(ctx, r.execCtx.cfg, r.job, r.index, r.execCtx.cwd, timeout)
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
		"\n[%s] Job %d (%s) retry attempt %d/%d with timeout %v\n",
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
		cfg:              cfg,
		jobs:             jobs,
		total:            len(jobs),
		cwd:              cwd,
		interactive:      usesInteractiveTerminal(cfg) && !cfg.dryRun,
		sem:              make(chan struct{}, maxAtLeastOne(cfg.concurrent)),
		terminalRuntimes: make([]*terminalRuntime, len(jobs)),
	}

	if execCtx.interactive {
		execCtx.signalEvents = make(chan SignalEvent, maxAtLeastOne(len(jobs))*2)
		execCtx.uiSignalEvents = make(chan SignalEvent, maxAtLeastOne(len(jobs))*2)
		execCtx.jobDoneSignals = make(map[string]chan SignalEvent, len(jobs))
		for _, jb := range jobs {
			execCtx.jobDoneSignals[jb.safeName] = make(chan SignalEvent, 1)
		}
		execCtx.signalServer = newSignalServerFunc(cfg.signalPort, execCtx.signalEvents, collectJobIDs(jobs))
		execCtx.startSignalDispatcher(ctx)
		if err := execCtx.startSignalServer(ctx); err != nil {
			execCtx.cleanup()
			return nil, err
		}
	}

	uiTerminals := make([]*Terminal, len(jobs))
	execCtx.uiCh, execCtx.uiProg, execCtx.uiDone = setupUI(
		ctx,
		jobs,
		uiTerminals,
		execCtx.uiSignalEvents,
		execCtx.interactive,
	)
	return execCtx, nil
}

func (j *jobExecutionContext) startSignalDispatcher(ctx context.Context) {
	signalEvents := j.signalEvents
	if signalEvents == nil {
		return
	}
	uiSignalEvents := j.uiSignalEvents
	jobDoneSignals := j.jobDoneSignals

	j.signalDispatchDone = make(chan struct{})
	go func() {
		defer close(j.signalDispatchDone)
		for {
			select {
			case ev, ok := <-signalEvents:
				if !ok {
					return
				}
				if uiSignalEvents != nil {
					select {
					case uiSignalEvents <- ev:
					default:
						slog.Warn("dropping UI signal event", "type", ev.Type, "job_id", ev.JobID)
					}
				}
				if ev.Type == SignalEventTypeDone {
					doneCh := jobDoneSignals[ev.JobID]
					if doneCh != nil {
						select {
						case doneCh <- ev:
						default:
						}
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (j *jobExecutionContext) startSignalServer(ctx context.Context) error {
	if j.signalServer == nil {
		return nil
	}

	j.signalServerErr = make(chan error, 1)
	go func() {
		j.signalServerErr <- j.signalServer.Start(ctx)
	}()

	if err := waitForSignalServerReady(ctx, j.signalServer.Port(), j.signalServerErr); err != nil {
		return err
	}

	slog.Info("signal server ready", "port", j.signalServer.Port())
	return nil
}

func (j *jobExecutionContext) cleanup() {
	j.closeAllTerminalRuntimes()
	j.shutdownSignalServer()
	if j.uiProg != nil {
		if j.uiCh != nil {
			close(j.uiCh)
		}
		time.Sleep(uiMessageDrainDelay)
		j.uiProg.Quit()
		if j.uiDone != nil {
			select {
			case <-j.uiDone:
			case <-time.After(time.Second):
			}
		}
	}
}

func (j *jobExecutionContext) shutdownSignalServer() {
	if j.signalServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer cancel()
		if err := j.signalServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("signal server shutdown failed", "error", err)
		}
	}

	if j.signalEvents != nil {
		close(j.signalEvents)
		j.signalEvents = nil
	}

	if j.signalDispatchDone != nil {
		select {
		case <-j.signalDispatchDone:
		case <-time.After(time.Second):
		}
	}

	if j.signalServerErr != nil {
		select {
		case err := <-j.signalServerErr:
			if err != nil {
				slog.Warn("signal server exited with error", "error", err)
			}
		default:
		}
	}

	if j.uiSignalEvents != nil {
		close(j.uiSignalEvents)
		j.uiSignalEvents = nil
	}
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

func (j *jobExecutionContext) afterJobSuccess(ctx context.Context, jb *job) error {
	if j.cfg.mode != model.ExecutionModePRReview {
		return nil
	}
	if strings.TrimSpace(j.cfg.reviewsDir) == "" {
		return fmt.Errorf("missing reviews directory for review post-processing")
	}

	resolvedIssues, err := collectNewlyResolvedIssues(jb.groups)
	if err != nil {
		return err
	}

	providerBackedIssues := filterResolvedIssuesWithProviderRefs(resolvedIssues)
	if len(providerBackedIssues) > 0 {
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
		} else {
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
			} else {
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
		}
	}

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

func executeInteractiveJobWithTimeout(
	ctx context.Context,
	execCtx *jobExecutionContext,
	jb *job,
	index int,
	timeout time.Duration,
) jobAttemptResult {
	term, monitor, setupFailure := prepareInteractiveRuntime(ctx, execCtx, jb, index)
	if setupFailure != nil {
		return *setupFailure
	}

	if result, ready := waitForTerminalAndSendComposer(ctx, execCtx, jb, index, term); !ready {
		return result
	}

	slog.Info("interactive job started", "job_id", jb.safeName, "index", index)

	var jobDone <-chan SignalEvent
	if doneCh := execCtx.jobDoneSignals[jb.safeName]; doneCh != nil {
		jobDone = doneCh
	}
	activityTimeout := startActivityWatchdog(ctx, monitor, timeout, term.processDone)

	select {
	case <-jobDone:
		slog.Info("job done signal received", "job_id", jb.safeName)
		return jobAttemptResult{status: attemptStatusSuccess, exitCode: 0}
	case <-term.outputDone:
		return handleInteractiveProcessExit(execCtx, index, jb, term)
	case <-term.processDone:
		return handleInteractiveProcessExit(execCtx, index, jb, term)
	case <-activityTimeout:
		return handleInteractiveTimeout(execCtx, index, jb, term, timeout)
	case <-ctx.Done():
		return handleInteractiveCancellation(execCtx, index, jb, term, ctx.Err())
	}
}

func prepareInteractiveRuntime(
	ctx context.Context,
	execCtx *jobExecutionContext,
	jb *job,
	index int,
) (*Terminal, *activityMonitor, *jobAttemptResult) {
	cmd := buildJobCommandFunc(ctx, execCtx.cfg, jb)
	if cmd == nil {
		result := handleNilCommand(jb, index)
		return nil, nil, &result
	}
	configureCommandEnvironment(cmd, execCtx.cwd, nil, execCtx.cfg.ide)

	outFile, err := createLogFile(jb.outLog)
	if err != nil {
		result := setupFailureResult(jb, fmt.Errorf("create output log: %w", err))
		return nil, nil, &result
	}
	errFile, err := createLogFile(jb.errLog)
	if err != nil {
		_ = outFile.Close()
		result := setupFailureResult(jb, fmt.Errorf("create error log: %w", err))
		return nil, nil, &result
	}

	monitor := newActivityMonitor()
	term := NewTerminal(defaultTerminalWidth, defaultTerminalHeight, jb.safeName)
	term.SetOutputMirror(newActivityWriter(io.MultiWriter(
		outFile,
		&terminalOutputWriter{index: index, uiCh: execCtx.uiCh},
	), monitor))

	if err := term.Start(cmd); err != nil {
		_ = outFile.Close()
		_ = errFile.Close()
		result := setupFailureResult(jb, fmt.Errorf("start terminal: %w", err))
		return nil, nil, &result
	}

	execCtx.setTerminalRuntime(index, term, outFile, errFile)
	if execCtx.uiCh != nil {
		execCtx.uiCh <- jobStartedMsg{Index: index, Terminal: term}
	}
	resetJobDoneSignal(execCtx.jobDoneSignals[jb.safeName])
	return term, monitor, nil
}

func waitForTerminalAndSendComposer(
	ctx context.Context,
	execCtx *jobExecutionContext,
	jb *job,
	index int,
	term *Terminal,
) (jobAttemptResult, bool) {
	if err := waitForReadyFunc(ctx, term.emu); err != nil {
		execCtx.closeTerminalRuntime(index)
		if errors.Is(err, context.Canceled) {
			return jobAttemptResult{status: attemptStatusCanceled, exitCode: exitCodeCanceled, failure: &failInfo{
				codeFile: strings.Join(jb.codeFiles, ", "),
				exitCode: exitCodeCanceled,
				outLog:   jb.outLog,
				errLog:   jb.errLog,
				err:      err,
			}}, false
		}
		return jobAttemptResult{status: attemptStatusFailure, exitCode: -1, failure: &failInfo{
			codeFile: strings.Join(jb.codeFiles, ", "),
			exitCode: -1,
			outLog:   jb.outLog,
			errLog:   jb.errLog,
			err:      fmt.Errorf("wait for terminal readiness: %w", err),
		}}, false
	}
	if execCtx.uiCh != nil {
		execCtx.uiCh <- terminalReadyMsg{Index: index}
	}

	composerMessage := buildComposerMessage(jb)
	if execCtx.uiCh != nil {
		execCtx.uiCh <- composerSendMsg{Index: index, Message: composerMessage}
	}
	if err := sendComposerInputFunc(term.pty, composerMessage); err != nil {
		execCtx.closeTerminalRuntime(index)
		return jobAttemptResult{status: attemptStatusFailure, exitCode: -1, failure: &failInfo{
			codeFile: strings.Join(jb.codeFiles, ", "),
			exitCode: -1,
			outLog:   jb.outLog,
			errLog:   jb.errLog,
			err:      fmt.Errorf("send composer input: %w", err),
		}}, false
	}
	return jobAttemptResult{}, true
}

func executeLegacyJobWithTimeout(
	ctx context.Context,
	cfg *config,
	jb *job,
	index int,
	cwd string,
	timeout time.Duration,
) jobAttemptResult {
	cmd := buildJobCommandFunc(ctx, cfg, jb)
	if cmd == nil {
		return handleNilCommand(jb, index)
	}
	configureCommandEnvironment(cmd, cwd, jb.prompt, cfg.ide)

	outFile, err := createLogFile(jb.outLog)
	if err != nil {
		return jobAttemptResult{status: attemptStatusSetupFailed, exitCode: -1, failure: &failInfo{
			codeFile: strings.Join(jb.codeFiles, ", "),
			exitCode: -1,
			outLog:   jb.outLog,
			errLog:   jb.errLog,
			err:      fmt.Errorf("create output log: %w", err),
		}}
	}
	errFile, err := createLogFile(jb.errLog)
	if err != nil {
		_ = outFile.Close()
		return jobAttemptResult{status: attemptStatusSetupFailed, exitCode: -1, failure: &failInfo{
			codeFile: strings.Join(jb.codeFiles, ", "),
			exitCode: -1,
			outLog:   jb.outLog,
			errLog:   jb.errLog,
			err:      fmt.Errorf("create error log: %w", err),
		}}
	}
	defer func() {
		_ = outFile.Close()
		_ = errFile.Close()
	}()

	monitor := newActivityMonitor()
	cmd.Stdout = newActivityWriter(io.MultiWriter(outFile, os.Stdout), monitor)
	cmd.Stderr = newActivityWriter(io.MultiWriter(errFile, os.Stderr), monitor)

	return executeCommandAndResolve(ctx, timeout, monitor, cmd, jb)
}

func (j *jobExecutionContext) setTerminalRuntime(index int, term *Terminal, outFile, errFile *os.File) {
	j.terminalMu.Lock()
	defer j.terminalMu.Unlock()
	j.terminalRuntimes[index] = &terminalRuntime{
		terminal: term,
		outFile:  outFile,
		errFile:  errFile,
	}
}

func (j *jobExecutionContext) closeTerminalRuntime(index int) {
	j.terminalMu.Lock()
	runtime := j.terminalRuntimes[index]
	j.terminalRuntimes[index] = nil
	j.terminalMu.Unlock()

	closeTerminalRuntime(runtime)
}

func (j *jobExecutionContext) closeAllTerminalRuntimes() {
	j.terminalMu.Lock()
	runtimes := make([]*terminalRuntime, len(j.terminalRuntimes))
	copy(runtimes, j.terminalRuntimes)
	for i := range j.terminalRuntimes {
		j.terminalRuntimes[i] = nil
	}
	j.terminalMu.Unlock()

	for _, runtime := range runtimes {
		closeTerminalRuntime(runtime)
	}
}

func closeTerminalRuntime(runtime *terminalRuntime) {
	if runtime == nil {
		return
	}
	if runtime.terminal != nil {
		_ = runtime.terminal.Close()
	}
	if runtime.outFile != nil {
		_ = runtime.outFile.Close()
	}
	if runtime.errFile != nil {
		_ = runtime.errFile.Close()
	}
}

func handleInteractiveProcessExit(execCtx *jobExecutionContext, index int, jb *job, term *Terminal) jobAttemptResult {
	execCtx.closeTerminalRuntime(index)

	err := term.ProcessError()
	exitCode := term.ExitCode()
	if err == nil {
		err = fmt.Errorf("job exited before sending done signal")
	}
	if exitCode == 0 {
		err = fmt.Errorf("job exited before sending done signal: %w", err)
	}

	return jobAttemptResult{status: attemptStatusFailure, exitCode: exitCode, failure: &failInfo{
		codeFile: strings.Join(jb.codeFiles, ", "),
		exitCode: exitCode,
		outLog:   jb.outLog,
		errLog:   jb.errLog,
		err:      err,
	}}
}

func handleInteractiveTimeout(
	execCtx *jobExecutionContext,
	index int,
	jb *job,
	term *Terminal,
	timeout time.Duration,
) jobAttemptResult {
	if term != nil {
		select {
		case <-term.outputDone:
			return handleInteractiveProcessExit(execCtx, index, jb, term)
		case <-term.processDone:
			return handleInteractiveProcessExit(execCtx, index, jb, term)
		default:
		}
	}

	execCtx.closeTerminalRuntime(index)
	timeoutErr := fmt.Errorf("activity timeout: no output received for %v", timeout)
	return jobAttemptResult{status: attemptStatusTimeout, exitCode: exitCodeTimeout, failure: &failInfo{
		codeFile: strings.Join(jb.codeFiles, ", "),
		exitCode: exitCodeTimeout,
		outLog:   jb.outLog,
		errLog:   jb.errLog,
		err:      timeoutErr,
	}}
}

func handleInteractiveCancellation(
	execCtx *jobExecutionContext,
	index int,
	jb *job,
	_ *Terminal,
	cancelErr error,
) jobAttemptResult {
	execCtx.closeTerminalRuntime(index)
	if cancelErr == nil {
		cancelErr = fmt.Errorf("job canceled by shutdown")
	}
	return jobAttemptResult{status: attemptStatusCanceled, exitCode: exitCodeCanceled, failure: &failInfo{
		codeFile: strings.Join(jb.codeFiles, ", "),
		exitCode: exitCodeCanceled,
		outLog:   jb.outLog,
		errLog:   jb.errLog,
		err:      cancelErr,
	}}
}

func executeCommandAndResolve(
	ctx context.Context,
	timeout time.Duration,
	monitor *activityMonitor,
	cmd *exec.Cmd,
	jb *job,
) jobAttemptResult {
	if cmd == nil {
		return jobAttemptResult{status: attemptStatusSetupFailed, exitCode: -1, failure: &failInfo{
			codeFile: strings.Join(jb.codeFiles, ", "),
			exitCode: -1,
			outLog:   jb.outLog,
			errLog:   jb.errLog,
			err:      fmt.Errorf("failed to set up command"),
		}}
	}

	cmdDone := make(chan error, 1)
	cmdDoneSignal := make(chan struct{})
	go func() {
		cmdDone <- cmd.Run()
		close(cmdDoneSignal)
	}()

	activityTimeout := startActivityWatchdog(ctx, monitor, timeout, cmdDoneSignal)
	select {
	case err := <-cmdDone:
		return handleCommandCompletion(err, jb)
	case <-activityTimeout:
		return handleLegacyActivityTimeout(cmd, cmdDone, jb, timeout)
	case <-ctx.Done():
		return handleLegacyCommandCancellation(cmd, cmdDone, jb)
	}
}

func startActivityWatchdog(
	ctx context.Context,
	monitor *activityMonitor,
	timeout time.Duration,
	done <-chan struct{},
) <-chan struct{} {
	if monitor == nil || timeout <= 0 {
		return nil
	}

	checkInterval := activityCheckInterval
	activityTimeout := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(checkInterval)
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
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return activityTimeout
}

func handleCommandCompletion(err error, jb *job) jobAttemptResult {
	if err != nil {
		ec := exitCodeOf(err)
		return jobAttemptResult{status: attemptStatusFailure, exitCode: ec, failure: &failInfo{
			codeFile: strings.Join(jb.codeFiles, ", "),
			exitCode: ec,
			outLog:   jb.outLog,
			errLog:   jb.errLog,
			err:      err,
		}}
	}
	return jobAttemptResult{status: attemptStatusSuccess, exitCode: 0}
}

func handleLegacyCommandCancellation(cmd *exec.Cmd, cmdDone <-chan error, jb *job) jobAttemptResult {
	terminateLegacyProcess(cmd, cmdDone, "cancel")
	return jobAttemptResult{status: attemptStatusCanceled, exitCode: exitCodeCanceled, failure: &failInfo{
		codeFile: strings.Join(jb.codeFiles, ", "),
		exitCode: exitCodeCanceled,
		outLog:   jb.outLog,
		errLog:   jb.errLog,
		err:      fmt.Errorf("job canceled by shutdown"),
	}}
}

func handleLegacyActivityTimeout(
	cmd *exec.Cmd,
	cmdDone <-chan error,
	jb *job,
	timeout time.Duration,
) jobAttemptResult {
	terminateLegacyProcess(cmd, cmdDone, "activity timeout")
	return jobAttemptResult{status: attemptStatusTimeout, exitCode: exitCodeTimeout, failure: &failInfo{
		codeFile: strings.Join(jb.codeFiles, ", "),
		exitCode: exitCodeTimeout,
		outLog:   jb.outLog,
		errLog:   jb.errLog,
		err:      fmt.Errorf("activity timeout: no output received for %v", timeout),
	}}
}

func waitForSignalServerReady(ctx context.Context, port int, startErr <-chan error) error {
	if port <= 0 {
		return fmt.Errorf("invalid signal server port %d", port)
	}

	startupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 200 * time.Millisecond}
	url := fmt.Sprintf("http://localhost:%d/health", port)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-startupCtx.Done():
			select {
			case err := <-startErr:
				if err != nil {
					return err
				}
			default:
			}
			return fmt.Errorf("signal server did not become ready within %v", 5*time.Second)
		case err := <-startErr:
			if err != nil {
				return err
			}
			return errors.New("signal server exited before becoming ready")
		case <-ticker.C:
			req, err := http.NewRequestWithContext(startupCtx, http.MethodGet, url, http.NoBody)
			if err != nil {
				return err
			}
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

type terminalOutputWriter struct {
	index int
	uiCh  chan<- uiMsg
}

func (w *terminalOutputWriter) Write(p []byte) (int, error) {
	if w.uiCh == nil || len(p) == 0 {
		return len(p), nil
	}

	data := append([]byte(nil), p...)
	select {
	case w.uiCh <- terminalOutputMsg{Index: w.index, Data: data}:
	default:
	}
	return len(p), nil
}

func buildComposerMessage(jb *job) string {
	if strings.TrimSpace(jb.outPromptPath) == "" {
		return "Read and execute the prepared task prompt."
	}
	return fmt.Sprintf("Read and execute the task described in %s", jb.outPromptPath)
}

func usesInteractiveTerminal(cfg *config) bool {
	return cfg != nil && cfg.ide == model.IDEClaude
}

func collectJobIDs(jobs []job) []string {
	jobIDs := make([]string, 0, len(jobs))
	for _, jb := range jobs {
		if strings.TrimSpace(jb.safeName) == "" {
			continue
		}
		jobIDs = append(jobIDs, jb.safeName)
	}
	return jobIDs
}

func createLogFile(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func setupFailureResult(jb *job, err error) jobAttemptResult {
	return jobAttemptResult{status: attemptStatusSetupFailed, exitCode: -1, failure: &failInfo{
		codeFile: strings.Join(jb.codeFiles, ", "),
		exitCode: -1,
		outLog:   jb.outLog,
		errLog:   jb.errLog,
		err:      err,
	}}
}

func resetJobDoneSignal(signalCh chan SignalEvent) {
	if signalCh == nil {
		return
	}
	for {
		select {
		case <-signalCh:
		default:
			return
		}
	}
}

func handleNilCommand(jb *job, index int) jobAttemptResult {
	codeFileLabel := strings.Join(jb.codeFiles, ", ")
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: -1,
		outLog:   jb.outLog,
		errLog:   jb.errLog,
		err:      fmt.Errorf("failed to set up command (see logs)"),
	}
	fmt.Fprintf(os.Stderr, "\nFailed to set up job %d (%s): %v\n", index+1, codeFileLabel, failure.err)
	return jobAttemptResult{status: attemptStatusSetupFailed, exitCode: -1, failure: &failure}
}

func notifyJobStart(useUI bool, uiCh chan uiMsg, index int, jb *job, cfg *config) {
	if useUI {
		uiCh <- jobStartedMsg{Index: index}
		return
	}

	commandCfg := buildIDECommandConfig(cfg, jb)
	shellCmd := agent.BuildShellCommandString(commandCfg)
	ideName := agent.DisplayName(cfg.ide)
	totalIssues := countTotalIssues(jb)
	codeFileLabel := formatCodeFileLabel(jb.codeFiles)
	fmt.Printf(
		"\n=== Running %s (non-interactive) for batch: %s (%d issues)\n$ %s\n",
		ideName,
		codeFileLabel,
		totalIssues,
		shellCmd,
	)
}

func countTotalIssues(jb *job) int {
	total := 0
	for _, items := range jb.groups {
		total += len(items)
	}
	return total
}

func formatCodeFileLabel(codeFiles []string) string {
	label := strings.Join(codeFiles, ", ")
	if len(codeFiles) > 1 {
		return fmt.Sprintf("%d files: %s", len(codeFiles), label)
	}
	return label
}

func buildJobCommand(ctx context.Context, cfg *config, jb *job) *exec.Cmd {
	return agent.Command(ctx, buildIDECommandConfig(cfg, jb))
}

func buildIDECommandConfig(cfg *config, jb *job) *model.RuntimeConfig {
	commandCfg := &model.RuntimeConfig{
		IDE:             cfg.ide,
		Model:           cfg.model,
		AddDirs:         cfg.addDirs,
		ReasoningEffort: cfg.reasoningEffort,
		SystemPrompt:    "",
	}
	if cfg.ide == model.IDEClaude {
		commandCfg.SystemPrompt = buildClaudeSystemPrompt(cfg.mode, jb.safeName, cfg.signalPort, cfg.reasoningEffort)
	}
	return commandCfg
}

func buildClaudeSystemPrompt(
	mode model.ExecutionMode,
	jobID string,
	signalPort int,
	reasoningEffort string,
) string {
	sections := make([]string, 0, 2)
	if thinking := strings.TrimSpace(prompt.ClaudeReasoningPrompt(reasoningEffort)); thinking != "" {
		sections = append(sections, thinking)
	}
	sections = append(sections, prompt.BuildSystemPrompt(mode, jobID, signalPort))
	return strings.Join(sections, "\n\n")
}

func configureCommandEnvironment(cmd *exec.Cmd, cwd string, promptBytes []byte, ideType string) {
	cmd.Dir = cwd
	if promptBytes != nil {
		cmd.Stdin = bytes.NewReader(promptBytes)
	}
	env := append([]string{}, os.Environ()...)
	if len(cmd.Env) > 0 {
		env = append(env, cmd.Env...)
	}
	env = append(env,
		"FORCE_COLOR=1",
		"CLICOLOR_FORCE=1",
		"TERM=xterm-256color",
	)
	if ideType == model.IDEClaude {
		env = append(env, "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1")
	}
	cmd.Env = env
}

func recordFailureWithContext(
	failuresMu *sync.Mutex,
	jb *job,
	failures *[]failInfo,
	err error,
	exitCode int,
) failInfo {
	codeFileLabel := strings.Join(jb.codeFiles, ", ")
	failure := failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCode,
		outLog:   jb.outLog,
		errLog:   jb.errLog,
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

func terminateLegacyProcess(cmd *exec.Cmd, cmdDone <-chan error, reason string) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		slog.Warn("send SIGTERM to legacy process failed", "reason", reason, "error", err)
		return
	}

	select {
	case <-cmdDone:
	case <-time.After(processTerminationGracePeriod):
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			slog.Warn("kill legacy process failed", "reason", reason, "error", err)
		}
	}
}

func maxAtLeastOne(value int) int {
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
			if !prompt.IsReviewResolved(currentContent) || prompt.IsReviewResolved(entry.Content) {
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

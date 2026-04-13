package executor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const runtimeEventBusBufferSize = 64

// Execute runs the prepared jobs and manages shutdown, retries, and summaries.
func Execute(
	ctx context.Context,
	jobs []model.Job,
	runArtifacts model.RunArtifacts,
	runJournal *journal.Journal,
	bus *events.Bus[events.Event],
	cfg *model.RuntimeConfig,
	manager model.RuntimeManager,
) (retErr error) {
	internalCfg, err := prepareExecutionConfig(ctx, cfg, runArtifacts, manager)
	if err != nil {
		return err
	}
	internalJobs := newJobs(jobs)
	var streamer *workflowEventStreamer
	defer func() {
		if streamer != nil {
			if err := streamer.FinalizeAndStop(); err != nil {
				retErr = errors.Join(retErr, err)
			}
		}
	}()
	bus = ensureRuntimeEventBus(internalCfg, runJournal, bus)
	streamer = startWorkflowEventStreamer(bus, internalCfg, os.Stdout)
	startedAt := time.Now().UTC()
	defer func() {
		if err := closeRunJournal(runJournal); err != nil {
			retErr = errors.Join(retErr, err)
		}
	}()

	if err := emitRunStart(ctx, runJournal, runArtifacts, internalCfg, internalJobs); err != nil {
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
	if err := finalizeExecution(
		ctx,
		runJournal,
		runArtifacts,
		internalCfg,
		internalJobs,
		result,
		failed,
		failures,
		total,
		startedAt,
	); err != nil {
		return err
	}

	if shutdownErr != nil {
		if internalCfg.HumanOutputEnabled() {
			fmt.Fprintf(os.Stderr, "\nShutdown interrupted: %v\n", shutdownErr)
		}
		return shutdownErr
	}
	if len(failures) > 0 {
		return errors.New("one or more groups failed; see logs above")
	}
	return nil
}

func prepareExecutionConfig(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	runArtifacts model.RunArtifacts,
	manager model.RuntimeManager,
) (*config, error) {
	internalCfg := newConfig(cfg, runArtifacts)
	internalCfg.RuntimeManager = manager

	preStart, err := model.DispatchMutableHook(
		ctx,
		internalCfg.RuntimeManager,
		"run.pre_start",
		runPreStartPayload{
			RunID:     runArtifacts.RunID,
			Config:    hookRuntimeConfig(internalCfg),
			Artifacts: runArtifacts,
		},
	)
	if err != nil {
		return nil, err
	}
	applyHookRuntimeConfig(internalCfg, preStart.Config)
	return internalCfg, nil
}

func emitRunStart(
	ctx context.Context,
	runJournal *journal.Journal,
	runArtifacts model.RunArtifacts,
	internalCfg *config,
	internalJobs []job,
) error {
	if err := submitRunEvent(
		ctx,
		runJournal,
		runArtifacts.RunID,
		events.EventKindRunStarted,
		kinds.RunStartedPayload{
			Mode:            string(internalCfg.Mode),
			Name:            internalCfg.Name,
			WorkspaceRoot:   internalCfg.WorkspaceRoot,
			IDE:             internalCfg.IDE,
			Model:           internalCfg.Model,
			ReasoningEffort: internalCfg.ReasoningEffort,
			AccessMode:      internalCfg.AccessMode,
			ArtifactsDir:    runArtifacts.RunDir,
			JobsTotal:       len(internalJobs),
		},
	); err != nil {
		return err
	}

	model.DispatchObserverHook(
		ctx,
		internalCfg.RuntimeManager,
		"run.post_start",
		runPostStartPayload{
			RunID:  runArtifacts.RunID,
			Config: hookRuntimeConfig(internalCfg),
		},
	)
	return nil
}

func finalizeExecution(
	ctx context.Context,
	runJournal *journal.Journal,
	runArtifacts model.RunArtifacts,
	internalCfg *config,
	internalJobs []job,
	result executionResult,
	failed int32,
	failures []failInfo,
	total int,
	startedAt time.Time,
) error {
	reason := hookShutdownReason(result)
	model.DispatchObserverHook(
		ctx,
		internalCfg.RuntimeManager,
		"run.pre_shutdown",
		runPreShutdownPayload{
			RunID:  runArtifacts.RunID,
			Reason: reason,
		},
	)
	if err := emitExecutionResult(internalCfg, result); err != nil {
		return err
	}
	if internalCfg.HumanOutputEnabled() {
		summarizeResults(failed, failures, total)
	}
	refreshTaskMetaOnExit(internalCfg)
	if err := emitRunTerminalEvent(ctx, runJournal, result, internalJobs, startedAt); err != nil {
		return err
	}
	model.DispatchObserverHook(
		ctx,
		internalCfg.RuntimeManager,
		"run.post_shutdown",
		runPostShutdownPayload{
			RunID:   runArtifacts.RunID,
			Reason:  reason,
			Summary: hookRunSummary(result),
		},
	)
	return nil
}

func ensureRuntimeEventBus(
	cfg *config,
	runJournal *journal.Journal,
	bus *events.Bus[events.Event],
) *events.Bus[events.Event] {
	if cfg != nil && (cfg.UIEnabled() || cfg.EventStreamEnabled()) && bus == nil {
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
		logger:        runtimeLoggerFor(cfg, cfg.UIEnabled()),
		journal:       runJournal,
		bus:           bus,
		sem:           make(chan struct{}, atLeastOne(cfg.Concurrent)),
		activeClients: make(map[agent.Client]struct{}),
	}
	for idx := range execCtx.jobs {
		execCtx.jobs[idx].OutBuffer = newLineBuffer(cfg.TailLines)
		execCtx.jobs[idx].ErrBuffer = newLineBuffer(cfg.TailLines)
	}
	execCtx.ui = setupUI(ctx, execCtx.jobs, cfg, bus, cfg.UIEnabled())
	return execCtx, nil
}

func (j *jobExecutionContext) cleanup() {
	if err := j.shutdownUI(); err != nil {
		if j != nil && j.cfg.HumanOutputEnabled() {
			fmt.Fprintf(os.Stderr, "UI shutdown error: %v\n", err)
		}
	}
}

func (j *jobExecutionContext) runtimeLogger() *slog.Logger {
	if j != nil && j.logger != nil {
		return j.logger
	}
	if j != nil {
		return runtimeLoggerFor(j.cfg, j.cfg != nil && j.cfg.UIEnabled())
	}
	return runtimeLogger(false)
}

func (j *jobExecutionContext) awaitUIAfterCompletion() error {
	if j.ui == nil {
		return nil
	}
	// Normal completion must leave the event adapter running until the operator
	// exits the completed cockpit. Closing it early can drop the final
	// session/job completion events and leave the UI visually stuck in RUNNING.
	return j.ui.Wait()
}

func (j *jobExecutionContext) shutdownUI() error {
	if j.ui == nil {
		return nil
	}
	j.ui.CloseEvents()
	j.ui.Shutdown()
	return j.ui.Wait()
}

func (j *jobExecutionContext) publishShutdownStatus(state shutdownState) {
	if state.Phase != shutdownPhaseDraining {
		return
	}
	j.submitEventOrWarn(
		events.EventKindShutdownDraining,
		kinds.ShutdownDrainingPayload{
			ShutdownBase: kinds.ShutdownBase{
				Source:      string(state.Source),
				RequestedAt: state.RequestedAt,
				DeadlineAt:  state.DeadlineAt,
			},
		},
	)
}

func (j *jobExecutionContext) launchWorkers(jobCtx context.Context) {
	if j.cfg.Mode == model.ExecutionModePRDTasks {
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
	if j == nil || !j.cfg.HumanOutputEnabled() {
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
	event, err := newRuntimeEvent(j.cfg.RunArtifacts.RunID, kind, payload)
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
			ShutdownBase: kinds.ShutdownBase{
				Source:      string(state.Source),
				RequestedAt: state.RequestedAt,
				DeadlineAt:  state.DeadlineAt,
			},
		},
	)
}

func (j *jobExecutionContext) emitShutdownTerminated(state shutdownState, forced bool) {
	if !state.Active() {
		return
	}
	j.submitEventOrWarn(
		events.EventKindShutdownTerminated,
		kinds.ShutdownTerminatedPayload{
			ShutdownBase: kinds.ShutdownBase{
				Source:      string(state.Source),
				RequestedAt: state.RequestedAt,
				DeadlineAt:  state.DeadlineAt,
			},
			Forced: forced,
		},
	)
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
			f.CodeFile,
			f.ExitCode,
			f.OutLog,
			f.ErrLog,
		)
	}
}

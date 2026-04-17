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
	"github.com/compozy/compozy/internal/core/sound"
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

	normalCompletionFinalized := false
	failed, failures, total, shutdownErr := executeJobsWithGracefulShutdown(
		ctx,
		internalJobs,
		internalCfg,
		runJournal,
		bus,
		buildNormalCompletionHook(
			ctx,
			runJournal,
			runArtifacts,
			internalCfg,
			internalJobs,
			startedAt,
			&normalCompletionFinalized,
		),
	)
	if !normalCompletionFinalized {
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

func buildNormalCompletionHook(
	ctx context.Context,
	runJournal *journal.Journal,
	runArtifacts model.RunArtifacts,
	internalCfg *config,
	internalJobs []job,
	startedAt time.Time,
	finalized *bool,
) normalCompletionHook {
	return func(failed int32, failures []failInfo, total int) error {
		result := buildExecutionResult(internalCfg, internalJobs, failures, nil)
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
		if finalized != nil {
			*finalized = true
		}
		return nil
	}
}

func prepareExecutionConfig(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	runArtifacts model.RunArtifacts,
	manager model.RuntimeManager,
) (*config, error) {
	preparedConfig := snapshotWorkflowPreparedStateConfig(cfg)
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
	if err := validateWorkflowPreparedStateMutation(preparedConfig, &preStart.Config); err != nil {
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
	if err := model.WaitForObserverHooks(ctx, internalCfg.RuntimeManager); err != nil {
		return fmt.Errorf("wait for pending observer hooks: %w", err)
	}
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
	notifySoundForKind(
		ctx,
		internalCfg,
		terminalEventKindFor(result.Status),
		runtimeLoggerFor(internalCfg, internalCfg.UIEnabled()),
	)
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

type workflowPreparedStateConfig struct {
	workspaceRoot    string
	name             string
	tasksDir         string
	mode             model.ExecutionMode
	includeCompleted bool
	provider         string
	pr               string
	reviewsDir       string
	round            int
	autoCommit       bool
	agentName        string
	ide              string
	model            string
	reasoningEffort  string
	accessMode       string
	addDirs          []string
	taskRuntimeRules []model.TaskRuntimeRule
}

func snapshotWorkflowPreparedStateConfig(cfg *model.RuntimeConfig) workflowPreparedStateConfig {
	if cfg == nil {
		return workflowPreparedStateConfig{}
	}
	return workflowPreparedStateConfig{
		workspaceRoot:    cfg.WorkspaceRoot,
		name:             cfg.Name,
		tasksDir:         cfg.TasksDir,
		mode:             cfg.Mode,
		includeCompleted: cfg.IncludeCompleted,
		provider:         cfg.Provider,
		pr:               cfg.PR,
		reviewsDir:       cfg.ReviewsDir,
		round:            cfg.Round,
		autoCommit:       cfg.AutoCommit,
		agentName:        cfg.AgentName,
		ide:              cfg.IDE,
		model:            cfg.Model,
		reasoningEffort:  cfg.ReasoningEffort,
		accessMode:       cfg.AccessMode,
		addDirs:          append([]string(nil), cfg.AddDirs...),
		taskRuntimeRules: model.CloneTaskRuntimeRules(cfg.TaskRuntimeRules),
	}
}

func validateWorkflowPreparedStateMutation(
	before workflowPreparedStateConfig,
	cfg *model.RuntimeConfig,
) error {
	current := snapshotWorkflowPreparedStateConfig(cfg)
	for _, check := range []struct {
		field   string
		changed bool
	}{
		{field: "workspace_root", changed: current.workspaceRoot != before.workspaceRoot},
		{field: "name", changed: current.name != before.name},
		{field: "tasks_dir", changed: current.tasksDir != before.tasksDir},
		{field: "mode", changed: current.mode != before.mode},
		{field: "include_completed", changed: current.includeCompleted != before.includeCompleted},
		{field: "provider", changed: current.provider != before.provider},
		{field: "pr", changed: current.pr != before.pr},
		{field: "reviews_dir", changed: current.reviewsDir != before.reviewsDir},
		{field: "round", changed: current.round != before.round},
		{field: "auto_commit", changed: current.autoCommit != before.autoCommit},
		{field: "agent_name", changed: current.agentName != before.agentName},
		{field: "ide", changed: current.ide != before.ide},
		{field: "model", changed: current.model != before.model},
		{field: "reasoning_effort", changed: current.reasoningEffort != before.reasoningEffort},
		{field: "access_mode", changed: current.accessMode != before.accessMode},
		{field: "add_dirs", changed: !equalStringSlices(current.addDirs, before.addDirs)},
		{field: "task_runtime_rules", changed: !equalTaskRuntimeRules(current.taskRuntimeRules, before.taskRuntimeRules)},
	} {
		if check.changed {
			return fmt.Errorf(
				"run.pre_start cannot mutate %s after workflow state preparation",
				check.field,
			)
		}
	}
	return nil
}

func equalStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func equalTaskRuntimeRules(left []model.TaskRuntimeRule, right []model.TaskRuntimeRule) bool {
	if len(left) != len(right) {
		return false
	}
	for idx := range left {
		if !equalOptionalString(left[idx].ID, right[idx].ID) ||
			!equalOptionalString(left[idx].Type, right[idx].Type) ||
			!equalOptionalString(left[idx].IDE, right[idx].IDE) ||
			!equalOptionalString(left[idx].Model, right[idx].Model) ||
			!equalOptionalString(left[idx].ReasoningEffort, right[idx].ReasoningEffort) {
			return false
		}
	}
	return true
}

func equalOptionalString(left *string, right *string) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
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

// notifySoundForKind plays the configured sound for a terminal lifecycle
// event kind. It runs synchronously so the audio finishes before run
// cleanup tears state down. When the [sound] feature flag is off this
// is a no-op.
func notifySoundForKind(ctx context.Context, cfg *config, kind events.EventKind, logger *slog.Logger) {
	if cfg == nil || !cfg.SoundEnabled {
		return
	}
	sound.Notify(ctx, sound.Config{
		Player:      sound.New(),
		OnCompleted: cfg.SoundOnCompleted,
		OnFailed:    cfg.SoundOnFailed,
	}, kind, logger)
}

// terminalEventKindFor maps an executor result status to the lifecycle event
// kind that finalizeExecution emits. Mirrors the switch in emitRunTerminalEvent.
func terminalEventKindFor(status string) events.EventKind {
	switch status {
	case runStatusSucceeded:
		return events.EventKindRunCompleted
	case runStatusCanceled:
		return events.EventKindRunCancelled
	default:
		return events.EventKindRunFailed
	}
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
	if j.requiresOrderedWorkerExecution() {
		j.launchOrderedWorkers(jobCtx)
		return
	}
	for idx := range j.jobs {
		jb := &j.jobs[idx]
		j.wg.Add(1)
		go j.executeJob(jobCtx, idx, jb)
	}
}

func (j *jobExecutionContext) requiresOrderedWorkerExecution() bool {
	if j == nil || j.cfg == nil {
		return false
	}
	if j.cfg.Mode == model.ExecutionModePRDTasks {
		return true
	}
	return atLeastOne(j.cfg.Concurrent) == 1
}

func (j *jobExecutionContext) launchOrderedWorkers(jobCtx context.Context) {
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

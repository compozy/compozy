package parallelrun

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/looplab/fsm"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/recovery"
	"github.com/compozy/compozy/internal/core/workspace"
)

// WorktreeBase identifies the branch/ref a task worktree should branch from.
type WorktreeBase struct {
	Branch string
	Commit string
}

// TaskSpec describes one PRD task scheduled by the orchestrator.
type TaskSpec struct {
	ID     TaskID
	Number int
	Title  string
	Slug   string
}

// TaskLaunchSpec describes one isolated task run launched from a wave base.
type TaskLaunchSpec struct {
	RunID     string
	WaveIndex int
	Task      TaskSpec
	Base      WorktreeBase
}

// TaskRunResult is the daemon-neutral metadata for one child task run.
type TaskRunResult struct {
	Task         TaskSpec
	RunID        string
	WorktreePath string
	BaseBranch   string
	BaseCommit   string
}

// PreparedTaskRun is the daemon-neutral task execution seam. It executes one
// task-scoped run and can restart the same task in the same workspace for
// recovery.
type PreparedTaskRun interface {
	recovery.PreparedRun
	Result() TaskRunResult
	FailedConfig() *model.RuntimeConfig
}

// TaskLauncher prepares one task-scoped worktree child run.
type TaskLauncher interface {
	PrepareTask(ctx context.Context, spec TaskLaunchSpec) (PreparedTaskRun, error)
}

// IntegrationSpec describes the dedicated integration branch worktree.
type IntegrationSpec struct {
	WorkspaceRoot     string
	IntegrationPath   string
	IntegrationBranch string
	BaseRef           string
}

// WorktreeLifecycle is the orchestrator's git/write-back boundary.
type WorktreeLifecycle interface {
	CreateIntegrationBranch(ctx context.Context, spec IntegrationSpec) error
	Commit(ctx context.Context, path string, message string) (string, error)
	SquashMerge(ctx context.Context, integrationPath string, worktreeRef string, message string) (ConflictSet, error)
	Head(ctx context.Context, path string) (string, error)
	FastForward(ctx context.Context, workspaceRoot string, targetBranch string, integrationBranch string) error
	DiscardIntegrationBranch(
		ctx context.Context,
		workspaceRoot string,
		integrationPath string,
		integrationBranch string,
	) error
	Remove(ctx context.Context, workspaceRoot string, path string) error
	Prune(ctx context.Context, workspaceRoot string) error
}

// ConflictSet describes a squash merge result.
type ConflictSet struct {
	Files []string
	Clean bool
}

// Plan contains the complete happy-path execution input.
type Plan struct {
	RunID             string
	WorkspaceRoot     string
	BaseBranch        string
	BaseCommit        string
	IntegrationBranch string
	IntegrationPath   string
	Waves             Waves
	Tasks             []TaskSpec
	Config            workspace.ParallelTasksConfig
	Recovery          workspace.AgentRecoveryConfig
}

// ParallelPlan preserves the task-specified public API name.
type ParallelPlan = Plan

// OutcomeStatus is the terminal status returned by the orchestrator.
type OutcomeStatus string

const (
	ParallelOutcomeCompleted  OutcomeStatus = "completed"
	ParallelOutcomeCanceled   OutcomeStatus = "canceled"
	ParallelOutcomeRolledBack OutcomeStatus = "rolled_back"
)

// ParallelOutcomeStatus preserves the task-specified public API name.
type ParallelOutcomeStatus = OutcomeStatus

// TaskOutcome records the integrated result for one task.
type TaskOutcome struct {
	Task           TaskSpec
	WaveIndex      int
	RunID          string
	WorktreePath   string
	WorktreeCommit string
	Status         TaskOutcomeStatus
	BlockedBy      TaskID
	Error          string
}

// TaskOutcomeStatus is the per-task status reported by the parallel run.
type TaskOutcomeStatus string

const (
	TaskOutcomeMerged    TaskOutcomeStatus = "merged"
	TaskOutcomeRecovered TaskOutcomeStatus = "recovered"
	TaskOutcomeFailed    TaskOutcomeStatus = "failed"
	TaskOutcomeSkipped   TaskOutcomeStatus = "skipped"
)

// StatusReport returns the user-facing per-task status string.
func (o TaskOutcome) StatusReport() string {
	if o.Status == TaskOutcomeSkipped && o.BlockedBy != "" {
		return fmt.Sprintf("%s (blocked by %s)", o.Status, o.BlockedBy)
	}
	return string(o.Status)
}

// WaveOutcome records all task outcomes for one wave.
type WaveOutcome struct {
	Index int
	Tasks []TaskOutcome
}

// Outcome is the typed result of a parallel run.
type Outcome struct {
	Status            OutcomeStatus
	IntegrationBranch string
	IntegrationPath   string
	Waves             []WaveOutcome
	Tasks             []TaskOutcome
}

// ParallelOutcome preserves the task-specified public API name.
type ParallelOutcome = Outcome

// ExecutionOrchestrator drives the parallel task FSM happy path.
type ExecutionOrchestrator struct {
	worktrees        WorktreeLifecycle
	launcher         TaskLauncher
	recoveryStrategy recovery.RemediationStrategy
	conflictResolver ConflictResolver
	emitter          ParallelEventEmitter
	log              *slog.Logger
}

// ParallelExecutionOrchestrator preserves the task-specified public API name.
type ParallelExecutionOrchestrator = ExecutionOrchestrator

// ExecutionOrchestratorOption configures ExecutionOrchestrator.
type ExecutionOrchestratorOption func(*ExecutionOrchestrator)

// ParallelExecutionOrchestratorOption preserves the task-specified public API name.
type ParallelExecutionOrchestratorOption = ExecutionOrchestratorOption

// WithLogger supplies the logger used for FSM transition logs.
func WithLogger(log *slog.Logger) ExecutionOrchestratorOption {
	return func(o *ExecutionOrchestrator) {
		if log != nil {
			o.log = log
		}
	}
}

// WithRecoveryStrategy supplies the remediation strategy used for failed tasks.
func WithRecoveryStrategy(strategy recovery.RemediationStrategy) ExecutionOrchestratorOption {
	return func(o *ExecutionOrchestrator) {
		o.recoveryStrategy = strategy
	}
}

// WithConflictResolver supplies the agentic merge-conflict resolver.
func WithConflictResolver(resolver ConflictResolver) ExecutionOrchestratorOption {
	return func(o *ExecutionOrchestrator) {
		if resolver != nil {
			o.conflictResolver = resolver
		}
	}
}

// WithEventEmitter supplies the sink for task.parallel.* lifecycle events. When
// unset the orchestrator emits to a no-op sink.
func WithEventEmitter(emitter ParallelEventEmitter) ExecutionOrchestratorOption {
	return func(o *ExecutionOrchestrator) {
		if emitter != nil {
			o.emitter = emitter
		}
	}
}

// NewParallelExecutionOrchestrator constructs an FSM-backed parallel orchestrator.
func NewParallelExecutionOrchestrator(
	worktrees WorktreeLifecycle,
	launcher TaskLauncher,
	opts ...ExecutionOrchestratorOption,
) *ExecutionOrchestrator {
	o := &ExecutionOrchestrator{
		worktrees:        worktrees,
		launcher:         launcher,
		conflictResolver: NewAgenticConflictResolution(),
		emitter:          noopEventEmitter{},
		log:              slog.Default(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

// Run executes every wave, merges successful task worktrees, and fast-forwards
// the target branch on completion.
func (o *ExecutionOrchestrator) Run(ctx context.Context, plan ParallelPlan) (ParallelOutcome, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if o == nil {
		return ParallelOutcome{}, errors.New("parallel execution: missing orchestrator")
	}
	if err := o.validatePlan(plan); err != nil {
		return ParallelOutcome{}, err
	}

	machine := newParallelFSM()
	outcome := Outcome{
		IntegrationBranch: strings.TrimSpace(plan.IntegrationBranch),
		IntegrationPath:   strings.TrimSpace(plan.IntegrationPath),
	}
	if err := o.createIntegrationBranch(ctx, plan); err != nil {
		return outcome, fmt.Errorf("create parallel integration branch: %w", err)
	}
	levels := plan.Waves.Levels()
	if err := o.runWaves(ctx, machine, plan, levels, &outcome); err != nil {
		return outcome, err
	}
	if err := o.finalize(ctx, machine, plan, levels, &outcome); err != nil {
		return outcome, err
	}
	return outcome, nil
}

func (o *ExecutionOrchestrator) createIntegrationBranch(ctx context.Context, plan ParallelPlan) error {
	return o.worktrees.CreateIntegrationBranch(ctx, IntegrationSpec{
		WorkspaceRoot:     strings.TrimSpace(plan.WorkspaceRoot),
		IntegrationPath:   strings.TrimSpace(plan.IntegrationPath),
		IntegrationBranch: strings.TrimSpace(plan.IntegrationBranch),
		BaseRef:           strings.TrimSpace(plan.BaseCommit),
	})
}

func (o *ExecutionOrchestrator) runWaves(
	ctx context.Context,
	machine *fsm.FSM,
	plan ParallelPlan,
	levels [][]TaskID,
	outcome *ParallelOutcome,
) error {
	currentBase := WorktreeBase{
		Branch: strings.TrimSpace(plan.IntegrationBranch),
		Commit: strings.TrimSpace(plan.BaseCommit),
	}
	tasksByID := taskSpecsByID(plan.Tasks)
	failed := map[TaskID]bool{}
	blockedBy := map[TaskID]TaskID{}
	waveTotal := len(levels)
	for waveIndex, level := range levels {
		if err := o.runOneWave(
			ctx,
			machine,
			plan,
			waveIndex,
			waveTotal,
			level,
			currentBase,
			tasksByID,
			failed,
			&blockedBy,
			outcome,
		); err != nil {
			return err
		}
		head, err := o.worktrees.Head(ctx, plan.IntegrationPath)
		if err != nil {
			return fmt.Errorf("resolve integration head after wave %d: %w", waveIndex+1, err)
		}
		currentBase.Commit = head
	}
	return nil
}

func (o *ExecutionOrchestrator) runOneWave(
	ctx context.Context,
	machine *fsm.FSM,
	plan ParallelPlan,
	waveIndex int,
	waveTotal int,
	level []TaskID,
	currentBase WorktreeBase,
	tasksByID map[TaskID]TaskSpec,
	failed map[TaskID]bool,
	blockedBy *map[TaskID]TaskID,
	outcome *ParallelOutcome,
) error {
	if err := o.checkCanceled(ctx, machine, outcome); err != nil {
		return err
	}
	if err := o.transition(ctx, machine, parallelEventStartWave, waveIndex); err != nil {
		return err
	}
	runnable, skipped := splitRunnableLevel(waveIndex, level, tasksByID, *blockedBy)
	o.emitWaveTasksStarted(ctx, plan, waveIndex, waveTotal, runnable, tasksByID)
	waveOutcome := WaveOutcome{Index: waveIndex, Tasks: skipped}
	executions, err := o.runWave(ctx, plan, waveIndex, runnable, currentBase, tasksByID)
	if err != nil {
		return o.wrapCancelError(ctx, machine, outcome, err)
	}
	if hasFailedExecutions(executions) {
		if err := o.transition(ctx, machine, parallelEventRecoverWave, waveIndex); err != nil {
			return err
		}
		o.recoverFailedExecutions(ctx, plan, executions)
	}
	newFailures := appendFailedTaskOutcomes(&waveOutcome, executions)
	if len(newFailures) > 0 {
		for _, taskID := range newFailures {
			failed[taskID] = true
		}
		updated := plan.Waves.BlockedBy(failed)
		if updated == nil {
			updated = map[TaskID]TaskID{}
		}
		*blockedBy = updated
	}
	if err := o.transition(ctx, machine, parallelEventMergeWave, waveIndex); err != nil {
		return err
	}
	o.emitMergeStarted(ctx, plan, waveIndex, waveTotal)
	mergeOutcome, err := o.mergeWave(ctx, machine, plan, waveIndex, mergeableExecutions(executions), outcome)
	if err != nil {
		return o.wrapCancelError(ctx, machine, outcome, err)
	}
	waveOutcome.Tasks = append(waveOutcome.Tasks, mergeOutcome.Tasks...)
	sortTaskOutcomes(waveOutcome.Tasks)
	outcome.Waves = append(outcome.Waves, waveOutcome)
	outcome.Tasks = append(outcome.Tasks, waveOutcome.Tasks...)
	if err := o.transition(ctx, machine, parallelEventFinishWave, waveIndex); err != nil {
		return err
	}
	o.emitWaveCompleted(ctx, plan, waveIndex, waveTotal)
	return nil
}

// emitWaveTasksStarted announces every runnable task entering a wave so the TUI
// can group sidebar cards by wave before the per-task child runs start streaming.
func (o *ExecutionOrchestrator) emitWaveTasksStarted(
	ctx context.Context,
	plan ParallelPlan,
	waveIndex, waveTotal int,
	runnable []TaskID,
	tasksByID map[TaskID]TaskSpec,
) {
	for _, taskID := range runnable {
		task, ok := tasksByID[taskID]
		if !ok {
			task = TaskSpec{ID: taskID}
		}
		o.emitWaveStarted(ctx, plan, waveIndex, waveTotal, task)
	}
}

func (o *ExecutionOrchestrator) finalize(
	ctx context.Context,
	machine *fsm.FSM,
	plan ParallelPlan,
	levels [][]TaskID,
	outcome *ParallelOutcome,
) error {
	if err := o.checkCanceled(ctx, machine, outcome); err != nil {
		return err
	}
	if err := o.transition(ctx, machine, parallelEventFinalize, len(levels)); err != nil {
		return err
	}
	if err := o.worktrees.FastForward(ctx, plan.WorkspaceRoot, plan.BaseBranch, plan.IntegrationBranch); err != nil {
		return fmt.Errorf("fast-forward %s to %s: %w", plan.BaseBranch, plan.IntegrationBranch, err)
	}
	if err := o.cleanupCompleted(ctx, plan, outcome.Tasks); err != nil {
		return err
	}
	if err := o.transition(ctx, machine, parallelEventComplete, len(levels)); err != nil {
		return err
	}
	outcome.Status = ParallelOutcomeCompleted
	return nil
}

func (o *ExecutionOrchestrator) validatePlan(plan ParallelPlan) error {
	if o.worktrees == nil {
		return errors.New("parallel execution: missing worktree lifecycle")
	}
	if o.launcher == nil {
		return errors.New("parallel execution: missing task launcher")
	}
	if o.conflictResolver == nil {
		return errors.New("parallel execution: missing conflict resolver")
	}
	required := []struct {
		name  string
		value string
	}{
		{name: "run id", value: plan.RunID},
		{name: "workspace root", value: plan.WorkspaceRoot},
		{name: "base branch", value: plan.BaseBranch},
		{name: "base commit", value: plan.BaseCommit},
		{name: "integration branch", value: plan.IntegrationBranch},
		{name: "integration path", value: plan.IntegrationPath},
	}
	for _, item := range required {
		if strings.TrimSpace(item.value) == "" {
			return fmt.Errorf("parallel execution: %s is required", item.name)
		}
	}
	tasksByID := taskSpecsByID(plan.Tasks)
	for _, level := range plan.Waves.Levels() {
		for _, id := range level {
			task, ok := tasksByID[id]
			if !ok {
				return fmt.Errorf("parallel execution: missing task spec for %q", id)
			}
			if task.Number <= 0 {
				return fmt.Errorf("parallel execution: task %q has invalid number %d", id, task.Number)
			}
			if strings.TrimSpace(task.Slug) == "" {
				return fmt.Errorf("parallel execution: task %q has empty slug", id)
			}
		}
	}
	return nil
}

func (o *ExecutionOrchestrator) runWave(
	ctx context.Context,
	plan ParallelPlan,
	waveIndex int,
	level []TaskID,
	base WorktreeBase,
	tasksByID map[TaskID]TaskSpec,
) ([]taskExecution, error) {
	orderedTasks := taskSpecsForLevel(level, tasksByID)
	limit := maxConcurrency(plan.Config)
	sem := make(chan struct{}, limit)
	results := make([]taskExecution, len(orderedTasks))
	errs := make([]error, len(orderedTasks))
	var wg sync.WaitGroup
	launched := 0

	for idx := range orderedTasks {
		if err := ctx.Err(); err != nil {
			break
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break
		}
		if err := ctx.Err(); err != nil {
			break
		}
		index := idx
		task := orderedTasks[idx]
		wg.Add(1)
		launched++
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			prepared, err := o.launcher.PrepareTask(ctx, TaskLaunchSpec{
				RunID:     strings.TrimSpace(plan.RunID),
				WaveIndex: waveIndex,
				Task:      task,
				Base:      base,
			})
			if err != nil {
				errs[index] = fmt.Errorf("prepare task %s: %w", task.ID, err)
				return
			}
			outcome, executeErr := prepared.Execute(ctx)
			result := prepared.Result()
			if result.Task.ID == "" {
				result.Task = task
			}
			if result.RunID == "" {
				result.RunID = strings.TrimSpace(outcome.RunID)
			}
			results[index] = taskExecution{
				prepared: prepared,
				result:   result,
				outcome:  outcome,
				err:      executeErr,
			}
			if fatalErr := taskExecutionFatalError(task.ID, outcome, executeErr); fatalErr != nil {
				errs[index] = fatalErr
			}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return results[:launched], err
	}
	if err := errors.Join(errs...); err != nil {
		return results[:launched], err
	}
	return results[:launched], nil
}

func (o *ExecutionOrchestrator) mergeWave(
	ctx context.Context,
	machine *fsm.FSM,
	plan ParallelPlan,
	waveIndex int,
	executions []taskExecution,
	runOutcome *ParallelOutcome,
) (WaveOutcome, error) {
	orderedRuns := append([]taskExecution(nil), executions...)
	sort.SliceStable(orderedRuns, func(i, j int) bool {
		return orderedRuns[i].result.Task.Number < orderedRuns[j].result.Task.Number
	})
	outcome := WaveOutcome{Index: waveIndex}
	for index := range orderedRuns {
		if err := ctx.Err(); err != nil {
			return outcome, err
		}
		execution := &orderedRuns[index]
		run := execution.result
		commit, err := o.worktrees.Commit(ctx, run.WorktreePath, commitMessage(run.Task))
		if err != nil {
			return outcome, fmt.Errorf("commit task %s worktree: %w", run.Task.ID, err)
		}
		conflicts, err := o.worktrees.SquashMerge(ctx, plan.IntegrationPath, commit, commitMessage(run.Task))
		if err != nil {
			return outcome, fmt.Errorf("squash merge task %s: %w", run.Task.ID, err)
		}
		if !conflicts.Clean {
			o.emitConflictDetected(ctx, plan, waveIndex, run.Task, conflicts, 1, resolverMaxAttempts(plan))
			result, err := o.resolveConflict(ctx, machine, plan, waveIndex, run.Task, conflicts)
			if err != nil {
				return outcome, o.rollback(ctx, machine, plan, waveIndex, runOutcome, err)
			}
			if !result.Resolved || !result.Builds {
				err := fmt.Errorf(
					"squash merge task %s conflict resolver exhausted after %d attempt(s): resolved=%t builds=%t files=%s",
					run.Task.ID,
					result.Attempts,
					result.Resolved,
					result.Builds,
					strings.Join(conflicts.Files, ", "),
				)
				return outcome, o.rollback(ctx, machine, plan, waveIndex, runOutcome, err)
			}
			if _, err := o.worktrees.Commit(ctx, plan.IntegrationPath, commitMessage(run.Task)); err != nil {
				return outcome, o.rollback(
					ctx,
					machine,
					plan,
					waveIndex,
					runOutcome,
					fmt.Errorf("commit resolved squash merge for task %s: %w", run.Task.ID, err),
				)
			}
		}
		taskOutcome := TaskOutcome{
			Task:           run.Task,
			WaveIndex:      waveIndex,
			RunID:          strings.TrimSpace(run.RunID),
			WorktreePath:   strings.TrimSpace(run.WorktreePath),
			WorktreeCommit: strings.TrimSpace(commit),
			Status:         mergedStatus(*execution),
		}
		outcome.Tasks = append(outcome.Tasks, taskOutcome)
		o.emitTaskOutcome(ctx, plan, taskOutcome)
	}
	return outcome, nil
}

func (o *ExecutionOrchestrator) resolveConflict(
	ctx context.Context,
	machine *fsm.FSM,
	plan ParallelPlan,
	waveIndex int,
	task TaskSpec,
	conflicts ConflictSet,
) (ConflictResult, error) {
	if err := o.transition(ctx, machine, parallelEventResolve, waveIndex); err != nil {
		return ConflictResult{}, err
	}
	o.emitConflictResolving(ctx, plan, waveIndex, task, conflicts, 1, resolverMaxAttempts(plan))
	message := commitMessage(task)
	result, err := o.conflictResolver.Resolve(ctx, conflictResolverInput(plan, task, conflicts, message))
	if err != nil {
		return result, err
	}
	hasMarkers, markerErr := conflictMarkersPresent(plan.IntegrationPath, conflicts.Files)
	if markerErr != nil {
		return result, markerErr
	}
	if hasMarkers {
		result.Resolved = false
		return result, nil
	}
	if result.Resolved && result.Builds {
		if err := o.transition(ctx, machine, parallelEventResolved, waveIndex); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (o *ExecutionOrchestrator) rollback(
	ctx context.Context,
	machine *fsm.FSM,
	plan ParallelPlan,
	waveIndex int,
	outcome *ParallelOutcome,
	cause error,
) error {
	rollbackCtx := context.WithoutCancel(ctx)
	if outcome != nil {
		outcome.Status = ParallelOutcomeRolledBack
	}
	discardErr := o.worktrees.DiscardIntegrationBranch(
		rollbackCtx,
		plan.WorkspaceRoot,
		plan.IntegrationPath,
		plan.IntegrationBranch,
	)
	pruneErr := o.worktrees.Prune(rollbackCtx, plan.WorkspaceRoot)
	transitionErr := o.transition(rollbackCtx, machine, parallelEventRollback, -1)
	o.emitRolledBack(rollbackCtx, plan, waveIndex)
	return errors.Join(cause, discardErr, pruneErr, transitionErr)
}

func (o *ExecutionOrchestrator) cleanupCompleted(
	ctx context.Context,
	plan ParallelPlan,
	tasks []TaskOutcome,
) error {
	for index := range tasks {
		task := &tasks[index]
		if !task.Status.removesWorktree() || strings.TrimSpace(task.WorktreePath) == "" {
			continue
		}
		if err := o.worktrees.Remove(ctx, plan.WorkspaceRoot, task.WorktreePath); err != nil {
			return fmt.Errorf("remove task %s worktree: %w", task.Task.ID, err)
		}
	}
	if err := o.worktrees.DiscardIntegrationBranch(
		ctx,
		plan.WorkspaceRoot,
		plan.IntegrationPath,
		plan.IntegrationBranch,
	); err != nil {
		return fmt.Errorf("discard integration branch %s: %w", plan.IntegrationBranch, err)
	}
	if err := o.worktrees.Prune(ctx, plan.WorkspaceRoot); err != nil {
		return fmt.Errorf("prune worktrees for %s: %w", plan.WorkspaceRoot, err)
	}
	return nil
}

func (o *ExecutionOrchestrator) checkCanceled(
	ctx context.Context,
	machine *fsm.FSM,
	outcome *ParallelOutcome,
) error {
	if err := ctx.Err(); err != nil {
		if outcome != nil {
			outcome.Status = ParallelOutcomeCanceled
		}
		if transitionErr := o.transition(
			context.WithoutCancel(ctx),
			machine,
			parallelEventCancel,
			-1,
		); transitionErr != nil {
			return errors.Join(err, transitionErr)
		}
		return err
	}
	return nil
}

func (o *ExecutionOrchestrator) wrapCancelError(
	ctx context.Context,
	machine *fsm.FSM,
	outcome *ParallelOutcome,
	err error,
) error {
	if ctx.Err() == nil {
		return err
	}
	return errors.Join(err, o.checkCanceled(ctx, machine, outcome))
}

func (o *ExecutionOrchestrator) transition(
	ctx context.Context,
	machine *fsm.FSM,
	event string,
	waveIndex int,
) error {
	if machine == nil {
		return errors.New("parallel execution: missing fsm")
	}
	from := machine.Current()
	if err := machine.Event(ctx, event); err != nil {
		return fmt.Errorf("parallel execution fsm transition %q from %q: %w", event, from, err)
	}
	to := machine.Current()
	if o.log != nil {
		attrs := []any{
			"component", "parallel_execution",
			"from", from,
			"to", to,
			"event", event,
		}
		if waveIndex >= 0 {
			attrs = append(attrs, "wave", waveIndex+1)
		}
		o.log.Debug("parallel execution fsm transition", attrs...)
	}
	return nil
}

type taskExecution struct {
	prepared  PreparedTaskRun
	result    TaskRunResult
	outcome   recovery.RunOutcome
	err       error
	recovered bool
}

func (o *ExecutionOrchestrator) recoverFailedExecutions(
	ctx context.Context,
	plan ParallelPlan,
	executions []taskExecution,
) {
	for idx := range executions {
		if executions[idx].outcome.Status != recovery.StatusFailed {
			continue
		}
		prepared := executions[idx].prepared
		if prepared == nil {
			continue
		}
		recoveredOutcome, err := o.recoverTask(ctx, plan, prepared, executions[idx].outcome, executions[idx].err)
		executions[idx].outcome = recoveredOutcome
		executions[idx].err = err
		if recoveredOutcome.Status == recovery.StatusSucceeded && err == nil {
			executions[idx].recovered = true
			if runID := strings.TrimSpace(recoveredOutcome.RunID); runID != "" {
				executions[idx].result.RunID = runID
			}
		}
	}
}

func (o *ExecutionOrchestrator) recoverTask(
	ctx context.Context,
	plan ParallelPlan,
	prepared PreparedTaskRun,
	initial recovery.RunOutcome,
	initialErr error,
) (recovery.RunOutcome, error) {
	orchestrator := recovery.NewRunRecoveryOrchestrator(
		o.recoveryStrategy,
		plan.Recovery,
		recovery.WithFailedRunConfig(prepared.FailedConfig()),
		recovery.WithRecoveryLogger(o.log),
	)
	return orchestrator.Run(ctx, cachedPreparedRun{
		prepared: prepared,
		outcome:  initial,
		err:      initialErr,
	})
}

type cachedPreparedRun struct {
	prepared recovery.PreparedRun
	outcome  recovery.RunOutcome
	err      error
}

func (r cachedPreparedRun) Execute(context.Context) (recovery.RunOutcome, error) {
	return r.outcome, r.err
}

func (r cachedPreparedRun) RestartFailed(ctx context.Context, failedJobIDs []string) (recovery.RunOutcome, error) {
	if r.prepared == nil {
		return recovery.RunOutcome{}, errors.New("parallel recovery: missing prepared task run")
	}
	return r.prepared.RestartFailed(ctx, failedJobIDs)
}

func taskExecutionFatalError(taskID TaskID, outcome recovery.RunOutcome, err error) error {
	if err == nil {
		return nil
	}
	if outcome.Status == recovery.StatusFailed {
		return nil
	}
	return fmt.Errorf("run task %s: %w", taskID, err)
}

func hasFailedExecutions(executions []taskExecution) bool {
	for index := range executions {
		execution := &executions[index]
		if execution.outcome.Status == recovery.StatusFailed {
			return true
		}
	}
	return false
}

func mergeableExecutions(executions []taskExecution) []taskExecution {
	result := make([]taskExecution, 0, len(executions))
	for index := range executions {
		execution := &executions[index]
		if execution.outcome.Status == recovery.StatusSucceeded && execution.err == nil {
			result = append(result, *execution)
		}
	}
	return result
}

func appendFailedTaskOutcomes(outcome *WaveOutcome, executions []taskExecution) []TaskID {
	failed := make([]TaskID, 0)
	for index := range executions {
		execution := &executions[index]
		if execution.outcome.Status == recovery.StatusSucceeded && execution.err == nil {
			continue
		}
		task := execution.result.Task
		if task.ID == "" {
			continue
		}
		failed = append(failed, task.ID)
		outcome.Tasks = append(outcome.Tasks, TaskOutcome{
			Task:         task,
			WaveIndex:    outcome.Index,
			RunID:        strings.TrimSpace(execution.result.RunID),
			WorktreePath: strings.TrimSpace(execution.result.WorktreePath),
			Status:       TaskOutcomeFailed,
			Error:        taskExecutionError(*execution),
		})
	}
	sortTaskIDs(failed)
	return failed
}

func taskExecutionError(execution taskExecution) string {
	if execution.err != nil {
		return strings.TrimSpace(execution.err.Error())
	}
	for _, job := range execution.outcome.Jobs {
		if job.Status == recovery.StatusFailed && strings.TrimSpace(job.Error) != "" {
			return strings.TrimSpace(job.Error)
		}
	}
	return ""
}

func splitRunnableLevel(
	waveIndex int,
	level []TaskID,
	tasksByID map[TaskID]TaskSpec,
	blockedBy map[TaskID]TaskID,
) ([]TaskID, []TaskOutcome) {
	runnable := make([]TaskID, 0, len(level))
	skipped := make([]TaskOutcome, 0)
	for _, taskID := range level {
		blocker, blocked := blockedBy[taskID]
		if !blocked {
			runnable = append(runnable, taskID)
			continue
		}
		task := tasksByID[taskID]
		skipped = append(skipped, TaskOutcome{
			Task:      task,
			WaveIndex: waveIndex,
			Status:    TaskOutcomeSkipped,
			BlockedBy: blocker,
			Error:     fmt.Sprintf("blocked by %s", blocker),
		})
	}
	sortTaskOutcomes(skipped)
	return runnable, skipped
}

func mergedStatus(execution taskExecution) TaskOutcomeStatus {
	if execution.recovered {
		return TaskOutcomeRecovered
	}
	return TaskOutcomeMerged
}

func (s TaskOutcomeStatus) removesWorktree() bool {
	return s == TaskOutcomeMerged || s == TaskOutcomeRecovered
}

func sortTaskOutcomes(outcomes []TaskOutcome) {
	sort.SliceStable(outcomes, func(i, j int) bool {
		return outcomes[i].Task.Number < outcomes[j].Task.Number
	})
}

func taskSpecsByID(tasks []TaskSpec) map[TaskID]TaskSpec {
	result := make(map[TaskID]TaskSpec, len(tasks))
	for _, task := range tasks {
		if task.ID == "" {
			continue
		}
		result[task.ID] = task
	}
	return result
}

func taskSpecsForLevel(level []TaskID, tasksByID map[TaskID]TaskSpec) []TaskSpec {
	tasks := make([]TaskSpec, 0, len(level))
	for _, id := range level {
		if task, ok := tasksByID[id]; ok {
			tasks = append(tasks, task)
		}
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].Number < tasks[j].Number
	})
	return tasks
}

func maxConcurrency(cfg workspace.ParallelTasksConfig) int {
	effective := cfg.ApplyDefaults()
	if effective.MaxConcurrency == nil || *effective.MaxConcurrency < 1 {
		return workspace.DefaultParallelTasksMaxConcurrency
	}
	return *effective.MaxConcurrency
}

func commitMessage(task TaskSpec) string {
	title := strings.TrimSpace(task.Title)
	if title == "" {
		title = strings.TrimSpace(string(task.ID))
	}
	number := strconv.Itoa(task.Number)
	if task.Number < 10 {
		number = "0" + number
	}
	return "task " + number + ": " + title
}

package parallelrun

import (
	"context"
	"encoding/json"
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
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
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
	WaveTotal int
	Task      TaskSpec
	Base      WorktreeBase
}

// TaskRunResult is the daemon-neutral metadata for one child task run.
type TaskRunResult struct {
	Task                    TaskSpec
	RunID                   string
	WorktreePath            string
	BaseBranch              string
	BaseCommit              string
	ScopeSupported          bool
	ProducedPaths           []string
	PreExistingChangedPaths []string
	ScopeError              string
	ScopeArtifactPath       string
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
	CommitTask(ctx context.Context, spec TaskCommitSpec) (string, error)
	CommitStaged(ctx context.Context, spec StagedCommitSpec) (string, error)
	SquashMerge(ctx context.Context, integrationPath string, worktreeRef string, message string) (ConflictSet, error)
	Head(ctx context.Context, path string) (string, error)
	FastForward(ctx context.Context, workspaceRoot string, targetBranch string, integrationBranch string) error
	SyncTaskArtifacts(ctx context.Context, workspaceRoot string, tasks []TaskOutcome) error
	CleanupTaskWorktree(ctx context.Context, spec TaskWorktreeCleanupSpec) (WorktreeCleanupResult, error)
	CleanupIntegration(ctx context.Context, spec IntegrationCleanupSpec) (WorktreeCleanupResult, error)
}

// WorktreeCleanupStatus records the real post-cleanup state exposed to users.
type WorktreeCleanupStatus string

const (
	WorktreeCleanupStatusActive    WorktreeCleanupStatus = "active"
	WorktreeCleanupStatusRemoved   WorktreeCleanupStatus = "removed"
	WorktreeCleanupStatusPreserved WorktreeCleanupStatus = "preserved"
)

// WorktreeCleanupResult classifies a safe cleanup decision without turning an
// intentionally preserved tree into an execution failure.
type WorktreeCleanupResult struct {
	Status WorktreeCleanupStatus
	Reason string
}

// TaskWorktreeCleanupSpec supplies the evidence needed to remove or preserve a
// task tree. ContentIntegrated is true only after the final branch was
// fast-forwarded successfully.
type TaskWorktreeCleanupSpec struct {
	WorkspaceRoot     string
	Path              string
	BaseCommit        string
	ContentIntegrated bool
}

// IntegrationCleanupSpec supplies the evidence needed to remove or preserve
// the integration worktree and branch.
type IntegrationCleanupSpec struct {
	WorkspaceRoot     string
	IntegrationPath   string
	IntegrationBranch string
	BaseCommit        string
	ContentIntegrated bool
}

// TaskCommitSpec constrains a task worktree commit to paths the child run
// proved it produced after its pre-agent baseline snapshot.
type TaskCommitSpec struct {
	Path                    string
	Message                 string
	ScopeSupported          bool
	ProducedPaths           []string
	PreExistingChangedPaths []string
	ScopeError              string
	ScopeArtifactPath       string
}

type taskCommitScopeError struct {
	err error
}

func (e taskCommitScopeError) Error() string {
	return e.err.Error()
}

func (e taskCommitScopeError) Unwrap() error {
	return e.err
}

// NewTaskCommitScopeError marks a task commit failure caused by invalid or
// contaminated produced-path scope metadata.
func NewTaskCommitScopeError(err error) error {
	if err == nil {
		return nil
	}
	return taskCommitScopeError{err: err}
}

// IsTaskCommitScopeError reports whether err came from produced-path scope validation.
func IsTaskCommitScopeError(err error) bool {
	var scopeErr taskCommitScopeError
	return errors.As(err, &scopeErr)
}

// StagedCommitSpec constrains an integration commit to already-staged clean
// merge paths plus explicitly staged conflict paths.
type StagedCommitSpec struct {
	Path         string
	Message      string
	AllowedPaths []string
	StagePaths   []string
}

// ConflictSet describes a squash merge result.
type ConflictSet struct {
	Files       []string
	StagedFiles []string
	Clean       bool
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
	ParallelOutcomeFailed     OutcomeStatus = "failed"
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
	BaseCommit     string
	WorktreeCommit string
	WorktreeStatus string
	WorktreeReason string
	Status         TaskOutcomeStatus
	BlockedBy      TaskID
	Error          string
}

// TaskOutcomeStatus is the per-task status reported by the parallel run.
type TaskOutcomeStatus = kinds.TaskParallelTaskStatus

const (
	TaskOutcomeMerged    = kinds.TaskParallelTaskStatusMerged
	TaskOutcomeRecovered = kinds.TaskParallelTaskStatusRecovered
	TaskOutcomeFailed    = kinds.TaskParallelTaskStatusFailed
	TaskOutcomeSkipped   = kinds.TaskParallelTaskStatusSkipped
	TaskOutcomeCanceled  = kinds.TaskParallelTaskStatusCanceled
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
	Status                      OutcomeStatus
	IntegrationBranch           string
	IntegrationPath             string
	Waves                       []WaveOutcome
	Tasks                       []TaskOutcome
	taskCleanupAttempted        bool
	integrationCleanupAttempted bool
	taskSettlementsAttempted    bool
}

// ParallelOutcome preserves the task-specified public API name.
type ParallelOutcome = Outcome

// ExecutionOrchestrator drives the parallel task FSM happy path.
type ExecutionOrchestrator struct {
	worktrees         WorktreeLifecycle
	launcher          TaskLauncher
	recoveryStrategy  recovery.RemediationStrategy
	recoveryEventSink recovery.EventSink
	conflictResolver  ConflictResolver
	emitter           ParallelEventEmitter
	log               *slog.Logger
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

// WithRecoveryEventSink supplies the sink for run.recovery_* lifecycle events.
func WithRecoveryEventSink(sink recovery.EventSink) ExecutionOrchestratorOption {
	return func(o *ExecutionOrchestrator) {
		o.recoveryEventSink = sink
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
	if o == nil {
		return ParallelOutcome{}, errors.New("parallel execution: missing orchestrator")
	}
	if ctx == nil {
		return ParallelOutcome{}, errors.New("parallel execution: context is required")
	}
	if err := o.validatePlan(plan); err != nil {
		return ParallelOutcome{}, err
	}

	machine := newParallelFSM()
	outcome := Outcome{
		IntegrationBranch: strings.TrimSpace(plan.IntegrationBranch),
		IntegrationPath:   strings.TrimSpace(plan.IntegrationPath),
	}
	if err := o.emitPlanStarted(ctx, plan); err != nil {
		return outcome, o.fail(ctx, machine, plan, -1, &outcome, err)
	}
	if err := o.createIntegrationBranch(ctx, plan); err != nil {
		cause := fmt.Errorf("create parallel integration branch: %w", err)
		return outcome, o.fail(ctx, machine, plan, -1, &outcome, cause)
	}
	levels := plan.Waves.Levels()
	if err := o.runWaves(ctx, machine, plan, levels, &outcome); err != nil {
		return o.settleRunError(ctx, machine, plan, outcome, err)
	}
	if err := failedTaskOutcomesError(outcome.Tasks); err != nil {
		return o.settleRunError(ctx, machine, plan, outcome, err)
	}
	if err := o.finalize(ctx, machine, plan, levels, &outcome); err != nil {
		return o.settleRunError(ctx, machine, plan, outcome, err)
	}
	return outcome, nil
}

func (o *ExecutionOrchestrator) settleRunError(
	ctx context.Context,
	machine *fsm.FSM,
	plan ParallelPlan,
	outcome ParallelOutcome,
	cause error,
) (ParallelOutcome, error) {
	settleCtx := context.WithoutCancel(ctx)
	o.cleanupTaskOutcomes(settleCtx, plan, &outcome, false)
	integrationErr := o.cleanupIntegration(settleCtx, plan, &outcome, false)
	taskEventErr := o.emitTaskSettlements(settleCtx, plan, &outcome)
	cause = errors.Join(cause, integrationErr, taskEventErr)
	if ctx.Err() != nil {
		settleErr := o.cancel(settleCtx, machine, plan, &outcome, errors.Join(cause, ctx.Err()))
		return outcome, settleErr
	}
	waveIndex := -1
	if len(outcome.Waves) > 0 {
		waveIndex = outcome.Waves[len(outcome.Waves)-1].Index
	}
	if isParallelRollbackError(cause) {
		settleErr := o.rollback(settleCtx, machine, plan, waveIndex, &outcome, cause)
		return outcome, settleErr
	}
	settleErr := o.fail(settleCtx, machine, plan, waveIndex, &outcome, cause)
	return outcome, settleErr
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
		if err := o.emitPhaseChanged(ctx, plan, waveIndex, parallelPhaseAdvancingBase); err != nil {
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
	if err := checkCanceled(ctx); err != nil {
		return err
	}
	if err := o.transition(ctx, machine, parallelEventStartWave, waveIndex); err != nil {
		return err
	}
	runnable, skipped := splitRunnableLevel(waveIndex, level, tasksByID, *blockedBy)
	if err := o.emitWaveTasksStarted(ctx, plan, waveIndex, waveTotal, runnable, tasksByID); err != nil {
		return err
	}
	waveOutcome := WaveOutcome{Index: waveIndex, Tasks: skipped}
	executions, err := o.runWave(ctx, plan, waveIndex, waveTotal, runnable, currentBase, tasksByID)
	if err != nil {
		status := TaskOutcomeFailed
		if ctx.Err() != nil {
			status = TaskOutcomeCanceled
		}
		waveOutcome.Tasks = append(
			waveOutcome.Tasks,
			interruptedTaskOutcomes(waveIndex, runnable, tasksByID, executions, waveOutcome.Tasks, status, err)...,
		)
		recordWaveOutcome(outcome, waveOutcome)
		return wrapCancelError(ctx, err)
	}
	if hasFailedExecutions(executions) {
		if err := o.transition(ctx, machine, parallelEventRecoverWave, waveIndex); err != nil {
			return err
		}
		o.recoverFailedExecutions(ctx, plan, executions)
	}
	newFailures := appendFailedTaskOutcomes(&waveOutcome, executions)
	recordNewWaveFailures(plan, newFailures, failed, blockedBy)
	if err := o.transition(ctx, machine, parallelEventMergeWave, waveIndex); err != nil {
		return err
	}
	if err := o.emitMergeStarted(ctx, plan, waveIndex, waveTotal); err != nil {
		return err
	}
	mergeOutcome, err := o.mergeWave(ctx, machine, plan, waveIndex, mergeableExecutions(executions))
	if err != nil {
		waveOutcome.Tasks = append(waveOutcome.Tasks, mergeOutcome.Tasks...)
		waveOutcome.Tasks = append(
			waveOutcome.Tasks,
			interruptedTaskOutcomes(
				waveIndex,
				runnable,
				tasksByID,
				executions,
				waveOutcome.Tasks,
				TaskOutcomeFailed,
				err,
			)...,
		)
		recordWaveOutcome(outcome, waveOutcome)
		return wrapCancelError(ctx, err)
	}
	waveOutcome.Tasks = append(waveOutcome.Tasks, mergeOutcome.Tasks...)
	recordWaveOutcome(outcome, waveOutcome)
	if err := o.transition(ctx, machine, parallelEventFinishWave, waveIndex); err != nil {
		return err
	}
	return o.emitWaveCompleted(ctx, plan, waveIndex, waveTotal)
}

func recordNewWaveFailures(
	plan ParallelPlan,
	newFailures []TaskID,
	failed map[TaskID]bool,
	blockedBy *map[TaskID]TaskID,
) {
	if len(newFailures) == 0 {
		return
	}
	for _, taskID := range newFailures {
		failed[taskID] = true
	}
	updated := plan.Waves.BlockedBy(failed)
	if updated == nil {
		updated = map[TaskID]TaskID{}
	}
	*blockedBy = updated
}

// emitWaveTasksStarted announces every runnable task entering a wave so the TUI
// can group sidebar cards by wave before the per-task child runs start streaming.
func (o *ExecutionOrchestrator) emitWaveTasksStarted(
	ctx context.Context,
	plan ParallelPlan,
	waveIndex, waveTotal int,
	runnable []TaskID,
	tasksByID map[TaskID]TaskSpec,
) error {
	for _, taskID := range runnable {
		task, ok := tasksByID[taskID]
		if !ok {
			task = TaskSpec{ID: taskID}
		}
		if err := o.emitWaveStarted(ctx, plan, waveIndex, waveTotal, task); err != nil {
			return err
		}
	}
	return nil
}

func (o *ExecutionOrchestrator) finalize(
	ctx context.Context,
	machine *fsm.FSM,
	plan ParallelPlan,
	levels [][]TaskID,
	outcome *ParallelOutcome,
) error {
	if err := checkCanceled(ctx); err != nil {
		return err
	}
	if err := o.emitPhaseChanged(ctx, plan, len(levels)-1, parallelPhaseFinalizing); err != nil {
		return err
	}
	if err := o.transition(ctx, machine, parallelEventFinalize, len(levels)); err != nil {
		return err
	}
	if err := o.emitPhaseChanged(ctx, plan, len(levels)-1, parallelPhaseFastForwarding); err != nil {
		return err
	}
	if err := o.worktrees.FastForward(ctx, plan.WorkspaceRoot, plan.BaseBranch, plan.IntegrationBranch); err != nil {
		return fmt.Errorf("fast-forward %s to %s: %w", plan.BaseBranch, plan.IntegrationBranch, err)
	}
	if err := o.emitPhaseChanged(ctx, plan, len(levels)-1, parallelPhaseSyncingArtifacts); err != nil {
		return err
	}
	if err := o.worktrees.SyncTaskArtifacts(ctx, plan.WorkspaceRoot, outcome.Tasks); err != nil {
		return fmt.Errorf("sync completed task artifacts: %w", err)
	}
	if err := o.emitPhaseChanged(ctx, plan, len(levels)-1, parallelPhaseCleaningUp); err != nil {
		return err
	}
	o.cleanupTaskOutcomes(ctx, plan, outcome, true)
	if err := o.cleanupIntegration(ctx, plan, outcome, true); err != nil {
		return err
	}
	if err := o.emitTaskSettlements(ctx, plan, outcome); err != nil {
		return err
	}
	if err := o.transition(ctx, machine, parallelEventComplete, len(levels)); err != nil {
		return err
	}
	if err := o.emitCompleted(ctx, plan); err != nil {
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
	waveTotal int,
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
		results[index].result = TaskRunResult{Task: task, BaseBranch: base.Branch, BaseCommit: base.Commit}
		wg.Add(1)
		launched++
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			prepared, err := o.launcher.PrepareTask(ctx, TaskLaunchSpec{
				RunID:     strings.TrimSpace(plan.RunID),
				WaveIndex: waveIndex,
				WaveTotal: waveTotal,
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
			if result.BaseBranch == "" {
				result.BaseBranch = strings.TrimSpace(base.Branch)
			}
			if result.BaseCommit == "" {
				result.BaseCommit = strings.TrimSpace(base.Commit)
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
		commit, err := o.worktrees.CommitTask(ctx, taskCommitSpec(run))
		if err != nil {
			err = fmt.Errorf("commit task %s worktree: %w", run.Task.ID, err)
			return outcome, err
		}
		conflicts, err := o.worktrees.SquashMerge(ctx, plan.IntegrationPath, commit, commitMessage(run.Task))
		if err != nil {
			return outcome, fmt.Errorf("squash merge task %s: %w", run.Task.ID, err)
		}
		if !conflicts.Clean {
			if err := o.emitConflictDetected(
				ctx,
				plan,
				waveIndex,
				run.Task,
				conflicts,
				1,
				resolverMaxAttempts(plan),
			); err != nil {
				return outcome, err
			}
			result, err := o.resolveConflict(ctx, machine, plan, waveIndex, run.Task, conflicts)
			if err != nil {
				if IsConflictResolverSetupError(err) {
					return outcome, err
				}
				return outcome, newParallelRollbackError(err)
			}
			if !result.Resolved || !result.Validated {
				return outcome, newParallelRollbackError(
					conflictResolverExhaustedError(run.Task.ID, result, conflicts),
				)
			}
			if _, err := o.worktrees.CommitStaged(ctx, StagedCommitSpec{
				Path:         plan.IntegrationPath,
				Message:      commitMessage(run.Task),
				AllowedPaths: conflictCommitAllowedPaths(conflicts),
				StagePaths:   normalizedConflictFiles(conflicts.Files),
			}); err != nil {
				return outcome, newParallelRollbackError(
					fmt.Errorf("commit resolved squash merge for task %s: %w", run.Task.ID, err),
				)
			}
		}
		taskOutcome := TaskOutcome{
			Task:           run.Task,
			WaveIndex:      waveIndex,
			RunID:          strings.TrimSpace(run.RunID),
			WorktreePath:   strings.TrimSpace(run.WorktreePath),
			BaseCommit:     strings.TrimSpace(run.BaseCommit),
			WorktreeCommit: strings.TrimSpace(commit),
			WorktreeStatus: string(WorktreeCleanupStatusActive),
			Status:         mergedStatus(*execution),
		}
		outcome.Tasks = append(outcome.Tasks, taskOutcome)
		if err := o.emitTaskMerged(ctx, plan, taskOutcome); err != nil {
			return outcome, err
		}
	}
	return outcome, nil
}

func conflictResolverExhaustedError(taskID TaskID, result ConflictResult, conflicts ConflictSet) error {
	details := strings.TrimSpace(result.ValidationError)
	if details != "" {
		details = ": validation_error=" + details
	}
	return fmt.Errorf(
		"squash merge task %s conflict resolver exhausted after %d attempt(s): resolved=%t validated=%t files=%s%s",
		taskID,
		result.Attempts,
		result.Resolved,
		result.Validated,
		strings.Join(conflicts.Files, ", "),
		details,
	)
}

func taskCommitSpec(run TaskRunResult) TaskCommitSpec {
	return TaskCommitSpec{
		Path:                    strings.TrimSpace(run.WorktreePath),
		Message:                 commitMessage(run.Task),
		ScopeSupported:          run.ScopeSupported,
		ProducedPaths:           append([]string(nil), run.ProducedPaths...),
		PreExistingChangedPaths: append([]string(nil), run.PreExistingChangedPaths...),
		ScopeError:              strings.TrimSpace(run.ScopeError),
		ScopeArtifactPath:       strings.TrimSpace(run.ScopeArtifactPath),
	}
}

func conflictCommitAllowedPaths(conflicts ConflictSet) []string {
	allowed := append([]string(nil), conflicts.StagedFiles...)
	allowed = append(allowed, conflicts.Files...)
	return normalizedConflictFiles(allowed)
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
	if err := o.emitConflictResolving(
		ctx,
		plan,
		waveIndex,
		task,
		conflicts,
		1,
		resolverMaxAttempts(plan),
	); err != nil {
		return ConflictResult{}, err
	}
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
	if result.Resolved && result.Validated {
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
	if outcome != nil {
		outcome.Status = ParallelOutcomeRolledBack
	}
	transitionErr := o.transition(ctx, machine, parallelEventRollback, -1)
	emitErr := o.emitRolledBack(ctx, plan, waveIndex)
	return errors.Join(cause, transitionErr, emitErr)
}

func (o *ExecutionOrchestrator) fail(
	ctx context.Context,
	machine *fsm.FSM,
	plan ParallelPlan,
	waveIndex int,
	outcome *ParallelOutcome,
	cause error,
) error {
	failCtx := context.WithoutCancel(ctx)
	if outcome != nil {
		outcome.Status = ParallelOutcomeFailed
	}
	transitionErr := o.transition(failCtx, machine, parallelEventFail, waveIndex)
	emitErr := o.emitFailed(failCtx, plan, waveIndex, cause)
	return errors.Join(cause, transitionErr, emitErr)
}

func (o *ExecutionOrchestrator) cleanupTaskOutcomes(
	ctx context.Context,
	plan ParallelPlan,
	outcome *ParallelOutcome,
	contentIntegrated bool,
) {
	if outcome == nil {
		return
	}
	if outcome.taskCleanupAttempted {
		return
	}
	outcome.taskCleanupAttempted = true
	for index := range outcome.Tasks {
		task := &outcome.Tasks[index]
		if strings.TrimSpace(task.WorktreePath) == "" {
			continue
		}
		result, err := o.worktrees.CleanupTaskWorktree(ctx, TaskWorktreeCleanupSpec{
			WorkspaceRoot:     plan.WorkspaceRoot,
			Path:              task.WorktreePath,
			BaseCommit:        task.BaseCommit,
			ContentIntegrated: contentIntegrated && task.Status.IsIntegrated(),
		})
		if err != nil {
			task.WorktreeStatus = string(WorktreeCleanupStatusPreserved)
			task.WorktreeReason = fmt.Sprintf("cleanup inspection failed: %v", err)
			o.logCleanupPreserved(*task)
			continue
		}
		task.WorktreeStatus = string(result.Status)
		task.WorktreeReason = strings.TrimSpace(result.Reason)
		if result.Status == WorktreeCleanupStatusPreserved {
			o.logCleanupPreserved(*task)
		}
	}
}

func (o *ExecutionOrchestrator) logCleanupPreserved(task TaskOutcome) {
	if o.log == nil {
		return
	}
	o.log.Warn(
		"parallel task worktree preserved",
		"task_id",
		task.Task.ID,
		"worktree_path",
		task.WorktreePath,
		"reason",
		task.WorktreeReason,
	)
}

func (o *ExecutionOrchestrator) cleanupIntegration(
	ctx context.Context,
	plan ParallelPlan,
	outcome *ParallelOutcome,
	contentIntegrated bool,
) error {
	if outcome != nil {
		if outcome.integrationCleanupAttempted {
			return nil
		}
		outcome.integrationCleanupAttempted = true
	}
	result, err := o.worktrees.CleanupIntegration(ctx, IntegrationCleanupSpec{
		WorkspaceRoot:     plan.WorkspaceRoot,
		IntegrationPath:   plan.IntegrationPath,
		IntegrationBranch: plan.IntegrationBranch,
		BaseCommit:        plan.BaseCommit,
		ContentIntegrated: contentIntegrated,
	})
	if err != nil {
		return fmt.Errorf("cleanup integration branch %s: %w", plan.IntegrationBranch, err)
	}
	if result.Status == WorktreeCleanupStatusPreserved && o.log != nil {
		o.log.Warn(
			"parallel integration worktree preserved",
			"integration_branch",
			plan.IntegrationBranch,
			"integration_path",
			plan.IntegrationPath,
			"reason",
			strings.TrimSpace(result.Reason),
		)
	}
	return nil
}

func (o *ExecutionOrchestrator) emitTaskSettlements(
	ctx context.Context,
	plan ParallelPlan,
	outcome *ParallelOutcome,
) error {
	if outcome == nil || outcome.taskSettlementsAttempted {
		return nil
	}
	outcome.taskSettlementsAttempted = true
	for index := range outcome.Tasks {
		if err := o.emitTaskCompleted(ctx, plan, outcome.Tasks[index]); err != nil {
			return err
		}
	}
	return nil
}

func checkCanceled(ctx context.Context) error {
	return ctx.Err()
}

func (o *ExecutionOrchestrator) cancel(
	ctx context.Context,
	machine *fsm.FSM,
	plan ParallelPlan,
	outcome *ParallelOutcome,
	cause error,
) error {
	cancelCtx := context.WithoutCancel(ctx)
	if outcome != nil {
		outcome.Status = ParallelOutcomeCanceled
	}
	transitionErr := o.transition(cancelCtx, machine, parallelEventCancel, -1)
	emitErr := o.emitCanceled(cancelCtx, plan, cause)
	return errors.Join(cause, transitionErr, emitErr)
}

func wrapCancelError(ctx context.Context, err error) error {
	if ctx.Err() == nil {
		return err
	}
	return errors.Join(err, ctx.Err())
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

type parallelRollbackError struct {
	err error
}

func (e parallelRollbackError) Error() string {
	return e.err.Error()
}

func (e parallelRollbackError) Unwrap() error {
	return e.err
}

func newParallelRollbackError(err error) error {
	if err == nil {
		return nil
	}
	return parallelRollbackError{err: err}
}

func isParallelRollbackError(err error) bool {
	var rollbackErr parallelRollbackError
	return errors.As(err, &rollbackErr)
}

func (o *ExecutionOrchestrator) recoverFailedExecutions(
	ctx context.Context,
	plan ParallelPlan,
	executions []taskExecution,
) {
	failedIndexes := make([]int, 0)
	for idx := range executions {
		if executions[idx].outcome.Status != recovery.StatusFailed || executions[idx].prepared == nil {
			continue
		}
		failedIndexes = append(failedIndexes, idx)
	}
	if len(failedIndexes) == 0 {
		return
	}
	workerCount := min(maxConcurrency(plan.Config), len(failedIndexes))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				o.recoverExecutionAt(ctx, plan, executions, idx)
			}
		}()
	}
	for _, idx := range failedIndexes {
		if err := ctx.Err(); err != nil {
			break
		}
		select {
		case jobs <- idx:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		}
	}
	close(jobs)
	wg.Wait()
}

func (o *ExecutionOrchestrator) recoverExecutionAt(
	ctx context.Context,
	plan ParallelPlan,
	executions []taskExecution,
	idx int,
) {
	prepared := executions[idx].prepared
	recoveredOutcome, err := o.recoverTask(ctx, plan, prepared, executions[idx].outcome, executions[idx].err)
	executions[idx].outcome = recoveredOutcome
	executions[idx].err = err
	result := prepared.Result()
	if result.Task.ID == "" {
		result.Task = executions[idx].result.Task
	}
	if result.RunID == "" {
		result.RunID = executions[idx].result.RunID
	}
	executions[idx].result = result
	if recoveredOutcome.Status == recovery.StatusSucceeded && err == nil {
		executions[idx].recovered = true
		if runID := strings.TrimSpace(recoveredOutcome.RunID); runID != "" {
			executions[idx].result.RunID = runID
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
		recovery.WithRecoveryEventSink(taskRecoveryEventSink{delegate: o.recoveryEventSink, prepared: prepared}),
		recovery.WithRecoveryLogger(o.log),
	)
	return orchestrator.Run(ctx, cachedPreparedRun{
		prepared: prepared,
		outcome:  initial,
		err:      initialErr,
	})
}

type taskRecoveryEventSink struct {
	delegate recovery.EventSink
	prepared PreparedTaskRun
}

func (s taskRecoveryEventSink) Submit(ctx context.Context, event events.Event) error {
	if s.delegate == nil {
		return nil
	}
	runID := ""
	if s.prepared != nil {
		if runID = strings.TrimSpace(s.prepared.Result().RunID); runID != "" {
			event.RunID = runID
			event = withTaskRecoveryRunID(event, runID)
		}
	}
	return s.delegate.Submit(ctx, event)
}

func withTaskRecoveryRunID(event events.Event, runID string) events.Event {
	if strings.TrimSpace(runID) == "" || len(event.Payload) == 0 {
		return event
	}
	switch event.Kind {
	case events.EventKindRunRecoveryStarted:
		var payload kinds.RunRecoveryStartedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return event
		}
		payload.RecoveryRunID = runID
		return withRecoveryPayload(event, payload)
	case events.EventKindRunRecoveryRestarting:
		var payload kinds.RunRecoveryRestartingPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return event
		}
		payload.RecoveryRunID = runID
		return withRecoveryPayload(event, payload)
	case events.EventKindRunRecovered:
		var payload kinds.RunRecoveredPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return event
		}
		payload.RecoveryRunID = runID
		return withRecoveryPayload(event, payload)
	case events.EventKindRunRecoveryExhausted:
		var payload kinds.RunRecoveryExhaustedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return event
		}
		payload.RecoveryRunID = runID
		return withRecoveryPayload(event, payload)
	default:
		return event
	}
}

func withRecoveryPayload(event events.Event, payload any) events.Event {
	raw, err := json.Marshal(payload)
	if err != nil {
		return event
	}
	event.Payload = raw
	return event
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
			Task:           task,
			WaveIndex:      outcome.Index,
			RunID:          strings.TrimSpace(execution.result.RunID),
			WorktreePath:   strings.TrimSpace(execution.result.WorktreePath),
			BaseCommit:     strings.TrimSpace(execution.result.BaseCommit),
			WorktreeStatus: initialWorktreeStatus(execution.result.WorktreePath),
			Status:         TaskOutcomeFailed,
			Error:          taskExecutionError(*execution),
		})
	}
	sortTaskIDs(failed)
	return failed
}

func interruptedTaskOutcomes(
	waveIndex int,
	runnable []TaskID,
	tasksByID map[TaskID]TaskSpec,
	executions []taskExecution,
	existing []TaskOutcome,
	status TaskOutcomeStatus,
	cause error,
) []TaskOutcome {
	existingByID := make(map[TaskID]struct{}, len(existing))
	for index := range existing {
		existingByID[existing[index].Task.ID] = struct{}{}
	}
	executionsByID := make(map[TaskID]taskExecution, len(executions))
	for index := range executions {
		taskID := executions[index].result.Task.ID
		if taskID != "" {
			executionsByID[taskID] = executions[index]
		}
	}
	message := ""
	if cause != nil {
		message = strings.TrimSpace(cause.Error())
	}
	result := make([]TaskOutcome, 0, len(runnable))
	for _, taskID := range runnable {
		if _, ok := existingByID[taskID]; ok {
			continue
		}
		execution, ok := executionsByID[taskID]
		task := tasksByID[taskID]
		if ok && execution.result.Task.ID != "" {
			task = execution.result.Task
		}
		item := TaskOutcome{Task: task, WaveIndex: waveIndex, Status: status, Error: message}
		if ok {
			item.RunID = strings.TrimSpace(execution.result.RunID)
			item.WorktreePath = strings.TrimSpace(execution.result.WorktreePath)
			item.BaseCommit = strings.TrimSpace(execution.result.BaseCommit)
			item.WorktreeStatus = initialWorktreeStatus(execution.result.WorktreePath)
			if executionError := taskExecutionError(execution); executionError != "" {
				item.Error = executionError
			}
		}
		result = append(result, item)
	}
	sortTaskOutcomes(result)
	return result
}

func recordWaveOutcome(outcome *ParallelOutcome, wave WaveOutcome) {
	if outcome == nil {
		return
	}
	sortTaskOutcomes(wave.Tasks)
	outcome.Waves = append(outcome.Waves, wave)
	outcome.Tasks = append(outcome.Tasks, wave.Tasks...)
}

func initialWorktreeStatus(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return string(WorktreeCleanupStatusActive)
}

func failedTaskOutcomesError(outcomes []TaskOutcome) error {
	failures := make([]string, 0)
	for index := range outcomes {
		outcome := outcomes[index]
		if outcome.Status != TaskOutcomeFailed && outcome.Status != TaskOutcomeSkipped {
			continue
		}
		detail := outcome.StatusReport()
		if message := strings.TrimSpace(outcome.Error); message != "" {
			detail += ": " + message
		}
		failures = append(failures, fmt.Sprintf("%s=%s", outcome.Task.ID, detail))
	}
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("parallel task execution did not complete: %s", strings.Join(failures, "; "))
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
